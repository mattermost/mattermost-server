// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package einterfaces

import (
	"io"

	"github.com/mattermost/mattermost-server/v5/model"
)

type OAuthProvider interface {
	GetUserFromJSON(data io.Reader, tokenUser *model.User) (*model.User, error)
	GetSSOSettings(config *model.Config, service string) (*model.SSOSettings, error)
	GetUserFromIDToken(idToken string) (*model.User, error)
	IsSameUser(dbUser, oAuthUser *model.User) bool
}

var oauthProviders = make(map[string]OAuthProvider)

func RegisterOAuthProvider(name string, newProvider OAuthProvider) {
	oauthProviders[name] = newProvider
}

func GetOAuthProvider(name string) OAuthProvider {
	provider, ok := oauthProviders[name]
	if ok {
		return provider
	}
	return nil
}
