// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"

	"github.com/dgryski/dgoogauth"
	"github.com/mattermost/rsc/qr"
	"github.com/pkg/errors"
)

// InvalidToken indicates the case where the token validation has failed.
var InvalidToken = errors.New("invalid mfa token")

const (
	// This will result in 160 bits of entropy (base32 encoded), as recommended by rfc4226.
	mfaSecretSize = 20
)

type Store interface {
	UpdateMfaActive(userID string, active bool) error
	UpdateMfaSecret(userID, secret string) error
}

type MFA struct {
	store Store
}

func New(store Store) *MFA {
	return &MFA{store}
}

// newRandomBase32String returns a base32 encoded string of a random slice
// of bytes of the given size. The resulting entropy will be (8 * size) bits.
func newRandomBase32String(size int) string {
	data := make([]byte, size)
	rand.Read(data)
	return base32.StdEncoding.EncodeToString(data)
}

func getIssuerFromUrl(uri string) string {
	issuer := "Mattermost"
	siteUrl := strings.TrimSpace(uri)

	if siteUrl != "" {
		siteUrl = strings.TrimPrefix(siteUrl, "https://")
		siteUrl = strings.TrimPrefix(siteUrl, "http://")
		issuer = strings.TrimPrefix(siteUrl, "www.")
	}

	return url.QueryEscape(issuer)
}

// GenerateSecret generates a new user mfa secret and store it with the StoreSecret function provided
func (m *MFA) GenerateSecret(siteURL, userEmail, userID string) (string, []byte, error) {
	issuer := getIssuerFromUrl(siteURL)

	secret := newRandomBase32String(mfaSecretSize)

	authLink := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, userEmail, secret, issuer)

	code, err := qr.Encode(authLink, qr.H)

	if err != nil {
		return "", nil, errors.Wrap(err, "unable to generate qr code")
	}

	img := code.PNG()

	if err := m.store.UpdateMfaSecret(userID, secret); err != nil {
		return "", nil, errors.Wrap(err, "unable to store mfa secret")
	}

	return secret, img, nil
}

// Activate set the mfa as active and store it with the StoreActive function provided
func (m *MFA) Activate(userMfaSecret, userID string, token string) error {
	otpConfig := &dgoogauth.OTPConfig{
		Secret:      userMfaSecret,
		WindowSize:  3,
		HotpCounter: 0,
	}

	trimmedToken := strings.TrimSpace(token)

	ok, err := otpConfig.Authenticate(trimmedToken)
	if err != nil {
		return errors.Wrap(err, "unable to parse the token")
	}

	if !ok {
		return InvalidToken
	}

	if err := m.store.UpdateMfaActive(userID, true); err != nil {
		return errors.Wrap(err, "unable to store mfa active")
	}

	return nil
}

// Deactivate set the mfa as deactive, remove the mfa secret, store it with the StoreActive and StoreSecret functions provided
func (m *MFA) Deactivate(userID string) error {
	if err := m.store.UpdateMfaActive(userID, false); err != nil {
		return errors.Wrap(err, "unable to store mfa active")
	}

	if err := m.store.UpdateMfaSecret(userID, ""); err != nil {
		return errors.Wrap(err, "unable to store mfa secret")
	}

	return nil
}

// Validate the provide token using the secret provided
func (m *MFA) ValidateToken(secret, token string) (bool, error) {
	otpConfig := &dgoogauth.OTPConfig{
		Secret:      secret,
		WindowSize:  3,
		HotpCounter: 0,
	}

	trimmedToken := strings.TrimSpace(token)
	ok, err := otpConfig.Authenticate(trimmedToken)
	if err != nil {
		return false, errors.Wrap(err, "unable to parse the token")
	}

	return ok, nil
}
