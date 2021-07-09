// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
	"net/http"
)

const (
	AccessTokenGrantType  = "authorization_code"
	AccessTokenType       = "bearer"
	RefreshTokenGrantType = "refresh_token"
)

type AccessData struct {
	ClientID     string `json:"client_id"`
	UserID       string `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	RedirectURI  string `json:"redirect_uri"`
	ExpiresAt    int64  `json:"expires_at"`
	Scope        string `json:"scope"`
}

type AccessResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int32  `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// IsValid validates the AccessData and returns an error if it isn't configured
// correctly.
func (ad *AccessData) IsValid() *AppError {

	if ad.ClientID == "" || len(ad.ClientID) > 26 {
		return NewAppError("AccessData.IsValid", "model.access.is_valid.client_id.app_error", nil, "", http.StatusBadRequest)
	}

	if ad.UserID == "" || len(ad.UserID) > 26 {
		return NewAppError("AccessData.IsValid", "model.access.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(ad.Token) != 26 {
		return NewAppError("AccessData.IsValid", "model.access.is_valid.access_token.app_error", nil, "", http.StatusBadRequest)
	}

	if len(ad.RefreshToken) > 26 {
		return NewAppError("AccessData.IsValid", "model.access.is_valid.refresh_token.app_error", nil, "", http.StatusBadRequest)
	}

	if ad.RedirectURI == "" || len(ad.RedirectURI) > 256 || !IsValidHTTPURL(ad.RedirectURI) {
		return NewAppError("AccessData.IsValid", "model.access.is_valid.redirect_uri.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (ad *AccessData) IsExpired() bool {

	if ad.ExpiresAt <= 0 {
		return false
	}

	if GetMillis() > ad.ExpiresAt {
		return true
	}

	return false
}

func (ad *AccessData) ToJSON() string {
	b, _ := json.Marshal(ad)
	return string(b)
}

func AccessDataFromJSON(data io.Reader) *AccessData {
	var ad *AccessData
	json.NewDecoder(data).Decode(&ad)
	return ad
}

func (ar *AccessResponse) ToJSON() string {
	b, _ := json.Marshal(ar)
	return string(b)
}

func AccessResponseFromJSON(data io.Reader) *AccessResponse {
	var ar *AccessResponse
	json.NewDecoder(data).Decode(&ar)
	return ar
}
