// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package users

import "errors"

var (
	AcceptedDomainError       = errors.New("the email provided does not belong to an accepted domain")
	VerifyUserError           = errors.New("could not update verify email field")
	UserCountError            = errors.New("could not get the total number of the users.")
	UserCreationDisabledError = errors.New("user creation is not allowed")

	GetTokenError            = errors.New("could not get token")
	GetSessionError          = errors.New("could not get session")
	DeleteTokenError         = errors.New("could not delete token")
	DeleteSessionError       = errors.New("could not delete session")
	DeleteAllAccessDataError = errors.New("could not delete all access data")

	DefaultFontError   = errors.New("could not get default font")
	UserInitialsError  = errors.New("could not get user initials")
	ImageEncodingError = errors.New("could not encode image")
)

// ErrInvalidPassword indicates an error against the password settings
type ErrInvalidPassword struct {
	id string
}

func NewErrInvalidPassword(id string) *ErrInvalidPassword {
	return &ErrInvalidPassword{
		id: id,
	}
}

func (e *ErrInvalidPassword) Error() string {
	return "invalid password"
}

func (e *ErrInvalidPassword) ID() string {
	return e.id
}
