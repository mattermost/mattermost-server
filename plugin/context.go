// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

// Context passes through metadata about the request or hook event.
// For requests this is built in app/plugin_requests.go
// For hooks, app.PluginContext() is called.
type Context struct {
	SessionId      string
	RequestId      string
	IpAddress      string
	AcceptLanguage string
	UserAgent      string
}
