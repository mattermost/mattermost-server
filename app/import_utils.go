// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/rand"
	"math/big"
)

const (
	passwordSpecialChars     = "!$%^&*(),."
	passwordNumbers          = "0123456789"
	passwordUpperCaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	passwordLowerCaseLetters = "abcdefghijklmnopqrstuvwxyz"
	passwordAllChars         = passwordSpecialChars + passwordNumbers + passwordUpperCaseLetters + passwordLowerCaseLetters
)

func randInt(max int) (int, error) {
	val, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(val.Int64()), nil
}

func generatePassword(minimumLength int) (string, error) {
	upperIDx, err := randInt(len(passwordUpperCaseLetters))
	if err != nil {
		return "", err
	}
	numberIDx, err := randInt(len(passwordNumbers))
	if err != nil {
		return "", err
	}
	lowerIDx, err := randInt(len(passwordLowerCaseLetters))
	if err != nil {
		return "", err
	}
	specialIDx, err := randInt(len(passwordSpecialChars))
	if err != nil {
		return "", err
	}

	// Make sure we are guaranteed at least one of each type to meet any possible password complexity requirements.
	password := string([]rune(passwordUpperCaseLetters)[upperIDx]) +
		string([]rune(passwordNumbers)[numberIDx]) +
		string([]rune(passwordLowerCaseLetters)[lowerIDx]) +
		string([]rune(passwordSpecialChars)[specialIDx])

	for len(password) < minimumLength {
		i, err := randInt(len(passwordAllChars))
		if err != nil {
			return "", err
		}
		password = password + string([]rune(passwordAllChars)[i])
	}

	return password, nil
}
