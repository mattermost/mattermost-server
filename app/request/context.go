// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package request

import (
	"context"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

type Context struct {
	t              i18n.TranslateFunc
	session        model.Session
	requestID      string
	ipAddress      string
	path           string
	userAgent      string
	acceptLanguage string

	context context.Context
}

func NewContext(ctx context.Context, requestID, ipAddress, path, userAgent, acceptLanguage string, session model.Session, t i18n.TranslateFunc) *Context {
	return &Context{
		t:              t,
		session:        session,
		requestID:      requestID,
		ipAddress:      ipAddress,
		path:           path,
		userAgent:      userAgent,
		acceptLanguage: acceptLanguage,
		context:        ctx,
	}
}

func EmptyContext() *Context {
	return &Context{
		t:       i18n.T,
		context: context.Background(),
	}
}

func (c *Context) T(translationID string, args ...interface{}) string {
	return c.t(translationID, args...)
}
func (c *Context) Session() *model.Session {
	return &c.session
}
func (c *Context) RequestID() string {
	return c.requestID
}
func (c *Context) IDAddress() string {
	return c.ipAddress
}
func (c *Context) Path() string {
	return c.path
}
func (c *Context) UserAgent() string {
	return c.userAgent
}
func (c *Context) AcceptLanguage() string {
	return c.acceptLanguage
}

func (c *Context) Context() context.Context {
	return c.context
}

func (c *Context) SetSession(s *model.Session) {
	c.session = *s
}

func (c *Context) SetT(t i18n.TranslateFunc) {
	c.t = t
}
func (c *Context) SetRequestID(s string) {
	c.requestID = s
}
func (c *Context) SetIDAddress(s string) {
	c.ipAddress = s
}
func (c *Context) SetUserAgent(s string) {
	c.userAgent = s
}
func (c *Context) SetAcceptLanguage(s string) {
	c.acceptLanguage = s
}
func (c *Context) SetPath(s string) {
	c.path = s
}
func (c *Context) SetContext(ctx context.Context) {
	c.context = ctx
}

func (c *Context) GetT() i18n.TranslateFunc {
	return c.t
}
