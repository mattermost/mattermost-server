// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"
)

const (
	OAuthActionSignup     = "signup"
	OAuthActionLogin      = "login"
	OAuthActionEmailToSSO = "email_to_sso"
	OAuthActionSSOToEmail = "sso_to_email"
	OAuthActionMobile     = "mobile"
)

type OAuthApp struct {
	ID           string      `json:"id"`
	CreatorID    string      `json:"creator_id"`
	CreateAt     int64       `json:"create_at"`
	UpdateAt     int64       `json:"update_at"`
	ClientSecret string      `json:"client_secret"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	IconURL      string      `json:"icon_url"`
	CallbackURLs StringArray `json:"callback_urls"`
	Homepage     string      `json:"homepage"`
	IsTrusted    bool        `json:"is_trusted"`
}

// IsValid validates the app and returns an error if it isn't configured
// correctly.
func (a *OAuthApp) IsValid() *AppError {

	if !IsValidID(a.ID) {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.app_id.app_error", nil, "", http.StatusBadRequest)
	}

	if a.CreateAt == 0 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.create_at.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if a.UpdateAt == 0 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.update_at.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if !IsValidID(a.CreatorID) {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.creator_id.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if a.ClientSecret == "" || len(a.ClientSecret) > 128 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.client_secret.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if a.Name == "" || len(a.Name) > 64 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.name.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if len(a.CallbackURLs) == 0 || len(fmt.Sprintf("%s", a.CallbackURLs)) > 1024 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.callback.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	for _, callback := range a.CallbackURLs {
		if !IsValidHttpURL(callback) {
			return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.callback.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if a.Homepage == "" || len(a.Homepage) > 256 || !IsValidHttpURL(a.Homepage) {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.homepage.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(a.Description) > 512 {
		return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.description.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
	}

	if a.IconURL != "" {
		if len(a.IconURL) > 512 || !IsValidHttpURL(a.IconURL) {
			return NewAppError("OAuthApp.IsValid", "model.oauth.is_valid.icon_url.app_error", nil, "app_id="+a.ID, http.StatusBadRequest)
		}
	}

	return nil
}

// PreSave will set the Id and ClientSecret if missing.  It will also fill
// in the CreateAt, UpdateAt times. It should be run before saving the app to the db.
func (a *OAuthApp) PreSave() {
	if a.ID == "" {
		a.ID = NewID()
	}

	if a.ClientSecret == "" {
		a.ClientSecret = NewID()
	}

	a.CreateAt = GetMillis()
	a.UpdateAt = a.CreateAt
}

// PreUpdate should be run before updating the app in the db.
func (a *OAuthApp) PreUpdate() {
	a.UpdateAt = GetMillis()
}

func (a *OAuthApp) ToJSON() string {
	b, _ := json.Marshal(a)
	return string(b)
}

// Generate a valid strong etag so the browser can cache the results
func (a *OAuthApp) Etag() string {
	return Etag(a.ID, a.UpdateAt)
}

// Remove any private data from the app object
func (a *OAuthApp) Sanitize() {
	a.ClientSecret = ""
}

func (a *OAuthApp) IsValidRedirectURL(url string) bool {
	for _, u := range a.CallbackURLs {
		if u == url {
			return true
		}
	}

	return false
}

func OAuthAppFromJSON(data io.Reader) *OAuthApp {
	var app *OAuthApp
	json.NewDecoder(data).Decode(&app)
	return app
}

func OAuthAppListToJSON(l []*OAuthApp) string {
	b, _ := json.Marshal(l)
	return string(b)
}

func OAuthAppListFromJSON(data io.Reader) []*OAuthApp {
	var o []*OAuthApp
	json.NewDecoder(data).Decode(&o)
	return o
}
