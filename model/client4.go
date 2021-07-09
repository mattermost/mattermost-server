// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderRequestID          = "X-Request-ID"
	HeaderVersionID          = "X-Version-ID"
	HeaderClusterID          = "X-Cluster-ID"
	HeaderEtagServer         = "ETag"
	HeaderEtagClient         = "If-None-Match"
	HeaderForwarded          = "X-Forwarded-For"
	HeaderRealID             = "X-Real-IP"
	HeaderForwardedProto     = "X-Forwarded-Proto"
	HeaderToken              = "token"
	HeaderCsrfToken          = "X-CSRF-Token"
	HeaderBearer             = "BEARER"
	HeaderAuth               = "Authorization"
	HeaderCloudToken         = "X-Cloud-Token"
	HeaderRemoteclusterToken = "X-RemoteCluster-Token"
	HeaderRemoteclusterID    = "X-RemoteCluster-Id"
	HeaderRequestedWith      = "X-Requested-With"
	HeaderRequestedWithXml   = "XMLHttpRequest"
	HeaderRange              = "Range"
	STATUS                   = "status"
	StatusOk                 = "OK"
	StatusFail               = "FAIL"
	StatusUnhealthy          = "UNHEALTHY"
	StatusRemove             = "REMOVE"

	ClientDir = "client"

	APIURLSuffixV1 = "/api/v1"
	APIURLSuffixV4 = "/api/v4"
	APIURLSuffix   = APIURLSuffixV4
)

type Response struct {
	StatusCode    int
	Error         *AppError
	RequestID     string
	Etag          string
	ServerVersion string
	Header        http.Header
}

type Client4 struct {
	URL        string       // The location of the server, for example  "http://localhost:8065"
	APIURL     string       // The api location of the server, for example "http://localhost:8065/api/v4"
	HttpClient *http.Client // The http client
	AuthToken  string
	AuthType   string
	HttpHeader map[string]string // Headers to be copied over for each request

	// TrueString is the string value sent to the server for true boolean query parameters.
	trueString string

	// FalseString is the string value sent to the server for false boolean query parameters.
	falseString string
}

// SetBoolString is a helper method for overriding how true and false query string parameters are
// sent to the server.
//
// This method is only exposed for testing. It is never necessary to configure these values
// in production.
func (c *Client4) SetBoolString(value bool, valueStr string) {
	if value {
		c.trueString = valueStr
	} else {
		c.falseString = valueStr
	}
}

// boolString builds the query string parameter for boolean values.
func (c *Client4) boolString(value bool) string {
	if value && c.trueString != "" {
		return c.trueString
	} else if !value && c.falseString != "" {
		return c.falseString
	}

	if value {
		return "true"
	}
	return "false"
}

func closeBody(r *http.Response) {
	if r.Body != nil {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}
}

// Must is a convenience function used for testing.
func (c *Client4) Must(result interface{}, resp *Response) interface{} {
	if resp.Error != nil {
		time.Sleep(time.Second)
		panic(resp.Error)
	}

	return result
}

func NewAPIv4Client(url string) *Client4 {
	url = strings.TrimRight(url, "/")
	return &Client4{url, url + APIURLSuffix, &http.Client{}, "", "", map[string]string{}, "", ""}
}

func NewAPIv4SocketClient(socketPath string) *Client4 {
	tr := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	client := NewAPIv4Client("http://_")
	client.HttpClient = &http.Client{Transport: tr}

	return client
}

func BuildErrorResponse(r *http.Response, err *AppError) *Response {
	var statusCode int
	var header http.Header
	if r != nil {
		statusCode = r.StatusCode
		header = r.Header
	} else {
		statusCode = 0
		header = make(http.Header)
	}

	return &Response{
		StatusCode: statusCode,
		Error:      err,
		Header:     header,
	}
}

func BuildResponse(r *http.Response) *Response {
	return &Response{
		StatusCode:    r.StatusCode,
		RequestID:     r.Header.Get(HeaderRequestID),
		Etag:          r.Header.Get(HeaderEtagServer),
		ServerVersion: r.Header.Get(HeaderVersionID),
		Header:        r.Header,
	}
}

func (c *Client4) SetToken(token string) {
	c.AuthToken = token
	c.AuthType = HeaderBearer
}

// MockSession is deprecated in favour of SetToken
func (c *Client4) MockSession(token string) {
	c.SetToken(token)
}

func (c *Client4) SetOAuthToken(token string) {
	c.AuthToken = token
	c.AuthType = HeaderToken
}

func (c *Client4) ClearOAuthToken() {
	c.AuthToken = ""
	c.AuthType = HeaderBearer
}

func (c *Client4) GetUsersRoute() string {
	return "/users"
}

func (c *Client4) GetUserRoute(userID string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/%v", userID)
}

func (c *Client4) GetUserThreadsRoute(userID, teamID string) string {
	return c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/threads"
}

func (c *Client4) GetUserThreadRoute(userID, teamID, threadID string) string {
	return c.GetUserThreadsRoute(userID, teamID) + "/" + threadID
}

func (c *Client4) GetUserCategoryRoute(userID, teamID string) string {
	return c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/channels/categories"
}

func (c *Client4) GetUserAccessTokensRoute() string {
	return fmt.Sprintf(c.GetUsersRoute() + "/tokens")
}

func (c *Client4) GetUserAccessTokenRoute(tokenID string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/tokens/%v", tokenID)
}

func (c *Client4) GetUserByUsernameRoute(userName string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/username/%v", userName)
}

func (c *Client4) GetUserByEmailRoute(email string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/email/%v", email)
}

func (c *Client4) GetBotsRoute() string {
	return "/bots"
}

func (c *Client4) GetBotRoute(botUserID string) string {
	return fmt.Sprintf("%s/%s", c.GetBotsRoute(), botUserID)
}

func (c *Client4) GetTeamsRoute() string {
	return "/teams"
}

func (c *Client4) GetTeamRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v", teamID)
}

func (c *Client4) GetTeamAutoCompleteCommandsRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v/commands/autocomplete", teamID)
}

func (c *Client4) GetTeamByNameRoute(teamName string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/name/%v", teamName)
}

func (c *Client4) GetTeamMemberRoute(teamID, userID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID)+"/members/%v", userID)
}

func (c *Client4) GetTeamMembersRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID) + "/members")
}

func (c *Client4) GetTeamStatsRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID) + "/stats")
}

func (c *Client4) GetTeamImportRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID) + "/import")
}

func (c *Client4) GetChannelsRoute() string {
	return "/channels"
}

func (c *Client4) GetChannelsForTeamRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID) + "/channels")
}

func (c *Client4) GetChannelRoute(channelID string) string {
	return fmt.Sprintf(c.GetChannelsRoute()+"/%v", channelID)
}

func (c *Client4) GetChannelByNameRoute(channelName, teamID string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamID)+"/channels/name/%v", channelName)
}

func (c *Client4) GetChannelsForTeamForUserRoute(teamID, userID string, includeDeleted bool) string {
	route := fmt.Sprintf(c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/channels")
	if includeDeleted {
		query := fmt.Sprintf("?include_deleted=%v", includeDeleted)
		return route + query
	}
	return route
}

func (c *Client4) GetChannelByNameForTeamNameRoute(channelName, teamName string) string {
	return fmt.Sprintf(c.GetTeamByNameRoute(teamName)+"/channels/name/%v", channelName)
}

func (c *Client4) GetChannelMembersRoute(channelID string) string {
	return fmt.Sprintf(c.GetChannelRoute(channelID) + "/members")
}

func (c *Client4) GetChannelMemberRoute(channelID, userID string) string {
	return fmt.Sprintf(c.GetChannelMembersRoute(channelID)+"/%v", userID)
}

func (c *Client4) GetPostsRoute() string {
	return "/posts"
}

func (c *Client4) GetPostsEphemeralRoute() string {
	return "/posts/ephemeral"
}

func (c *Client4) GetConfigRoute() string {
	return "/config"
}

func (c *Client4) GetLicenseRoute() string {
	return "/license"
}

func (c *Client4) GetPostRoute(postID string) string {
	return fmt.Sprintf(c.GetPostsRoute()+"/%v", postID)
}

func (c *Client4) GetFilesRoute() string {
	return "/files"
}

func (c *Client4) GetFileRoute(fileID string) string {
	return fmt.Sprintf(c.GetFilesRoute()+"/%v", fileID)
}

func (c *Client4) GetUploadsRoute() string {
	return "/uploads"
}

func (c *Client4) GetUploadRoute(uploadID string) string {
	return fmt.Sprintf("%s/%s", c.GetUploadsRoute(), uploadID)
}

func (c *Client4) GetPluginsRoute() string {
	return "/plugins"
}

func (c *Client4) GetPluginRoute(pluginID string) string {
	return fmt.Sprintf(c.GetPluginsRoute()+"/%v", pluginID)
}

func (c *Client4) GetSystemRoute() string {
	return "/system"
}

func (c *Client4) GetCloudRoute() string {
	return "/cloud"
}

func (c *Client4) GetTestEmailRoute() string {
	return "/email/test"
}

func (c *Client4) GetTestSiteURLRoute() string {
	return "/site_url/test"
}

func (c *Client4) GetTestS3Route() string {
	return "/file/s3_test"
}

func (c *Client4) GetDatabaseRoute() string {
	return "/database"
}

func (c *Client4) GetCacheRoute() string {
	return "/caches"
}

func (c *Client4) GetClusterRoute() string {
	return "/cluster"
}

func (c *Client4) GetIncomingWebhooksRoute() string {
	return "/hooks/incoming"
}

func (c *Client4) GetIncomingWebhookRoute(hookID string) string {
	return fmt.Sprintf(c.GetIncomingWebhooksRoute()+"/%v", hookID)
}

func (c *Client4) GetComplianceReportsRoute() string {
	return "/compliance/reports"
}

func (c *Client4) GetComplianceReportRoute(reportID string) string {
	return fmt.Sprintf("%s/%s", c.GetComplianceReportsRoute(), reportID)
}

func (c *Client4) GetComplianceReportDownloadRoute(reportID string) string {
	return fmt.Sprintf("%s/%s/download", c.GetComplianceReportsRoute(), reportID)
}

func (c *Client4) GetOutgoingWebhooksRoute() string {
	return "/hooks/outgoing"
}

func (c *Client4) GetOutgoingWebhookRoute(hookID string) string {
	return fmt.Sprintf(c.GetOutgoingWebhooksRoute()+"/%v", hookID)
}

func (c *Client4) GetPreferencesRoute(userID string) string {
	return fmt.Sprintf(c.GetUserRoute(userID) + "/preferences")
}

func (c *Client4) GetUserStatusRoute(userID string) string {
	return fmt.Sprintf(c.GetUserRoute(userID) + "/status")
}

func (c *Client4) GetUserStatusesRoute() string {
	return fmt.Sprintf(c.GetUsersRoute() + "/status")
}

func (c *Client4) GetSamlRoute() string {
	return "/saml"
}

func (c *Client4) GetLdapRoute() string {
	return "/ldap"
}

func (c *Client4) GetBrandRoute() string {
	return "/brand"
}

func (c *Client4) GetDataRetentionRoute() string {
	return "/data_retention"
}

func (c *Client4) GetDataRetentionPolicyRoute(policyID string) string {
	return fmt.Sprintf(c.GetDataRetentionRoute()+"/policies/%v", policyID)
}

func (c *Client4) GetElasticsearchRoute() string {
	return "/elasticsearch"
}

func (c *Client4) GetBleveRoute() string {
	return "/bleve"
}

func (c *Client4) GetCommandsRoute() string {
	return "/commands"
}

func (c *Client4) GetCommandRoute(commandID string) string {
	return fmt.Sprintf(c.GetCommandsRoute()+"/%v", commandID)
}

func (c *Client4) GetCommandMoveRoute(commandID string) string {
	return fmt.Sprintf(c.GetCommandsRoute()+"/%v/move", commandID)
}

func (c *Client4) GetEmojisRoute() string {
	return "/emoji"
}

func (c *Client4) GetEmojiRoute(emojiID string) string {
	return fmt.Sprintf(c.GetEmojisRoute()+"/%v", emojiID)
}

func (c *Client4) GetEmojiByNameRoute(name string) string {
	return fmt.Sprintf(c.GetEmojisRoute()+"/name/%v", name)
}

func (c *Client4) GetReactionsRoute() string {
	return "/reactions"
}

func (c *Client4) GetOAuthAppsRoute() string {
	return "/oauth/apps"
}

func (c *Client4) GetOAuthAppRoute(appID string) string {
	return fmt.Sprintf("/oauth/apps/%v", appID)
}

func (c *Client4) GetOpenGraphRoute() string {
	return "/opengraph"
}

func (c *Client4) GetJobsRoute() string {
	return "/jobs"
}

func (c *Client4) GetRolesRoute() string {
	return "/roles"
}

func (c *Client4) GetSchemesRoute() string {
	return "/schemes"
}

func (c *Client4) GetSchemeRoute(id string) string {
	return c.GetSchemesRoute() + fmt.Sprintf("/%v", id)
}

func (c *Client4) GetAnalyticsRoute() string {
	return "/analytics"
}

func (c *Client4) GetTimezonesRoute() string {
	return fmt.Sprintf(c.GetSystemRoute() + "/timezones")
}

func (c *Client4) GetChannelSchemeRoute(channelID string) string {
	return fmt.Sprintf(c.GetChannelsRoute()+"/%v/scheme", channelID)
}

func (c *Client4) GetTeamSchemeRoute(teamID string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v/scheme", teamID)
}

func (c *Client4) GetTotalUsersStatsRoute() string {
	return fmt.Sprintf(c.GetUsersRoute() + "/stats")
}

func (c *Client4) GetRedirectLocationRoute() string {
	return "/redirect_location"
}

func (c *Client4) GetServerBusyRoute() string {
	return "/server_busy"
}

func (c *Client4) GetUserTermsOfServiceRoute(userID string) string {
	return c.GetUserRoute(userID) + "/terms_of_service"
}

func (c *Client4) GetTermsOfServiceRoute() string {
	return "/terms_of_service"
}

func (c *Client4) GetGroupsRoute() string {
	return "/groups"
}

func (c *Client4) GetPublishUserTypingRoute(userID string) string {
	return c.GetUserRoute(userID) + "/typing"
}

func (c *Client4) GetGroupRoute(groupID string) string {
	return fmt.Sprintf("%s/%s", c.GetGroupsRoute(), groupID)
}

func (c *Client4) GetGroupSyncableRoute(groupID, syncableID string, syncableType GroupSyncableType) string {
	return fmt.Sprintf("%s/%ss/%s", c.GetGroupRoute(groupID), strings.ToLower(syncableType.String()), syncableID)
}

func (c *Client4) GetGroupSyncablesRoute(groupID string, syncableType GroupSyncableType) string {
	return fmt.Sprintf("%s/%ss", c.GetGroupRoute(groupID), strings.ToLower(syncableType.String()))
}

func (c *Client4) GetImportsRoute() string {
	return "/imports"
}

func (c *Client4) GetExportsRoute() string {
	return "/exports"
}

func (c *Client4) GetExportRoute(name string) string {
	return fmt.Sprintf(c.GetExportsRoute()+"/%v", name)
}

func (c *Client4) GetRemoteClusterRoute() string {
	return "/remotecluster"
}

func (c *Client4) GetSharedChannelsRoute() string {
	return "/sharedchannels"
}

func (c *Client4) GetPermissionsRoute() string {
	return "/permissions"
}

func (c *Client4) DoAPIGet(url string, etag string) (*http.Response, *AppError) {
	return c.DoAPIRequest(http.MethodGet, c.APIURL+url, "", etag)
}

func (c *Client4) DoAPIPost(url string, data string) (*http.Response, *AppError) {
	return c.DoAPIRequest(http.MethodPost, c.APIURL+url, data, "")
}

func (c *Client4) doAPIDeleteBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doAPIRequestBytes(http.MethodDelete, c.APIURL+url, data, "")
}

func (c *Client4) doAPIPatchBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doAPIRequestBytes(http.MethodPatch, c.APIURL+url, data, "")
}

func (c *Client4) doAPIPostBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doAPIRequestBytes(http.MethodPost, c.APIURL+url, data, "")
}

func (c *Client4) DoAPIPut(url string, data string) (*http.Response, *AppError) {
	return c.DoAPIRequest(http.MethodPut, c.APIURL+url, data, "")
}

func (c *Client4) doAPIPutBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doAPIRequestBytes(http.MethodPut, c.APIURL+url, data, "")
}

func (c *Client4) DoAPIDelete(url string) (*http.Response, *AppError) {
	return c.DoAPIRequest(http.MethodDelete, c.APIURL+url, "", "")
}

func (c *Client4) DoAPIRequest(method, url, data, etag string) (*http.Response, *AppError) {
	return c.doAPIRequestReader(method, url, strings.NewReader(data), map[string]string{HeaderEtagClient: etag})
}

func (c *Client4) DoAPIRequestWithHeaders(method, url, data string, headers map[string]string) (*http.Response, *AppError) {
	return c.doAPIRequestReader(method, url, strings.NewReader(data), headers)
}

func (c *Client4) doAPIRequestBytes(method, url string, data []byte, etag string) (*http.Response, *AppError) {
	return c.doAPIRequestReader(method, url, bytes.NewReader(data), map[string]string{HeaderEtagClient: etag})
}

func (c *Client4) doAPIRequestReader(method, url string, data io.Reader, headers map[string]string) (*http.Response, *AppError) {
	rq, err := http.NewRequest(method, url, data)
	if err != nil {
		return nil, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	for k, v := range headers {
		rq.Header.Set(k, v)
	}

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	if c.HttpHeader != nil && len(c.HttpHeader) > 0 {
		for k, v := range c.HttpHeader {
			rq.Header.Set(k, v)
		}
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0)
	}

	if rp.StatusCode == 304 {
		return rp, nil
	}

	if rp.StatusCode >= 300 {
		defer closeBody(rp)
		return rp, AppErrorFromJSON(rp.Body)
	}

	return rp, nil
}

func (c *Client4) DoUploadFile(url string, data []byte, contentType string) (*FileUploadResponse, *Response) {
	return c.doUploadFile(url, bytes.NewReader(data), contentType, 0)
}

func (c *Client4) doUploadFile(url string, body io.Reader, contentType string, contentLength int64) (*FileUploadResponse, *Response) {
	rq, err := http.NewRequest("POST", c.APIURL+url, body)
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	if contentLength != 0 {
		rq.ContentLength = contentLength
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return FileUploadResponseFromJSON(rp.Body), BuildResponse(rp)
}

func (c *Client4) DoEmojiUploadFile(url string, data []byte, contentType string) (*Emoji, *Response) {
	rq, err := http.NewRequest("POST", c.APIURL+url, bytes.NewReader(data))
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return EmojiFromJSON(rp.Body), BuildResponse(rp)
}

func (c *Client4) DoUploadImportTeam(url string, data []byte, contentType string) (map[string]string, *Response) {
	rq, err := http.NewRequest("POST", c.APIURL+url, bytes.NewReader(data))
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return MapFromJSON(rp.Body), BuildResponse(rp)
}

// CheckStatusOK is a convenience function for checking the standard OK response
// from the web service.
func CheckStatusOK(r *http.Response) bool {
	m := MapFromJSON(r.Body)
	defer closeBody(r)

	if m != nil && m[STATUS] == StatusOk {
		return true
	}

	return false
}

// Authentication Section

// LoginById authenticates a user by user id and password.
func (c *Client4) LoginByID(id string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["id"] = id
	m["password"] = password
	return c.login(m)
}

// Login authenticates a user by login id, which can be username, email or some sort
// of SSO identifier based on server configuration, and a password.
func (c *Client4) Login(loginID string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginID
	m["password"] = password
	return c.login(m)
}

// LoginByLdap authenticates a user by LDAP id and password.
func (c *Client4) LoginByLdap(loginID string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginID
	m["password"] = password
	m["ldap_only"] = c.boolString(true)
	return c.login(m)
}

// LoginWithDevice authenticates a user by login id (username, email or some sort
// of SSO identifier based on configuration), password and attaches a device id to
// the session.
func (c *Client4) LoginWithDevice(loginID string, password string, deviceID string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginID
	m["password"] = password
	m["device_id"] = deviceID
	return c.login(m)
}

// LoginWithMFA logs a user in with a MFA token
func (c *Client4) LoginWithMFA(loginID, password, mfaToken string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginID
	m["password"] = password
	m["token"] = mfaToken
	return c.login(m)
}

func (c *Client4) login(m map[string]string) (*User, *Response) {
	r, err := c.DoAPIPost("/users/login", MapToJSON(m))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	c.AuthToken = r.Header.Get(HeaderToken)
	c.AuthType = HeaderBearer
	return UserFromJSON(r.Body), BuildResponse(r)
}

// Logout terminates the current user's session.
func (c *Client4) Logout() (bool, *Response) {
	r, err := c.DoAPIPost("/users/logout", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	c.AuthToken = ""
	c.AuthType = HeaderBearer
	return CheckStatusOK(r), BuildResponse(r)
}

// SwitchAccountType changes a user's login type from one type to another.
func (c *Client4) SwitchAccountType(switchRequest *SwitchRequest) (string, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/login/switch", switchRequest.ToJSON())
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["follow_link"], BuildResponse(r)
}

// User Section

// CreateUser creates a user in the system based on the provided user struct.
func (c *Client4) CreateUser(user *User) (*User, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute(), user.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// CreateUserWithToken creates a user in the system based on the provided tokenId.
func (c *Client4) CreateUserWithToken(user *User, tokenID string) (*User, *Response) {
	if tokenID == "" {
		err := NewAppError("MissingHashOrData", "api.user.create_user.missing_token.app_error", nil, "", http.StatusBadRequest)
		return nil, &Response{StatusCode: err.StatusCode, Error: err}
	}

	query := fmt.Sprintf("?t=%v", tokenID)
	r, err := c.DoAPIPost(c.GetUsersRoute()+query, user.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return UserFromJSON(r.Body), BuildResponse(r)
}

// CreateUserWithInviteId creates a user in the system based on the provided invited id.
func (c *Client4) CreateUserWithInviteID(user *User, inviteID string) (*User, *Response) {
	if inviteID == "" {
		err := NewAppError("MissingInviteId", "api.user.create_user.missing_invite_id.app_error", nil, "", http.StatusBadRequest)
		return nil, &Response{StatusCode: err.StatusCode, Error: err}
	}

	query := fmt.Sprintf("?iid=%v", url.QueryEscape(inviteID))
	r, err := c.DoAPIPost(c.GetUsersRoute()+query, user.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return UserFromJSON(r.Body), BuildResponse(r)
}

// GetMe returns the logged in user.
func (c *Client4) GetMe(etag string) (*User, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(Me), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// GetUser returns a user based on the provided user id string.
func (c *Client4) GetUser(userID, etag string) (*User, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// GetUserByUsername returns a user based on the provided user name string.
func (c *Client4) GetUserByUsername(userName, etag string) (*User, *Response) {
	r, err := c.DoAPIGet(c.GetUserByUsernameRoute(userName), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// GetUserByEmail returns a user based on the provided user email string.
func (c *Client4) GetUserByEmail(email, etag string) (*User, *Response) {
	r, err := c.DoAPIGet(c.GetUserByEmailRoute(email), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// AutocompleteUsersInTeam returns the users on a team based on search term.
func (c *Client4) AutocompleteUsersInTeam(teamID string, username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?in_team=%v&name=%v&limit=%d", teamID, username, limit)
	r, err := c.DoAPIGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJSON(r.Body), BuildResponse(r)
}

// AutocompleteUsersInChannel returns the users in a channel based on search term.
func (c *Client4) AutocompleteUsersInChannel(teamID string, channelID string, username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?in_team=%v&in_channel=%v&name=%v&limit=%d", teamID, channelID, username, limit)
	r, err := c.DoAPIGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJSON(r.Body), BuildResponse(r)
}

// AutocompleteUsers returns the users in the system based on search term.
func (c *Client4) AutocompleteUsers(username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?name=%v&limit=%d", username, limit)
	r, err := c.DoAPIGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJSON(r.Body), BuildResponse(r)
}

// GetDefaultProfileImage gets the default user's profile image. Must be logged in.
func (c *Client4) GetDefaultProfileImage(userID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetUserRoute(userID)+"/image/default", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetDefaultProfileImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}

	return data, BuildResponse(r)
}

// GetProfileImage gets user's profile image. Must be logged in.
func (c *Client4) GetProfileImage(userID, etag string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetUserRoute(userID)+"/image", etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetProfileImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// GetUsers returns a page of users on the system. Page counting starts at 0.
func (c *Client4) GetUsers(page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetUsersInTeam(teamID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_team=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetNewUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetNewUsersInTeam(teamID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?sort=create_at&in_team=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetRecentlyActiveUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetRecentlyActiveUsersInTeam(teamID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?sort=last_activity_at&in_team=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetActiveUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetActiveUsersInTeam(teamID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?active=true&in_team=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersNotInTeam returns a page of users who are not in a team. Page counting starts at 0.
func (c *Client4) GetUsersNotInTeam(teamID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?not_in_team=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersInChannel returns a page of users in a channel. Page counting starts at 0.
func (c *Client4) GetUsersInChannel(channelID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_channel=%v&page=%v&per_page=%v", channelID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersInChannelByStatus returns a page of users in a channel. Page counting starts at 0. Sorted by Status
func (c *Client4) GetUsersInChannelByStatus(channelID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_channel=%v&page=%v&per_page=%v&sort=status", channelID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersNotInChannel returns a page of users not in a channel. Page counting starts at 0.
func (c *Client4) GetUsersNotInChannel(teamID, channelID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_team=%v&not_in_channel=%v&page=%v&per_page=%v", teamID, channelID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersWithoutTeam returns a page of users on the system that aren't on any teams. Page counting starts at 0.
func (c *Client4) GetUsersWithoutTeam(page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?without_team=1&page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersInGroup returns a page of users in a group. Page counting starts at 0.
func (c *Client4) GetUsersInGroup(groupID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_group=%v&page=%v&per_page=%v", groupID, page, perPage)
	r, err := c.DoAPIGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersByIds returns a list of users based on the provided user ids.
func (c *Client4) GetUsersByIDs(userIDs []string) ([]*User, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/ids", ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersByIds returns a list of users based on the provided user ids.
func (c *Client4) GetUsersByIDsWithOptions(userIDs []string, options *UserGetByIDsOptions) ([]*User, *Response) {
	v := url.Values{}
	if options.Since != 0 {
		v.Set("since", fmt.Sprintf("%d", options.Since))
	}

	url := c.GetUsersRoute() + "/ids"
	if len(v) > 0 {
		url += "?" + v.Encode()
	}

	r, err := c.DoAPIPost(url, ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersByUsernames returns a list of users based on the provided usernames.
func (c *Client4) GetUsersByUsernames(usernames []string) ([]*User, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/usernames", ArrayToJSON(usernames))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// GetUsersByGroupChannelIds returns a map with channel ids as keys
// and a list of users as values based on the provided user ids.
func (c *Client4) GetUsersByGroupChannelIDs(groupChannelIDs []string) (map[string][]*User, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/group_channels", ArrayToJSON(groupChannelIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	usersByChannelID := map[string][]*User{}
	json.NewDecoder(r.Body).Decode(&usersByChannelID)
	return usersByChannelID, BuildResponse(r)
}

// SearchUsers returns a list of users based on some search criteria.
func (c *Client4) SearchUsers(search *UserSearch) ([]*User, *Response) {
	r, err := c.doAPIPostBytes(c.GetUsersRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJSON(r.Body), BuildResponse(r)
}

// UpdateUser updates a user in the system based on the provided user struct.
func (c *Client4) UpdateUser(user *User) (*User, *Response) {
	r, err := c.DoAPIPut(c.GetUserRoute(user.ID), user.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// PatchUser partially updates a user in the system. Any missing fields are not updated.
func (c *Client4) PatchUser(userID string, patch *UserPatch) (*User, *Response) {
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/patch", patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// UpdateUserAuth updates a user AuthData (uthData, authService and password) in the system.
func (c *Client4) UpdateUserAuth(userID string, userAuth *UserAuth) (*UserAuth, *Response) {
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/auth", userAuth.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAuthFromJSON(r.Body), BuildResponse(r)
}

// UpdateUserMfa activates multi-factor authentication for a user if activate
// is true and a valid code is provided. If activate is false, then code is not
// required and multi-factor authentication is disabled for the user.
func (c *Client4) UpdateUserMfa(userID, code string, activate bool) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["activate"] = activate
	requestBody["code"] = code

	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/mfa", StringInterfaceToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// CheckUserMfa checks whether a user has MFA active on their account or not based on the
// provided login id.
// Deprecated: Clients should use Login method and check for MFA Error
func (c *Client4) CheckUserMfa(loginID string) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["login_id"] = loginID
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/mfa", StringInterfaceToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	data := StringInterfaceFromJSON(r.Body)
	mfaRequired, ok := data["mfa_required"].(bool)
	if !ok {
		return false, BuildResponse(r)
	}
	return mfaRequired, BuildResponse(r)
}

// GenerateMfaSecret will generate a new MFA secret for a user and return it as a string and
// as a base64 encoded image QR code.
func (c *Client4) GenerateMfaSecret(userID string) (*MfaSecret, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/mfa/generate", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MfaSecretFromJSON(r.Body), BuildResponse(r)
}

// UpdateUserPassword updates a user's password. Must be logged in as the user or be a system administrator.
func (c *Client4) UpdateUserPassword(userID, currentPassword, newPassword string) (bool, *Response) {
	requestBody := map[string]string{"current_password": currentPassword, "new_password": newPassword}
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/password", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserHashedPassword updates a user's password with an already-hashed password. Must be a system administrator.
func (c *Client4) UpdateUserHashedPassword(userID, newHashedPassword string) (bool, *Response) {
	requestBody := map[string]string{"already_hashed": "true", "new_password": newHashedPassword}
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/password", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PromoteGuestToUser convert a guest into a regular user
func (c *Client4) PromoteGuestToUser(guestID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(guestID)+"/promote", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DemoteUserToGuest convert a regular user into a guest
func (c *Client4) DemoteUserToGuest(guestID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(guestID)+"/demote", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserRoles updates a user's roles in the system. A user can have "system_user" and "system_admin" roles.
func (c *Client4) UpdateUserRoles(userID, roles string) (bool, *Response) {
	requestBody := map[string]string{"roles": roles}
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/roles", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserActive updates status of a user whether active or not.
func (c *Client4) UpdateUserActive(userID string, active bool) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["active"] = active
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/active", StringInterfaceToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteUser deactivates a user in the system based on the provided user id string.
func (c *Client4) DeleteUser(userID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetUserRoute(userID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteUser deletes a user in the system based on the provided user id string.
func (c *Client4) PermanentDeleteUser(userID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetUserRoute(userID) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ConvertUserToBot converts a user to a bot user.
func (c *Client4) ConvertUserToBot(userID string) (*Bot, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/convert_to_bot", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// ConvertBotToUser converts a bot user to a user.
func (c *Client4) ConvertBotToUser(userID string, userPatch *UserPatch, setSystemAdmin bool) (*User, *Response) {
	var query string
	if setSystemAdmin {
		query = "?set_system_admin=true"
	}
	r, err := c.DoAPIPost(c.GetBotRoute(userID)+"/convert_to_user"+query, userPatch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// PermanentDeleteAll permanently deletes all users in the system. This is a local only endpoint
func (c *Client4) PermanentDeleteAllUsers() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetUsersRoute())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SendPasswordResetEmail will send a link for password resetting to a user with the
// provided email.
func (c *Client4) SendPasswordResetEmail(email string) (bool, *Response) {
	requestBody := map[string]string{"email": email}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/password/reset/send", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ResetPassword uses a recovery code to update reset a user's password.
func (c *Client4) ResetPassword(token, newPassword string) (bool, *Response) {
	requestBody := map[string]string{"token": token, "new_password": newPassword}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/password/reset", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetSessions returns a list of sessions based on the provided user id string.
func (c *Client4) GetSessions(userID, etag string) ([]*Session, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/sessions", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SessionsFromJSON(r.Body), BuildResponse(r)
}

// RevokeSession revokes a user session based on the provided user id and session id strings.
func (c *Client4) RevokeSession(userID, sessionID string) (bool, *Response) {
	requestBody := map[string]string{"session_id": sessionID}
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/sessions/revoke", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RevokeAllSessions revokes all sessions for the provided user id string.
func (c *Client4) RevokeAllSessions(userID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/sessions/revoke/all", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RevokeAllSessions revokes all sessions for all the users.
func (c *Client4) RevokeSessionsFromAllUsers() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/sessions/revoke/all", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AttachDeviceId attaches a mobile device ID to the current session.
func (c *Client4) AttachDeviceID(deviceID string) (bool, *Response) {
	requestBody := map[string]string{"device_id": deviceID}
	r, err := c.DoAPIPut(c.GetUsersRoute()+"/sessions/device", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamsUnreadForUser will return an array with TeamUnread objects that contain the amount
// of unread messages and mentions the current user has for the teams it belongs to.
// An optional team ID can be set to exclude that team from the results. Must be authenticated.
func (c *Client4) GetTeamsUnreadForUser(userID, teamIDToExclude string) ([]*TeamUnread, *Response) {
	var optional string
	if teamIDToExclude != "" {
		optional += fmt.Sprintf("?exclude_team=%s", url.QueryEscape(teamIDToExclude))
	}

	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/teams/unread"+optional, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamsUnreadFromJSON(r.Body), BuildResponse(r)
}

// GetUserAudits returns a list of audit based on the provided user id string.
func (c *Client4) GetUserAudits(userID string, page int, perPage int, etag string) (Audits, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/audits"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AuditsFromJSON(r.Body), BuildResponse(r)
}

// VerifyUserEmail will verify a user's email using the supplied token.
func (c *Client4) VerifyUserEmail(token string) (bool, *Response) {
	requestBody := map[string]string{"token": token}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/email/verify", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// VerifyUserEmailWithoutToken will verify a user's email by its Id. (Requires manage system role)
func (c *Client4) VerifyUserEmailWithoutToken(userID string) (*User, *Response) {
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/email/verify/member", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJSON(r.Body), BuildResponse(r)
}

// SendVerificationEmail will send an email to the user with the provided email address, if
// that user exists. The email will contain a link that can be used to verify the user's
// email address.
func (c *Client4) SendVerificationEmail(email string) (bool, *Response) {
	requestBody := map[string]string{"email": email}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/email/verify/send", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SetDefaultProfileImage resets the profile image to a default generated one.
func (c *Client4) SetDefaultProfileImage(userID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetUserRoute(userID) + "/image")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	return CheckStatusOK(r), BuildResponse(r)
}

// SetProfileImage sets profile image of the user.
func (c *Client4) SetProfileImage(userID string, data []byte) (bool, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "profile.png")
	if err != nil {
		return false, &Response{Error: NewAppError("SetProfileImage", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return false, &Response{Error: NewAppError("SetProfileImage", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err = writer.Close(); err != nil {
		return false, &Response{Error: NewAppError("SetProfileImage", "model.client.set_profile_user.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetUserRoute(userID)+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetProfileImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetUserRoute(userID)+"/image", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// CreateUserAccessToken will generate a user access token that can be used in place
// of a session token to access the REST API. Must have the 'create_user_access_token'
// permission and if generating for another user, must have the 'edit_other_users'
// permission. A non-blank description is required.
func (c *Client4) CreateUserAccessToken(userID, description string) (*UserAccessToken, *Response) {
	requestBody := map[string]string{"description": description}
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+"/tokens", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenFromJSON(r.Body), BuildResponse(r)
}

// GetUserAccessTokens will get a page of access tokens' id, description, is_active
// and the user_id in the system. The actual token will not be returned. Must have
// the 'manage_system' permission.
func (c *Client4) GetUserAccessTokens(page int, perPage int) ([]*UserAccessToken, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUserAccessTokensRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJSON(r.Body), BuildResponse(r)
}

// GetUserAccessToken will get a user access tokens' id, description, is_active
// and the user_id of the user it is for. The actual token will not be returned.
// Must have the 'read_user_access_token' permission and if getting for another
// user, must have the 'edit_other_users' permission.
func (c *Client4) GetUserAccessToken(tokenID string) (*UserAccessToken, *Response) {
	r, err := c.DoAPIGet(c.GetUserAccessTokenRoute(tokenID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenFromJSON(r.Body), BuildResponse(r)
}

// GetUserAccessTokensForUser will get a paged list of user access tokens showing id,
// description and user_id for each. The actual tokens will not be returned. Must have
// the 'read_user_access_token' permission and if getting for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) GetUserAccessTokensForUser(userID string, page, perPage int) ([]*UserAccessToken, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/tokens"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJSON(r.Body), BuildResponse(r)
}

// RevokeUserAccessToken will revoke a user access token by id. Must have the
// 'revoke_user_access_token' permission and if revoking for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) RevokeUserAccessToken(tokenID string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenID}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/tokens/revoke", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SearchUserAccessTokens returns user access tokens matching the provided search term.
func (c *Client4) SearchUserAccessTokens(search *UserAccessTokenSearch) ([]*UserAccessToken, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/tokens/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJSON(r.Body), BuildResponse(r)
}

// DisableUserAccessToken will disable a user access token by id. Must have the
// 'revoke_user_access_token' permission and if disabling for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) DisableUserAccessToken(tokenID string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenID}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/tokens/disable", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// EnableUserAccessToken will enable a user access token by id. Must have the
// 'create_user_access_token' permission and if enabling for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) EnableUserAccessToken(tokenID string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenID}
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/tokens/enable", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Bots section

// CreateBot creates a bot in the system based on the provided bot struct.
func (c *Client4) CreateBot(bot *Bot) (*Bot, *Response) {
	r, err := c.doAPIPostBytes(c.GetBotsRoute(), bot.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// PatchBot partially updates a bot. Any missing fields are not updated.
func (c *Client4) PatchBot(userID string, patch *BotPatch) (*Bot, *Response) {
	r, err := c.doAPIPutBytes(c.GetBotRoute(userID), patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// GetBot fetches the given, undeleted bot.
func (c *Client4) GetBot(userID string, etag string) (*Bot, *Response) {
	r, err := c.DoAPIGet(c.GetBotRoute(userID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// GetBot fetches the given bot, even if it is deleted.
func (c *Client4) GetBotIncludeDeleted(userID string, etag string) (*Bot, *Response) {
	r, err := c.DoAPIGet(c.GetBotRoute(userID)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// GetBots fetches the given page of bots, excluding deleted.
func (c *Client4) GetBots(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJSON(r.Body), BuildResponse(r)
}

// GetBotsIncludeDeleted fetches the given page of bots, including deleted.
func (c *Client4) GetBotsIncludeDeleted(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_deleted="+c.boolString(true), page, perPage)
	r, err := c.DoAPIGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJSON(r.Body), BuildResponse(r)
}

// GetBotsOrphaned fetches the given page of bots, only including orphanded bots.
func (c *Client4) GetBotsOrphaned(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&only_orphaned="+c.boolString(true), page, perPage)
	r, err := c.DoAPIGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJSON(r.Body), BuildResponse(r)
}

// DisableBot disables the given bot in the system.
func (c *Client4) DisableBot(botUserID string) (*Bot, *Response) {
	r, err := c.doAPIPostBytes(c.GetBotRoute(botUserID)+"/disable", nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// EnableBot disables the given bot in the system.
func (c *Client4) EnableBot(botUserID string) (*Bot, *Response) {
	r, err := c.doAPIPostBytes(c.GetBotRoute(botUserID)+"/enable", nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// AssignBot assigns the given bot to the given user
func (c *Client4) AssignBot(botUserID, newOwnerID string) (*Bot, *Response) {
	r, err := c.doAPIPostBytes(c.GetBotRoute(botUserID)+"/assign/"+newOwnerID, nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJSON(r.Body), BuildResponse(r)
}

// SetBotIconImage sets LHS bot icon image.
func (c *Client4) SetBotIconImage(botUserID string, data []byte) (bool, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "icon.svg")
	if err != nil {
		return false, &Response{Error: NewAppError("SetBotIconImage", "model.client.set_bot_icon_image.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return false, &Response{Error: NewAppError("SetBotIconImage", "model.client.set_bot_icon_image.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err = writer.Close(); err != nil {
		return false, &Response{Error: NewAppError("SetBotIconImage", "model.client.set_bot_icon_image.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetBotRoute(botUserID)+"/icon", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetBotIconImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetBotRoute(botUserID)+"/icon", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// GetBotIconImage gets LHS bot icon image. Must be logged in.
func (c *Client4) GetBotIconImage(botUserID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetBotRoute(botUserID)+"/icon", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetBotIconImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// DeleteBotIconImage deletes LHS bot icon image. Must be logged in.
func (c *Client4) DeleteBotIconImage(botUserID string) (bool, *Response) {
	r, appErr := c.DoAPIDelete(c.GetBotRoute(botUserID) + "/icon")
	if appErr != nil {
		return false, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Team Section

// CreateTeam creates a team in the system based on the provided team struct.
func (c *Client4) CreateTeam(team *Team) (*Team, *Response) {
	r, err := c.DoAPIPost(c.GetTeamsRoute(), team.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// GetTeam returns a team based on the provided team id string.
func (c *Client4) GetTeam(teamID, etag string) (*Team, *Response) {
	r, err := c.DoAPIGet(c.GetTeamRoute(teamID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// GetAllTeams returns all teams based on permissions.
func (c *Client4) GetAllTeams(etag string, page int, perPage int) ([]*Team, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJSON(r.Body), BuildResponse(r)
}

// GetAllTeamsWithTotalCount returns all teams based on permissions.
func (c *Client4) GetAllTeamsWithTotalCount(etag string, page int, perPage int) ([]*Team, int64, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_total_count="+c.boolString(true), page, perPage)
	r, err := c.DoAPIGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	teamsListWithCount := TeamsWithCountFromJSON(r.Body)
	return teamsListWithCount.Teams, teamsListWithCount.TotalCount, BuildResponse(r)
}

// GetAllTeamsExcludePolicyConstrained returns all teams which are not part of a data retention policy.
// Must be a system administrator.
func (c *Client4) GetAllTeamsExcludePolicyConstrained(etag string, page int, perPage int) ([]*Team, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&exclude_policy_constrained=%v", page, perPage, true)
	r, err := c.DoAPIGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJSON(r.Body), BuildResponse(r)
}

// GetTeamByName returns a team based on the provided team name string.
func (c *Client4) GetTeamByName(name, etag string) (*Team, *Response) {
	r, err := c.DoAPIGet(c.GetTeamByNameRoute(name), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// SearchTeams returns teams matching the provided search term.
func (c *Client4) SearchTeams(search *TeamSearch) ([]*Team, *Response) {
	r, err := c.DoAPIPost(c.GetTeamsRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJSON(r.Body), BuildResponse(r)
}

// SearchTeamsPaged returns a page of teams and the total count matching the provided search term.
func (c *Client4) SearchTeamsPaged(search *TeamSearch) ([]*Team, int64, *Response) {
	if search.Page == nil {
		search.Page = NewInt(0)
	}
	if search.PerPage == nil {
		search.PerPage = NewInt(100)
	}
	r, err := c.DoAPIPost(c.GetTeamsRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	twc := TeamsWithCountFromJSON(r.Body)
	return twc.Teams, twc.TotalCount, BuildResponse(r)
}

// TeamExists returns true or false if the team exist or not.
func (c *Client4) TeamExists(name, etag string) (bool, *Response) {
	r, err := c.DoAPIGet(c.GetTeamByNameRoute(name)+"/exists", etag)
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapBoolFromJSON(r.Body)["exists"], BuildResponse(r)
}

// GetTeamsForUser returns a list of teams a user is on. Must be logged in as the user
// or be a system administrator.
func (c *Client4) GetTeamsForUser(userID, etag string) ([]*Team, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/teams", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJSON(r.Body), BuildResponse(r)
}

// GetTeamMember returns a team member based on the provided team and user id strings.
func (c *Client4) GetTeamMember(teamID, userID, etag string) (*TeamMember, *Response) {
	r, err := c.DoAPIGet(c.GetTeamMemberRoute(teamID, userID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJSON(r.Body), BuildResponse(r)
}

// UpdateTeamMemberRoles will update the roles on a team for a user.
func (c *Client4) UpdateTeamMemberRoles(teamID, userID, newRoles string) (bool, *Response) {
	requestBody := map[string]string{"roles": newRoles}
	r, err := c.DoAPIPut(c.GetTeamMemberRoute(teamID, userID)+"/roles", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamMemberSchemeRoles will update the scheme-derived roles on a team for a user.
func (c *Client4) UpdateTeamMemberSchemeRoles(teamID string, userID string, schemeRoles *SchemeRoles) (bool, *Response) {
	r, err := c.DoAPIPut(c.GetTeamMemberRoute(teamID, userID)+"/schemeRoles", schemeRoles.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeam will update a team.
func (c *Client4) UpdateTeam(team *Team) (*Team, *Response) {
	r, err := c.DoAPIPut(c.GetTeamRoute(team.ID), team.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// PatchTeam partially updates a team. Any missing fields are not updated.
func (c *Client4) PatchTeam(teamID string, patch *TeamPatch) (*Team, *Response) {
	r, err := c.DoAPIPut(c.GetTeamRoute(teamID)+"/patch", patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// RestoreTeam restores a previously deleted team.
func (c *Client4) RestoreTeam(teamID string) (*Team, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/restore", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// RegenerateTeamInviteId requests a new invite ID to be generated.
func (c *Client4) RegenerateTeamInviteID(teamID string) (*Team, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/regenerate_invite_id", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// SoftDeleteTeam deletes the team softly (archive only, not permanent delete).
func (c *Client4) SoftDeleteTeam(teamID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetTeamRoute(teamID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteTeam deletes the team, should only be used when needed for
// compliance and the like.
func (c *Client4) PermanentDeleteTeam(teamID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetTeamRoute(teamID) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamPrivacy modifies the team type (model.TeamOpen <--> model.TeamInvite) and sets
// the corresponding AllowOpenInvite appropriately.
func (c *Client4) UpdateTeamPrivacy(teamID string, privacy string) (*Team, *Response) {
	requestBody := map[string]string{"privacy": privacy}
	r, err := c.DoAPIPut(c.GetTeamRoute(teamID)+"/privacy", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// GetTeamMembers returns team members based on the provided team id string.
func (c *Client4) GetTeamMembers(teamID string, page int, perPage int, etag string) ([]*TeamMember, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetTeamMembersRoute(teamID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJSON(r.Body), BuildResponse(r)
}

// GetTeamMembersWithoutDeletedUsers returns team members based on the provided team id string. Additional parameters of sort and exclude_deleted_users accepted as well
// Could not add it to above function due to it be a breaking change.
func (c *Client4) GetTeamMembersSortAndWithoutDeletedUsers(teamID string, page int, perPage int, sort string, excludeDeletedUsers bool, etag string) ([]*TeamMember, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&sort=%v&exclude_deleted_users=%v", page, perPage, sort, excludeDeletedUsers)
	r, err := c.DoAPIGet(c.GetTeamMembersRoute(teamID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJSON(r.Body), BuildResponse(r)
}

// GetTeamMembersForUser returns the team members for a user.
func (c *Client4) GetTeamMembersForUser(userID string, etag string) ([]*TeamMember, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/teams/members", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJSON(r.Body), BuildResponse(r)
}

// GetTeamMembersByIds will return an array of team members based on the
// team id and a list of user ids provided. Must be authenticated.
func (c *Client4) GetTeamMembersByIDs(teamID string, userIDs []string) ([]*TeamMember, *Response) {
	r, err := c.DoAPIPost(fmt.Sprintf("/teams/%v/members/ids", teamID), ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJSON(r.Body), BuildResponse(r)
}

// AddTeamMember adds user to a team and return a team member.
func (c *Client4) AddTeamMember(teamID, userID string) (*TeamMember, *Response) {
	member := &TeamMember{TeamID: teamID, UserID: userID}
	r, err := c.DoAPIPost(c.GetTeamMembersRoute(teamID), member.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJSON(r.Body), BuildResponse(r)
}

// AddTeamMemberFromInvite adds a user to a team and return a team member using an invite id
// or an invite token/data pair.
func (c *Client4) AddTeamMemberFromInvite(token, inviteID string) (*TeamMember, *Response) {
	var query string

	if inviteID != "" {
		query += fmt.Sprintf("?invite_id=%v", inviteID)
	}

	if token != "" {
		query += fmt.Sprintf("?token=%v", token)
	}

	r, err := c.DoAPIPost(c.GetTeamsRoute()+"/members/invite"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJSON(r.Body), BuildResponse(r)
}

// AddTeamMembers adds a number of users to a team and returns the team members.
func (c *Client4) AddTeamMembers(teamID string, userIDs []string) ([]*TeamMember, *Response) {
	var members []*TeamMember
	for _, userID := range userIDs {
		member := &TeamMember{TeamID: teamID, UserID: userID}
		members = append(members, member)
	}

	r, err := c.DoAPIPost(c.GetTeamMembersRoute(teamID)+"/batch", TeamMembersToJSON(members))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJSON(r.Body), BuildResponse(r)
}

// AddTeamMembers adds a number of users to a team and returns the team members.
func (c *Client4) AddTeamMembersGracefully(teamID string, userIDs []string) ([]*TeamMemberWithError, *Response) {
	var members []*TeamMember
	for _, userID := range userIDs {
		member := &TeamMember{TeamID: teamID, UserID: userID}
		members = append(members, member)
	}

	r, err := c.DoAPIPost(c.GetTeamMembersRoute(teamID)+"/batch?graceful="+c.boolString(true), TeamMembersToJSON(members))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersWithErrorFromJSON(r.Body), BuildResponse(r)
}

// RemoveTeamMember will remove a user from a team.
func (c *Client4) RemoveTeamMember(teamID, userID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetTeamMemberRoute(teamID, userID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamStats returns a team stats based on the team id string.
// Must be authenticated.
func (c *Client4) GetTeamStats(teamID, etag string) (*TeamStats, *Response) {
	r, err := c.DoAPIGet(c.GetTeamStatsRoute(teamID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamStatsFromJSON(r.Body), BuildResponse(r)
}

// GetTotalUsersStats returns a total system user stats.
// Must be authenticated.
func (c *Client4) GetTotalUsersStats(etag string) (*UsersStats, *Response) {
	r, err := c.DoAPIGet(c.GetTotalUsersStatsRoute(), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UsersStatsFromJSON(r.Body), BuildResponse(r)
}

// GetTeamUnread will return a TeamUnread object that contains the amount of
// unread messages and mentions the user has for the specified team.
// Must be authenticated.
func (c *Client4) GetTeamUnread(teamID, userID string) (*TeamUnread, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+c.GetTeamRoute(teamID)+"/unread", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamUnreadFromJSON(r.Body), BuildResponse(r)
}

// ImportTeam will import an exported team from other app into a existing team.
func (c *Client4) ImportTeam(data []byte, filesize int, importFrom, filename, teamID string) (map[string]string, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	part, err = writer.CreateFormField("filesize")
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.file_size.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, strings.NewReader(strconv.Itoa(filesize))); err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.file_size.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	part, err = writer.CreateFormField("importFrom")
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.import_from.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err := io.Copy(part, strings.NewReader(importFrom)); err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.import_from.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err := writer.Close(); err != nil {
		return nil, &Response{Error: NewAppError("UploadImportTeam", "model.client.upload_post_attachment.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	return c.DoUploadImportTeam(c.GetTeamImportRoute(teamID), body.Bytes(), writer.FormDataContentType())
}

// InviteUsersToTeam invite users by email to the team.
func (c *Client4) InviteUsersToTeam(teamID string, userEmails []string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/invite/email", ArrayToJSON(userEmails))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InviteGuestsToTeam invite guest by email to some channels in a team.
func (c *Client4) InviteGuestsToTeam(teamID string, userEmails []string, channels []string, message string) (bool, *Response) {
	guestsInvite := GuestsInvite{
		Emails:   userEmails,
		Channels: channels,
		Message:  message,
	}
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/invite-guests/email", guestsInvite.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InviteUsersToTeam invite users by email to the team.
func (c *Client4) InviteUsersToTeamGracefully(teamID string, userEmails []string) ([]*EmailInviteWithError, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/invite/email?graceful="+c.boolString(true), ArrayToJSON(userEmails))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmailInviteWithErrorFromJSON(r.Body), BuildResponse(r)
}

// InviteGuestsToTeam invite guest by email to some channels in a team.
func (c *Client4) InviteGuestsToTeamGracefully(teamID string, userEmails []string, channels []string, message string) ([]*EmailInviteWithError, *Response) {
	guestsInvite := GuestsInvite{
		Emails:   userEmails,
		Channels: channels,
		Message:  message,
	}
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/invite-guests/email?graceful="+c.boolString(true), guestsInvite.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmailInviteWithErrorFromJSON(r.Body), BuildResponse(r)
}

// InvalidateEmailInvites will invalidate active email invitations that have not been accepted by the user.
func (c *Client4) InvalidateEmailInvites() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetTeamsRoute() + "/invites/email")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamInviteInfo returns a team object from an invite id containing sanitized information.
func (c *Client4) GetTeamInviteInfo(inviteID string) (*Team, *Response) {
	r, err := c.DoAPIGet(c.GetTeamsRoute()+"/invite/"+inviteID, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJSON(r.Body), BuildResponse(r)
}

// SetTeamIcon sets team icon of the team.
func (c *Client4) SetTeamIcon(teamID string, data []byte) (bool, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "teamIcon.png")
	if err != nil {
		return false, &Response{Error: NewAppError("SetTeamIcon", "model.client.set_team_icon.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return false, &Response{Error: NewAppError("SetTeamIcon", "model.client.set_team_icon.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err = writer.Close(); err != nil {
		return false, &Response{Error: NewAppError("SetTeamIcon", "model.client.set_team_icon.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetTeamRoute(teamID)+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetTeamIcon", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		// set to http.StatusForbidden(403)
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetTeamRoute(teamID)+"/image", "model.client.connecting.app_error", nil, err.Error(), 403)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// GetTeamIcon gets the team icon of the team.
func (c *Client4) GetTeamIcon(teamID, etag string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetTeamRoute(teamID)+"/image", etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetTeamIcon", "model.client.get_team_icon.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// RemoveTeamIcon updates LastTeamIconUpdate to 0 which indicates team icon is removed.
func (c *Client4) RemoveTeamIcon(teamID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetTeamRoute(teamID) + "/image")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Channel Section

// GetAllChannels get all the channels. Must be a system administrator.
func (c *Client4) GetAllChannels(page int, perPage int, etag string) (*ChannelListWithTeamData, *Response) {
	return c.getAllChannels(page, perPage, etag, ChannelSearchOpts{})
}

// GetAllChannelsIncludeDeleted get all the channels. Must be a system administrator.
func (c *Client4) GetAllChannelsIncludeDeleted(page int, perPage int, etag string) (*ChannelListWithTeamData, *Response) {
	return c.getAllChannels(page, perPage, etag, ChannelSearchOpts{IncludeDeleted: true})
}

// GetAllChannelsExcludePolicyConstrained gets all channels which are not part of a data retention policy.
// Must be a system administrator.
func (c *Client4) GetAllChannelsExcludePolicyConstrained(page, perPage int, etag string) (*ChannelListWithTeamData, *Response) {
	return c.getAllChannels(page, perPage, etag, ChannelSearchOpts{ExcludePolicyConstrained: true})
}

func (c *Client4) getAllChannels(page int, perPage int, etag string, opts ChannelSearchOpts) (*ChannelListWithTeamData, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_deleted=%v&exclude_policy_constrained=%v",
		page, perPage, opts.IncludeDeleted, opts.ExcludePolicyConstrained)
	r, err := c.DoAPIGet(c.GetChannelsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListWithTeamDataFromJSON(r.Body), BuildResponse(r)
}

// GetAllChannelsWithCount get all the channels including the total count. Must be a system administrator.
func (c *Client4) GetAllChannelsWithCount(page int, perPage int, etag string) (*ChannelListWithTeamData, int64, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_total_count="+c.boolString(true), page, perPage)
	r, err := c.DoAPIGet(c.GetChannelsRoute()+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	cwc := ChannelsWithCountFromJSON(r.Body)
	return cwc.Channels, cwc.TotalCount, BuildResponse(r)
}

// CreateChannel creates a channel based on the provided channel struct.
func (c *Client4) CreateChannel(channel *Channel) (*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsRoute(), channel.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// UpdateChannel updates a channel based on the provided channel struct.
func (c *Client4) UpdateChannel(channel *Channel) (*Channel, *Response) {
	r, err := c.DoAPIPut(c.GetChannelRoute(channel.ID), channel.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// PatchChannel partially updates a channel. Any missing fields are not updated.
func (c *Client4) PatchChannel(channelID string, patch *ChannelPatch) (*Channel, *Response) {
	r, err := c.DoAPIPut(c.GetChannelRoute(channelID)+"/patch", patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// ConvertChannelToPrivate converts public to private channel.
func (c *Client4) ConvertChannelToPrivate(channelID string) (*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelRoute(channelID)+"/convert", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// UpdateChannelPrivacy updates channel privacy
func (c *Client4) UpdateChannelPrivacy(channelID string, privacy string) (*Channel, *Response) {
	requestBody := map[string]string{"privacy": privacy}
	r, err := c.DoAPIPut(c.GetChannelRoute(channelID)+"/privacy", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// RestoreChannel restores a previously deleted channel. Any missing fields are not updated.
func (c *Client4) RestoreChannel(channelID string) (*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelRoute(channelID)+"/restore", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// CreateDirectChannel creates a direct message channel based on the two user
// ids provided.
func (c *Client4) CreateDirectChannel(userID1, userID2 string) (*Channel, *Response) {
	requestBody := []string{userID1, userID2}
	r, err := c.DoAPIPost(c.GetChannelsRoute()+"/direct", ArrayToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// CreateGroupChannel creates a group message channel based on userIds provided.
func (c *Client4) CreateGroupChannel(userIDs []string) (*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsRoute()+"/group", ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannel returns a channel based on the provided channel id string.
func (c *Client4) GetChannel(channelID, etag string) (*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelStats returns statistics for a channel.
func (c *Client4) GetChannelStats(channelID string, etag string) (*ChannelStats, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/stats", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelStatsFromJSON(r.Body), BuildResponse(r)
}

// GetChannelMembersTimezones gets a list of timezones for a channel.
func (c *Client4) GetChannelMembersTimezones(channelID string) ([]string, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/timezones", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

// GetPinnedPosts gets a list of pinned posts.
func (c *Client4) GetPinnedPosts(channelID string, etag string) (*PostList, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/pinned", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPrivateChannelsForTeam returns a list of private channels based on the provided team id string.
func (c *Client4) GetPrivateChannelsForTeam(teamID string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("/private?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetChannelsForTeamRoute(teamID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// GetPublicChannelsForTeam returns a list of public channels based on the provided team id string.
func (c *Client4) GetPublicChannelsForTeam(teamID string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetChannelsForTeamRoute(teamID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// GetDeletedChannelsForTeam returns a list of public channels based on the provided team id string.
func (c *Client4) GetDeletedChannelsForTeam(teamID string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("/deleted?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetChannelsForTeamRoute(teamID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// GetPublicChannelsByIdsForTeam returns a list of public channels based on provided team id string.
func (c *Client4) GetPublicChannelsByIDsForTeam(teamID string, channelIDs []string) ([]*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsForTeamRoute(teamID)+"/ids", ArrayToJSON(channelIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// GetChannelsForTeamForUser returns a list channels of on a team for a user.
func (c *Client4) GetChannelsForTeamForUser(teamID, userID string, includeDeleted bool, etag string) ([]*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelsForTeamForUserRoute(teamID, userID, includeDeleted), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// GetChannelsForTeamAndUserWithLastDeleteAt returns a list channels of a team for a user, additionally filtered with lastDeleteAt. This does not have any effect if includeDeleted is set to false.
func (c *Client4) GetChannelsForTeamAndUserWithLastDeleteAt(teamID, userID string, includeDeleted bool, lastDeleteAt int, etag string) ([]*Channel, *Response) {
	route := fmt.Sprintf(c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/channels")
	route += fmt.Sprintf("?include_deleted=%v&last_delete_at=%d", includeDeleted, lastDeleteAt)
	r, err := c.DoAPIGet(route, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// SearchChannels returns the channels on a team matching the provided search term.
func (c *Client4) SearchChannels(teamID string, search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsForTeamRoute(teamID)+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// SearchArchivedChannels returns the archived channels on a team matching the provided search term.
func (c *Client4) SearchArchivedChannels(teamID string, search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsForTeamRoute(teamID)+"/search_archived", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// SearchAllChannels search in all the channels. Must be a system administrator.
func (c *Client4) SearchAllChannels(search *ChannelSearch) (*ChannelListWithTeamData, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListWithTeamDataFromJSON(r.Body), BuildResponse(r)
}

// SearchAllChannelsPaged searches all the channels and returns the results paged with the total count.
func (c *Client4) SearchAllChannelsPaged(search *ChannelSearch) (*ChannelsWithCount, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelsWithCountFromJSON(r.Body), BuildResponse(r)
}

// SearchGroupChannels returns the group channels of the user whose members' usernames match the search term.
func (c *Client4) SearchGroupChannels(search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoAPIPost(c.GetChannelsRoute()+"/group/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJSON(r.Body), BuildResponse(r)
}

// DeleteChannel deletes channel based on the provided channel id string.
func (c *Client4) DeleteChannel(channelID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetChannelRoute(channelID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteChannel deletes a channel based on the provided channel id string.
func (c *Client4) PermanentDeleteChannel(channelID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetChannelRoute(channelID) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// MoveChannel moves the channel to the destination team.
func (c *Client4) MoveChannel(channelID, teamID string, force bool) (*Channel, *Response) {
	requestBody := map[string]interface{}{
		"team_id": teamID,
		"force":   force,
	}
	r, err := c.DoAPIPost(c.GetChannelRoute(channelID)+"/move", StringInterfaceToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelByName returns a channel based on the provided channel name and team id strings.
func (c *Client4) GetChannelByName(channelName, teamID string, etag string) (*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelByNameRoute(channelName, teamID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelByNameIncludeDeleted returns a channel based on the provided channel name and team id strings. Other then GetChannelByName it will also return deleted channels.
func (c *Client4) GetChannelByNameIncludeDeleted(channelName, teamID string, etag string) (*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelByNameRoute(channelName, teamID)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelByNameForTeamName returns a channel based on the provided channel name and team name strings.
func (c *Client4) GetChannelByNameForTeamName(channelName, teamName string, etag string) (*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelByNameForTeamNameRoute(channelName, teamName), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelByNameForTeamNameIncludeDeleted returns a channel based on the provided channel name and team name strings. Other then GetChannelByNameForTeamName it will also return deleted channels.
func (c *Client4) GetChannelByNameForTeamNameIncludeDeleted(channelName, teamName string, etag string) (*Channel, *Response) {
	r, err := c.DoAPIGet(c.GetChannelByNameForTeamNameRoute(channelName, teamName)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJSON(r.Body), BuildResponse(r)
}

// GetChannelMembers gets a page of channel members.
func (c *Client4) GetChannelMembers(channelID string, page, perPage int, etag string) (*ChannelMembers, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetChannelMembersRoute(channelID)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJSON(r.Body), BuildResponse(r)
}

// GetChannelMembersByIds gets the channel members in a channel for a list of user ids.
func (c *Client4) GetChannelMembersByIDs(channelID string, userIDs []string) (*ChannelMembers, *Response) {
	r, err := c.DoAPIPost(c.GetChannelMembersRoute(channelID)+"/ids", ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJSON(r.Body), BuildResponse(r)
}

// GetChannelMember gets a channel member.
func (c *Client4) GetChannelMember(channelID, userID, etag string) (*ChannelMember, *Response) {
	r, err := c.DoAPIGet(c.GetChannelMemberRoute(channelID, userID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJSON(r.Body), BuildResponse(r)
}

// GetChannelMembersForUser gets all the channel members for a user on a team.
func (c *Client4) GetChannelMembersForUser(userID, teamID, etag string) (*ChannelMembers, *Response) {
	r, err := c.DoAPIGet(fmt.Sprintf(c.GetUserRoute(userID)+"/teams/%v/channels/members", teamID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJSON(r.Body), BuildResponse(r)
}

// ViewChannel performs a view action for a user. Synonymous with switching channels or marking channels as read by a user.
func (c *Client4) ViewChannel(userID string, view *ChannelView) (*ChannelViewResponse, *Response) {
	url := fmt.Sprintf(c.GetChannelsRoute()+"/members/%v/view", userID)
	r, err := c.DoAPIPost(url, view.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelViewResponseFromJSON(r.Body), BuildResponse(r)
}

// GetChannelUnread will return a ChannelUnread object that contains the number of
// unread messages and mentions for a user.
func (c *Client4) GetChannelUnread(channelID, userID string) (*ChannelUnread, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+c.GetChannelRoute(channelID)+"/unread", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelUnreadFromJSON(r.Body), BuildResponse(r)
}

// UpdateChannelRoles will update the roles on a channel for a user.
func (c *Client4) UpdateChannelRoles(channelID, userID, roles string) (bool, *Response) {
	requestBody := map[string]string{"roles": roles}
	r, err := c.DoAPIPut(c.GetChannelMemberRoute(channelID, userID)+"/roles", MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateChannelMemberSchemeRoles will update the scheme-derived roles on a channel for a user.
func (c *Client4) UpdateChannelMemberSchemeRoles(channelID string, userID string, schemeRoles *SchemeRoles) (bool, *Response) {
	r, err := c.DoAPIPut(c.GetChannelMemberRoute(channelID, userID)+"/schemeRoles", schemeRoles.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateChannelNotifyProps will update the notification properties on a channel for a user.
func (c *Client4) UpdateChannelNotifyProps(channelID, userID string, props map[string]string) (bool, *Response) {
	r, err := c.DoAPIPut(c.GetChannelMemberRoute(channelID, userID)+"/notify_props", MapToJSON(props))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AddChannelMember adds user to channel and return a channel member.
func (c *Client4) AddChannelMember(channelID, userID string) (*ChannelMember, *Response) {
	requestBody := map[string]string{"user_id": userID}
	r, err := c.DoAPIPost(c.GetChannelMembersRoute(channelID)+"", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJSON(r.Body), BuildResponse(r)
}

// AddChannelMemberWithRootId adds user to channel and return a channel member. Post add to channel message has the postRootId.
func (c *Client4) AddChannelMemberWithRootID(channelID, userID, postRootID string) (*ChannelMember, *Response) {
	requestBody := map[string]string{"user_id": userID, "post_root_id": postRootID}
	r, err := c.DoAPIPost(c.GetChannelMembersRoute(channelID)+"", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJSON(r.Body), BuildResponse(r)
}

// RemoveUserFromChannel will delete the channel member object for a user, effectively removing the user from a channel.
func (c *Client4) RemoveUserFromChannel(channelID, userID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetChannelMemberRoute(channelID, userID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AutocompleteChannelsForTeam will return an ordered list of channels autocomplete suggestions.
func (c *Client4) AutocompleteChannelsForTeam(teamID, name string) (*ChannelList, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoAPIGet(c.GetChannelsForTeamRoute(teamID)+"/autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListFromJSON(r.Body), BuildResponse(r)
}

// AutocompleteChannelsForTeamForSearch will return an ordered list of your channels autocomplete suggestions.
func (c *Client4) AutocompleteChannelsForTeamForSearch(teamID, name string) (*ChannelList, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoAPIGet(c.GetChannelsForTeamRoute(teamID)+"/search_autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListFromJSON(r.Body), BuildResponse(r)
}

// Post Section

// CreatePost creates a post based on the provided post struct.
func (c *Client4) CreatePost(post *Post) (*Post, *Response) {
	r, err := c.DoAPIPost(c.GetPostsRoute(), post.ToUnsanitizedJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJSON(r.Body), BuildResponse(r)
}

// CreatePostEphemeral creates a ephemeral post based on the provided post struct which is send to the given user id.
func (c *Client4) CreatePostEphemeral(post *PostEphemeral) (*Post, *Response) {
	r, err := c.DoAPIPost(c.GetPostsEphemeralRoute(), post.ToUnsanitizedJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJSON(r.Body), BuildResponse(r)
}

// UpdatePost updates a post based on the provided post struct.
func (c *Client4) UpdatePost(postID string, post *Post) (*Post, *Response) {
	r, err := c.DoAPIPut(c.GetPostRoute(postID), post.ToUnsanitizedJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJSON(r.Body), BuildResponse(r)
}

// PatchPost partially updates a post. Any missing fields are not updated.
func (c *Client4) PatchPost(postID string, patch *PostPatch) (*Post, *Response) {
	r, err := c.DoAPIPut(c.GetPostRoute(postID)+"/patch", patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJSON(r.Body), BuildResponse(r)
}

// SetPostUnread marks channel where post belongs as unread on the time of the provided post.
func (c *Client4) SetPostUnread(userID string, postID string, collapsedThreadsSupported bool) *Response {
	b, _ := json.Marshal(map[string]bool{"collapsed_threads_supported": collapsedThreadsSupported})
	r, err := c.DoAPIPost(c.GetUserRoute(userID)+c.GetPostRoute(postID)+"/set_unread", string(b))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// PinPost pin a post based on provided post id string.
func (c *Client4) PinPost(postID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPostRoute(postID)+"/pin", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UnpinPost unpin a post based on provided post id string.
func (c *Client4) UnpinPost(postID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPostRoute(postID)+"/unpin", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetPost gets a single post.
func (c *Client4) GetPost(postID string, etag string) (*Post, *Response) {
	r, err := c.DoAPIGet(c.GetPostRoute(postID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJSON(r.Body), BuildResponse(r)
}

// DeletePost deletes a post from the provided post id string.
func (c *Client4) DeletePost(postID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetPostRoute(postID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetPostThread gets a post with all the other posts in the same thread.
func (c *Client4) GetPostThread(postID string, etag string, collapsedThreads bool) (*PostList, *Response) {
	url := c.GetPostRoute(postID) + "/thread"
	if collapsedThreads {
		url += "?collapsedThreads=true"
	}
	r, err := c.DoAPIGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPostsForChannel gets a page of posts with an array for ordering for a channel.
func (c *Client4) GetPostsForChannel(channelID string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUser returns flagged posts of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUser(userID string, page int, perPage int) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUserInTeam returns flagged posts in team of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUserInTeam(userID string, teamID string, page int, perPage int) (*PostList, *Response) {
	if !IsValidID(teamID) {
		return nil, &Response{StatusCode: http.StatusBadRequest, Error: NewAppError("GetFlaggedPostsForUserInTeam", "model.client.get_flagged_posts_in_team.missing_parameter.app_error", nil, "", http.StatusBadRequest)}
	}

	query := fmt.Sprintf("?team_id=%v&page=%v&per_page=%v", teamID, page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUserInChannel returns flagged posts in channel of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUserInChannel(userID string, channelID string, page int, perPage int) (*PostList, *Response) {
	if !IsValidID(channelID) {
		return nil, &Response{StatusCode: http.StatusBadRequest, Error: NewAppError("GetFlaggedPostsForUserInChannel", "model.client.get_flagged_posts_in_channel.missing_parameter.app_error", nil, "", http.StatusBadRequest)}
	}

	query := fmt.Sprintf("?channel_id=%v&page=%v&per_page=%v", channelID, page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPostsSince gets posts created after a specified time as Unix time in milliseconds.
func (c *Client4) GetPostsSince(channelID string, time int64, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?since=%v", time)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/posts"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPostsAfter gets a page of posts that were posted after the post provided.
func (c *Client4) GetPostsAfter(channelID, postID string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&after=%v", page, perPage, postID)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPostsBefore gets a page of posts that were posted before the post provided.
func (c *Client4) GetPostsBefore(channelID, postID string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&before=%v", page, perPage, postID)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// GetPostsAroundLastUnread gets a list of posts around last unread post by a user in a channel.
func (c *Client4) GetPostsAroundLastUnread(userID, channelID string, limitBefore, limitAfter int, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?limit_before=%v&limit_after=%v", limitBefore, limitAfter)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+c.GetChannelRoute(channelID)+"/posts/unread"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// SearchFiles returns any posts with matching terms string.
func (c *Client4) SearchFiles(teamID string, terms string, isOrSearch bool) (*FileInfoList, *Response) {
	params := SearchParameter{
		Terms:      &terms,
		IsOrSearch: &isOrSearch,
	}
	return c.SearchFilesWithParams(teamID, &params)
}

// SearchFilesWithParams returns any posts with matching terms string.
func (c *Client4) SearchFilesWithParams(teamID string, params *SearchParameter) (*FileInfoList, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/files/search", params.SearchParameterToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoListFromJSON(r.Body), BuildResponse(r)
}

// SearchPosts returns any posts with matching terms string.
func (c *Client4) SearchPosts(teamID string, terms string, isOrSearch bool) (*PostList, *Response) {
	params := SearchParameter{
		Terms:      &terms,
		IsOrSearch: &isOrSearch,
	}
	return c.SearchPostsWithParams(teamID, &params)
}

// SearchPostsWithParams returns any posts with matching terms string.
func (c *Client4) SearchPostsWithParams(teamID string, params *SearchParameter) (*PostList, *Response) {
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/posts/search", params.SearchParameterToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJSON(r.Body), BuildResponse(r)
}

// SearchPostsWithMatches returns any posts with matching terms string, including.
func (c *Client4) SearchPostsWithMatches(teamID string, terms string, isOrSearch bool) (*PostSearchResults, *Response) {
	requestBody := map[string]interface{}{"terms": terms, "is_or_search": isOrSearch}
	r, err := c.DoAPIPost(c.GetTeamRoute(teamID)+"/posts/search", StringInterfaceToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostSearchResultsFromJSON(r.Body), BuildResponse(r)
}

// DoPostAction performs a post action.
func (c *Client4) DoPostAction(postID, actionID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPostRoute(postID)+"/actions/"+actionID, "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DoPostActionWithCookie performs a post action with extra arguments
func (c *Client4) DoPostActionWithCookie(postID, actionID, selected, cookieStr string) (bool, *Response) {
	var body []byte
	if selected != "" || cookieStr != "" {
		body, _ = json.Marshal(DoPostActionRequest{
			SelectedOption: selected,
			Cookie:         cookieStr,
		})
	}
	r, err := c.DoAPIPost(c.GetPostRoute(postID)+"/actions/"+actionID, string(body))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// OpenInteractiveDialog sends a WebSocket event to a user's clients to
// open interactive dialogs, based on the provided trigger ID and other
// provided data. Used with interactive message buttons, menus and
// slash commands.
func (c *Client4) OpenInteractiveDialog(request OpenDialogRequest) (bool, *Response) {
	b, _ := json.Marshal(request)
	r, err := c.DoAPIPost("/actions/dialogs/open", string(b))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SubmitInteractiveDialog will submit the provided dialog data to the integration
// configured by the URL. Used with the interactive dialogs integration feature.
func (c *Client4) SubmitInteractiveDialog(request SubmitDialogRequest) (*SubmitDialogResponse, *Response) {
	b, _ := json.Marshal(request)
	r, err := c.DoAPIPost("/actions/dialogs/submit", string(b))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	var resp SubmitDialogResponse
	json.NewDecoder(r.Body).Decode(&resp)
	return &resp, BuildResponse(r)
}

// UploadFile will upload a file to a channel using a multipart request, to be later attached to a post.
// This method is functionally equivalent to Client4.UploadFileAsRequestBody.
func (c *Client4) UploadFile(data []byte, channelID string, filename string) (*FileUploadResponse, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormField("channel_id")
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.channel_id.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, err = io.Copy(part, strings.NewReader(channelID))
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.channel_id.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	part, err = writer.CreateFormFile("files", filename)
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	_, err = io.Copy(part, bytes.NewBuffer(data))
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	err = writer.Close()
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	return c.DoUploadFile(c.GetFilesRoute(), body.Bytes(), writer.FormDataContentType())
}

// UploadFileAsRequestBody will upload a file to a channel as the body of a request, to be later attached
// to a post. This method is functionally equivalent to Client4.UploadFile.
func (c *Client4) UploadFileAsRequestBody(data []byte, channelID string, filename string) (*FileUploadResponse, *Response) {
	return c.DoUploadFile(c.GetFilesRoute()+fmt.Sprintf("?channel_id=%v&filename=%v", url.QueryEscape(channelID), url.QueryEscape(filename)), data, http.DetectContentType(data))
}

// GetFile gets the bytes for a file by id.
func (c *Client4) GetFile(fileID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetFile", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// DownloadFile gets the bytes for a file by id, optionally adding headers to force the browser to download it.
func (c *Client4) DownloadFile(fileID string, download bool) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID)+fmt.Sprintf("?download=%v", download), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("DownloadFile", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// GetFileThumbnail gets the bytes for a file by id.
func (c *Client4) GetFileThumbnail(fileID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID)+"/thumbnail", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetFileThumbnail", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// DownloadFileThumbnail gets the bytes for a file by id, optionally adding headers to force the browser to download it.
func (c *Client4) DownloadFileThumbnail(fileID string, download bool) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID)+fmt.Sprintf("/thumbnail?download=%v", download), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("DownloadFileThumbnail", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// GetFileLink gets the public link of a file by id.
func (c *Client4) GetFileLink(fileID string) (string, *Response) {
	r, err := c.DoAPIGet(c.GetFileRoute(fileID)+"/link", "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["link"], BuildResponse(r)
}

// GetFilePreview gets the bytes for a file by id.
func (c *Client4) GetFilePreview(fileID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID)+"/preview", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetFilePreview", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// DownloadFilePreview gets the bytes for a file by id.
func (c *Client4) DownloadFilePreview(fileID string, download bool) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetFileRoute(fileID)+fmt.Sprintf("/preview?download=%v", download), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("DownloadFilePreview", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// GetFileInfo gets all the file info objects.
func (c *Client4) GetFileInfo(fileID string) (*FileInfo, *Response) {
	r, err := c.DoAPIGet(c.GetFileRoute(fileID)+"/info", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoFromJSON(r.Body), BuildResponse(r)
}

// GetFileInfosForPost gets all the file info objects attached to a post.
func (c *Client4) GetFileInfosForPost(postID string, etag string) ([]*FileInfo, *Response) {
	r, err := c.DoAPIGet(c.GetPostRoute(postID)+"/files/info", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfosFromJSON(r.Body), BuildResponse(r)
}

// General/System Section

// GenerateSupportPacket downloads the generated support packet
func (c *Client4) GenerateSupportPacket() ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetSystemRoute()+"/support_packet", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetFile", "model.client.read_job_result_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// GetPing will return ok if the running goRoutines are below the threshold and unhealthy for above.
func (c *Client4) GetPing() (string, *Response) {
	r, err := c.DoAPIGet(c.GetSystemRoute()+"/ping", "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return StatusUnhealthy, BuildErrorResponse(r, err)
	}
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["status"], BuildResponse(r)
}

// GetPingWithServerStatus will return ok if several basic server health checks
// all pass successfully.
func (c *Client4) GetPingWithServerStatus() (string, *Response) {
	r, err := c.DoAPIGet(c.GetSystemRoute()+"/ping?get_server_status="+c.boolString(true), "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return StatusUnhealthy, BuildErrorResponse(r, err)
	}
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["status"], BuildResponse(r)
}

// GetPingWithFullServerStatus will return the full status if several basic server
// health checks all pass successfully.
func (c *Client4) GetPingWithFullServerStatus() (map[string]string, *Response) {
	r, err := c.DoAPIGet(c.GetSystemRoute()+"/ping?get_server_status="+c.boolString(true), "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return map[string]string{"status": StatusUnhealthy}, BuildErrorResponse(r, err)
	}
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body), BuildResponse(r)
}

// TestEmail will attempt to connect to the configured SMTP server.
func (c *Client4) TestEmail(config *Config) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetTestEmailRoute(), config.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// TestSiteURL will test the validity of a site URL.
func (c *Client4) TestSiteURL(siteURL string) (bool, *Response) {
	requestBody := make(map[string]string)
	requestBody["site_url"] = siteURL
	r, err := c.DoAPIPost(c.GetTestSiteURLRoute(), MapToJSON(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// TestS3Connection will attempt to connect to the AWS S3.
func (c *Client4) TestS3Connection(config *Config) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetTestS3Route(), config.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetConfig will retrieve the server config with some sanitized items.
func (c *Client4) GetConfig() (*Config, *Response) {
	r, err := c.DoAPIGet(c.GetConfigRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJSON(r.Body), BuildResponse(r)
}

// ReloadConfig will reload the server configuration.
func (c *Client4) ReloadConfig() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetConfigRoute()+"/reload", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetOldClientConfig will retrieve the parts of the server configuration needed by the
// client, formatted in the old format.
func (c *Client4) GetOldClientConfig(etag string) (map[string]string, *Response) {
	r, err := c.DoAPIGet(c.GetConfigRoute()+"/client?format=old", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body), BuildResponse(r)
}

// GetEnvironmentConfig will retrieve a map mirroring the server configuration where fields
// are set to true if the corresponding config setting is set through an environment variable.
// Settings that haven't been set through environment variables will be missing from the map.
func (c *Client4) GetEnvironmentConfig() (map[string]interface{}, *Response) {
	r, err := c.DoAPIGet(c.GetConfigRoute()+"/environment", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StringInterfaceFromJSON(r.Body), BuildResponse(r)
}

// GetOldClientLicense will retrieve the parts of the server license needed by the
// client, formatted in the old format.
func (c *Client4) GetOldClientLicense(etag string) (map[string]string, *Response) {
	r, err := c.DoAPIGet(c.GetLicenseRoute()+"/client?format=old", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body), BuildResponse(r)
}

// DatabaseRecycle will recycle the connections. Discard current connection and get new one.
func (c *Client4) DatabaseRecycle() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetDatabaseRoute()+"/recycle", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InvalidateCaches will purge the cache and can affect the performance while is cleaning.
func (c *Client4) InvalidateCaches() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetCacheRoute()+"/invalidate", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateConfig will update the server configuration.
func (c *Client4) UpdateConfig(config *Config) (*Config, *Response) {
	r, err := c.DoAPIPut(c.GetConfigRoute(), config.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJSON(r.Body), BuildResponse(r)
}

// MigrateConfig will migrate existing config to the new one.
func (c *Client4) MigrateConfig(from, to string) (bool, *Response) {
	m := make(map[string]string, 2)
	m["from"] = from
	m["to"] = to
	r, err := c.DoAPIPost(c.GetConfigRoute()+"/migrate", MapToJSON(m))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return true, BuildResponse(r)
}

// UploadLicenseFile will add a license file to the system.
func (c *Client4) UploadLicenseFile(data []byte) (bool, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("license", "test-license.mattermost-license")
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLicenseFile", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return false, &Response{Error: NewAppError("UploadLicenseFile", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err = writer.Close(); err != nil {
		return false, &Response{Error: NewAppError("UploadLicenseFile", "model.client.set_profile_user.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetLicenseRoute(), bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLicenseFile", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetLicenseRoute(), "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// RemoveLicenseFile will remove the server license it exists. Note that this will
// disable all enterprise features.
func (c *Client4) RemoveLicenseFile() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetLicenseRoute())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetAnalyticsOld will retrieve analytics using the old format. New format is not
// available but the "/analytics" endpoint is reserved for it. The "name" argument is optional
// and defaults to "standard". The "teamId" argument is optional and will limit results
// to a specific team.
func (c *Client4) GetAnalyticsOld(name, teamID string) (AnalyticsRows, *Response) {
	query := fmt.Sprintf("?name=%v&team_id=%v", name, teamID)
	r, err := c.DoAPIGet(c.GetAnalyticsRoute()+"/old"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AnalyticsRowsFromJSON(r.Body), BuildResponse(r)
}

// Webhooks Section

// CreateIncomingWebhook creates an incoming webhook for a channel.
func (c *Client4) CreateIncomingWebhook(hook *IncomingWebhook) (*IncomingWebhook, *Response) {
	r, err := c.DoAPIPost(c.GetIncomingWebhooksRoute(), hook.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJSON(r.Body), BuildResponse(r)
}

// UpdateIncomingWebhook updates an incoming webhook for a channel.
func (c *Client4) UpdateIncomingWebhook(hook *IncomingWebhook) (*IncomingWebhook, *Response) {
	r, err := c.DoAPIPut(c.GetIncomingWebhookRoute(hook.ID), hook.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJSON(r.Body), BuildResponse(r)
}

// GetIncomingWebhooks returns a page of incoming webhooks on the system. Page counting starts at 0.
func (c *Client4) GetIncomingWebhooks(page int, perPage int, etag string) ([]*IncomingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetIncomingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookListFromJSON(r.Body), BuildResponse(r)
}

// GetIncomingWebhooksForTeam returns a page of incoming webhooks for a team. Page counting starts at 0.
func (c *Client4) GetIncomingWebhooksForTeam(teamID string, page int, perPage int, etag string) ([]*IncomingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&team_id=%v", page, perPage, teamID)
	r, err := c.DoAPIGet(c.GetIncomingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookListFromJSON(r.Body), BuildResponse(r)
}

// GetIncomingWebhook returns an Incoming webhook given the hook ID.
func (c *Client4) GetIncomingWebhook(hookID string, etag string) (*IncomingWebhook, *Response) {
	r, err := c.DoAPIGet(c.GetIncomingWebhookRoute(hookID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJSON(r.Body), BuildResponse(r)
}

// DeleteIncomingWebhook deletes and Incoming Webhook given the hook ID.
func (c *Client4) DeleteIncomingWebhook(hookID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetIncomingWebhookRoute(hookID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// CreateOutgoingWebhook creates an outgoing webhook for a team or channel.
func (c *Client4) CreateOutgoingWebhook(hook *OutgoingWebhook) (*OutgoingWebhook, *Response) {
	r, err := c.DoAPIPost(c.GetOutgoingWebhooksRoute(), hook.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJSON(r.Body), BuildResponse(r)
}

// UpdateOutgoingWebhook creates an outgoing webhook for a team or channel.
func (c *Client4) UpdateOutgoingWebhook(hook *OutgoingWebhook) (*OutgoingWebhook, *Response) {
	r, err := c.DoAPIPut(c.GetOutgoingWebhookRoute(hook.ID), hook.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJSON(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooks returns a page of outgoing webhooks on the system. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooks(page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJSON(r.Body), BuildResponse(r)
}

// GetOutgoingWebhook outgoing webhooks on the system requested by Hook Id.
func (c *Client4) GetOutgoingWebhook(hookID string) (*OutgoingWebhook, *Response) {
	r, err := c.DoAPIGet(c.GetOutgoingWebhookRoute(hookID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJSON(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooksForChannel returns a page of outgoing webhooks for a channel. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooksForChannel(channelID string, page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&channel_id=%v", page, perPage, channelID)
	r, err := c.DoAPIGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJSON(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooksForTeam returns a page of outgoing webhooks for a team. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooksForTeam(teamID string, page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&team_id=%v", page, perPage, teamID)
	r, err := c.DoAPIGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJSON(r.Body), BuildResponse(r)
}

// RegenOutgoingHookToken regenerate the outgoing webhook token.
func (c *Client4) RegenOutgoingHookToken(hookID string) (*OutgoingWebhook, *Response) {
	r, err := c.DoAPIPost(c.GetOutgoingWebhookRoute(hookID)+"/regen_token", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJSON(r.Body), BuildResponse(r)
}

// DeleteOutgoingWebhook delete the outgoing webhook on the system requested by Hook Id.
func (c *Client4) DeleteOutgoingWebhook(hookID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetOutgoingWebhookRoute(hookID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Preferences Section

// GetPreferences returns the user's preferences.
func (c *Client4) GetPreferences(userID string) (Preferences, *Response) {
	r, err := c.DoAPIGet(c.GetPreferencesRoute(userID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	preferences, _ := PreferencesFromJSON(r.Body)
	return preferences, BuildResponse(r)
}

// UpdatePreferences saves the user's preferences.
func (c *Client4) UpdatePreferences(userID string, preferences *Preferences) (bool, *Response) {
	r, err := c.DoAPIPut(c.GetPreferencesRoute(userID), preferences.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return true, BuildResponse(r)
}

// DeletePreferences deletes the user's preferences.
func (c *Client4) DeletePreferences(userID string, preferences *Preferences) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPreferencesRoute(userID)+"/delete", preferences.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return true, BuildResponse(r)
}

// GetPreferencesByCategory returns the user's preferences from the provided category string.
func (c *Client4) GetPreferencesByCategory(userID string, category string) (Preferences, *Response) {
	url := fmt.Sprintf(c.GetPreferencesRoute(userID)+"/%s", category)
	r, err := c.DoAPIGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	preferences, _ := PreferencesFromJSON(r.Body)
	return preferences, BuildResponse(r)
}

// GetPreferenceByCategoryAndName returns the user's preferences from the provided category and preference name string.
func (c *Client4) GetPreferenceByCategoryAndName(userID string, category string, preferenceName string) (*Preference, *Response) {
	url := fmt.Sprintf(c.GetPreferencesRoute(userID)+"/%s/name/%v", category, preferenceName)
	r, err := c.DoAPIGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PreferenceFromJSON(r.Body), BuildResponse(r)
}

// SAML Section

// GetSamlMetadata returns metadata for the SAML configuration.
func (c *Client4) GetSamlMetadata() (string, *Response) {
	r, err := c.DoAPIGet(c.GetSamlRoute()+"/metadata", "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	return buf.String(), BuildResponse(r)
}

func fileToMultipart(data []byte, filename string) ([]byte, *multipart.Writer, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("certificate", filename)
	if err != nil {
		return nil, nil, err
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return nil, nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	return body.Bytes(), writer, nil
}

// UploadSamlIdpCertificate will upload an IDP certificate for SAML and set the config to use it.
// The filename parameter is deprecated and ignored: the server will pick a hard-coded filename when writing to disk.
func (c *Client4) UploadSamlIDpCertificate(data []byte, filename string) (bool, *Response) {
	body, writer, err := fileToMultipart(data, filename)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadSamlIdpCertificate", "model.client.upload_saml_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetSamlRoute()+"/certificate/idp", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// UploadSamlPublicCertificate will upload a public certificate for SAML and set the config to use it.
// The filename parameter is deprecated and ignored: the server will pick a hard-coded filename when writing to disk.
func (c *Client4) UploadSamlPublicCertificate(data []byte, filename string) (bool, *Response) {
	body, writer, err := fileToMultipart(data, filename)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadSamlPublicCertificate", "model.client.upload_saml_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetSamlRoute()+"/certificate/public", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// UploadSamlPrivateCertificate will upload a private key for SAML and set the config to use it.
// The filename parameter is deprecated and ignored: the server will pick a hard-coded filename when writing to disk.
func (c *Client4) UploadSamlPrivateCertificate(data []byte, filename string) (bool, *Response) {
	body, writer, err := fileToMultipart(data, filename)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadSamlPrivateCertificate", "model.client.upload_saml_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetSamlRoute()+"/certificate/private", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// DeleteSamlIdpCertificate deletes the SAML IDP certificate from the server and updates the config to not use it and disable SAML.
func (c *Client4) DeleteSamlIDpCertificate() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetSamlRoute() + "/certificate/idp")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteSamlPublicCertificate deletes the SAML IDP certificate from the server and updates the config to not use it and disable SAML.
func (c *Client4) DeleteSamlPublicCertificate() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetSamlRoute() + "/certificate/public")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteSamlPrivateCertificate deletes the SAML IDP certificate from the server and updates the config to not use it and disable SAML.
func (c *Client4) DeleteSamlPrivateCertificate() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetSamlRoute() + "/certificate/private")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetSamlCertificateStatus returns metadata for the SAML configuration.
func (c *Client4) GetSamlCertificateStatus() (*SamlCertificateStatus, *Response) {
	r, err := c.DoAPIGet(c.GetSamlRoute()+"/certificate/status", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SamlCertificateStatusFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetSamlMetadataFromIDp(samlMetadataURL string) (*SamlMetadataResponse, *Response) {
	requestBody := make(map[string]string)
	requestBody["saml_metadata_url"] = samlMetadataURL
	r, err := c.DoAPIPost(c.GetSamlRoute()+"/metadatafromidp", MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}

	defer closeBody(r)
	return SamlMetadataResponseFromJSON(r.Body), BuildResponse(r)
}

// ResetSamlAuthDataToEmail resets the AuthData field of SAML users to their Email.
func (c *Client4) ResetSamlAuthDataToEmail(includeDeleted bool, dryRun bool, userIDs []string) (int64, *Response) {
	params := map[string]interface{}{
		"include_deleted": includeDeleted,
		"dry_run":         dryRun,
		"user_ids":        userIDs,
	}
	b, _ := json.Marshal(params)
	r, err := c.doAPIPostBytes(c.GetSamlRoute()+"/reset_auth_data", b)
	if err != nil {
		return 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	respBody := map[string]int64{}
	jsonErr := json.NewDecoder(r.Body).Decode(&respBody)
	if jsonErr != nil {
		appErr := NewAppError("Api4.ResetSamlAuthDataToEmail", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return 0, BuildErrorResponse(r, appErr)
	}
	return respBody["num_affected"], BuildResponse(r)
}

// Compliance Section

// CreateComplianceReport creates an incoming webhook for a channel.
func (c *Client4) CreateComplianceReport(report *Compliance) (*Compliance, *Response) {
	r, err := c.DoAPIPost(c.GetComplianceReportsRoute(), report.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ComplianceFromJSON(r.Body), BuildResponse(r)
}

// GetComplianceReports returns list of compliance reports.
func (c *Client4) GetComplianceReports(page, perPage int) (Compliances, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetComplianceReportsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CompliancesFromJSON(r.Body), BuildResponse(r)
}

// GetComplianceReport returns a compliance report.
func (c *Client4) GetComplianceReport(reportID string) (*Compliance, *Response) {
	r, err := c.DoAPIGet(c.GetComplianceReportRoute(reportID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ComplianceFromJSON(r.Body), BuildResponse(r)
}

// DownloadComplianceReport returns a full compliance report as a file.
func (c *Client4) DownloadComplianceReport(reportID string) ([]byte, *Response) {
	rq, err := http.NewRequest("GET", c.APIURL+c.GetComplianceReportDownloadRoute(reportID), nil)
	if err != nil {
		return nil, &Response{Error: NewAppError("DownloadComplianceReport", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, "BEARER "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, &Response{Error: NewAppError("DownloadComplianceReport", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	data, err := ioutil.ReadAll(rp.Body)
	if err != nil {
		return nil, BuildErrorResponse(rp, NewAppError("DownloadComplianceReport", "model.client.read_file.app_error", nil, err.Error(), rp.StatusCode))
	}

	return data, BuildResponse(rp)
}

// Cluster Section

// GetClusterStatus returns the status of all the configured cluster nodes.
func (c *Client4) GetClusterStatus() ([]*ClusterInfo, *Response) {
	r, err := c.DoAPIGet(c.GetClusterRoute()+"/status", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ClusterInfosFromJSON(r.Body), BuildResponse(r)
}

// LDAP Section

// SyncLdap will force a sync with the configured LDAP server.
// If includeRemovedMembers is true, then group members who left or were removed from a
// synced team/channel will be re-joined; otherwise, they will be excluded.
func (c *Client4) SyncLdap(includeRemovedMembers bool) (bool, *Response) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"include_removed_members": includeRemovedMembers,
	})
	r, err := c.doAPIPostBytes(c.GetLdapRoute()+"/sync", reqBody)
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// TestLdap will attempt to connect to the configured LDAP server and return OK if configured
// correctly.
func (c *Client4) TestLdap() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetLdapRoute()+"/test", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetLdapGroups retrieves the immediate child groups of the given parent group.
func (c *Client4) GetLdapGroups() ([]*Group, *Response) {
	path := fmt.Sprintf("%s/groups", c.GetLdapRoute())

	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	responseData := struct {
		Count  int      `json:"count"`
		Groups []*Group `json:"groups"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&responseData); err != nil {
		appErr := NewAppError("Api4.GetLdapGroups", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return nil, BuildErrorResponse(r, appErr)
	}
	for i := range responseData.Groups {
		responseData.Groups[i].DisplayName = *responseData.Groups[i].Name
	}

	return responseData.Groups, BuildResponse(r)
}

// LinkLdapGroup creates or undeletes a Mattermost group and associates it to the given LDAP group DN.
func (c *Client4) LinkLdapGroup(dn string) (*Group, *Response) {
	path := fmt.Sprintf("%s/groups/%s/link", c.GetLdapRoute(), dn)

	r, appErr := c.DoAPIPost(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupFromJSON(r.Body), BuildResponse(r)
}

// UnlinkLdapGroup deletes the Mattermost group associated with the given LDAP group DN.
func (c *Client4) UnlinkLdapGroup(dn string) (*Group, *Response) {
	path := fmt.Sprintf("%s/groups/%s/link", c.GetLdapRoute(), dn)

	r, appErr := c.DoAPIDelete(path)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupFromJSON(r.Body), BuildResponse(r)
}

// MigrateIdLdap migrates the LDAP enabled users to given attribute
func (c *Client4) MigrateIDLdap(toAttribute string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetLdapRoute()+"/migrateid", MapToJSON(map[string]string{
		"toAttribute": toAttribute,
	}))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetGroupsByChannel retrieves the Mattermost Groups associated with a given channel
func (c *Client4) GetGroupsByChannel(channelID string, opts GroupSearchOpts) ([]*GroupWithSchemeAdmin, int, *Response) {
	path := fmt.Sprintf("%s/groups?q=%v&include_member_count=%v&filter_allow_reference=%v", c.GetChannelRoute(channelID), opts.Q, opts.IncludeMemberCount, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, 0, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	responseData := struct {
		Groups []*GroupWithSchemeAdmin `json:"groups"`
		Count  int                     `json:"total_group_count"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&responseData); err != nil {
		appErr := NewAppError("Api4.GetGroupsByChannel", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return nil, 0, BuildErrorResponse(r, appErr)
	}

	return responseData.Groups, responseData.Count, BuildResponse(r)
}

// GetGroupsByTeam retrieves the Mattermost Groups associated with a given team
func (c *Client4) GetGroupsByTeam(teamID string, opts GroupSearchOpts) ([]*GroupWithSchemeAdmin, int, *Response) {
	path := fmt.Sprintf("%s/groups?q=%v&include_member_count=%v&filter_allow_reference=%v", c.GetTeamRoute(teamID), opts.Q, opts.IncludeMemberCount, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, 0, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	responseData := struct {
		Groups []*GroupWithSchemeAdmin `json:"groups"`
		Count  int                     `json:"total_group_count"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&responseData); err != nil {
		appErr := NewAppError("Api4.GetGroupsByTeam", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return nil, 0, BuildErrorResponse(r, appErr)
	}

	return responseData.Groups, responseData.Count, BuildResponse(r)
}

// GetGroupsAssociatedToChannelsByTeam retrieves the Mattermost Groups associated with channels in a given team
func (c *Client4) GetGroupsAssociatedToChannelsByTeam(teamID string, opts GroupSearchOpts) (map[string][]*GroupWithSchemeAdmin, *Response) {
	path := fmt.Sprintf("%s/groups_by_channels?q=%v&filter_allow_reference=%v", c.GetTeamRoute(teamID), opts.Q, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	responseData := struct {
		GroupsAssociatedToChannels map[string][]*GroupWithSchemeAdmin `json:"groups"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&responseData); err != nil {
		appErr := NewAppError("Api4.GetGroupsAssociatedToChannelsByTeam", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return nil, BuildErrorResponse(r, appErr)
	}

	return responseData.GroupsAssociatedToChannels, BuildResponse(r)
}

// GetGroups retrieves Mattermost Groups
func (c *Client4) GetGroups(opts GroupSearchOpts) ([]*Group, *Response) {
	path := fmt.Sprintf(
		"%s?include_member_count=%v&not_associated_to_team=%v&not_associated_to_channel=%v&filter_allow_reference=%v&q=%v&filter_parent_team_permitted=%v",
		c.GetGroupsRoute(),
		opts.IncludeMemberCount,
		opts.NotAssociatedToTeam,
		opts.NotAssociatedToChannel,
		opts.FilterAllowReference,
		opts.Q,
		opts.FilterParentTeamPermitted,
	)
	if opts.Since > 0 {
		path = fmt.Sprintf("%s&since=%v", path, opts.Since)
	}
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupsFromJSON(r.Body), BuildResponse(r)
}

// GetGroupsByUserId retrieves Mattermost Groups for a user
func (c *Client4) GetGroupsByUserID(userID string) ([]*Group, *Response) {
	path := fmt.Sprintf(
		"%s/%v/groups",
		c.GetUsersRoute(),
		userID,
	)

	r, appErr := c.DoAPIGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupsFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) MigrateAuthToLdap(fromAuthService string, matchField string, force bool) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/migrate_auth/ldap", StringInterfaceToJSON(map[string]interface{}{
		"from":        fromAuthService,
		"force":       force,
		"match_field": matchField,
	}))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

func (c *Client4) MigrateAuthToSaml(fromAuthService string, usersMap map[string]string, auto bool) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetUsersRoute()+"/migrate_auth/saml", StringInterfaceToJSON(map[string]interface{}{
		"from":    fromAuthService,
		"auto":    auto,
		"matches": usersMap,
	}))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UploadLdapPublicCertificate will upload a public certificate for LDAP and set the config to use it.
func (c *Client4) UploadLdapPublicCertificate(data []byte) (bool, *Response) {
	body, writer, err := fileToMultipart(data, LdapPublicCertificateName)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLdapPublicCertificate", "model.client.upload_ldap_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetLdapRoute()+"/certificate/public", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// UploadLdapPrivateCertificate will upload a private key for LDAP and set the config to use it.
func (c *Client4) UploadLdapPrivateCertificate(data []byte) (bool, *Response) {
	body, writer, err := fileToMultipart(data, LdapPrivateKeyName)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLdapPrivateCertificate", "model.client.upload_Ldap_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetLdapRoute()+"/certificate/private", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// DeleteLdapPublicCertificate deletes the LDAP IDP certificate from the server and updates the config to not use it and disable LDAP.
func (c *Client4) DeleteLdapPublicCertificate() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetLdapRoute() + "/certificate/public")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteLDAPPrivateCertificate deletes the LDAP IDP certificate from the server and updates the config to not use it and disable LDAP.
func (c *Client4) DeleteLdapPrivateCertificate() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetLdapRoute() + "/certificate/private")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Audits Section

// GetAudits returns a list of audits for the whole system.
func (c *Client4) GetAudits(page int, perPage int, etag string) (Audits, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet("/audits"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AuditsFromJSON(r.Body), BuildResponse(r)
}

// Brand Section

// GetBrandImage retrieves the previously uploaded brand image.
func (c *Client4) GetBrandImage() ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetBrandRoute()+"/image", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	if r.StatusCode >= 300 {
		return nil, BuildErrorResponse(r, AppErrorFromJSON(r.Body))
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetBrandImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}

	return data, BuildResponse(r)
}

// DeleteBrandImage deletes the brand image for the system.
func (c *Client4) DeleteBrandImage() *Response {
	r, err := c.DoAPIDelete(c.GetBrandRoute() + "/image")
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	return BuildResponse(r)
}

// UploadBrandImage sets the brand image for the system.
func (c *Client4) UploadBrandImage(data []byte) (bool, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "brand.png")
	if err != nil {
		return false, &Response{Error: NewAppError("UploadBrandImage", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return false, &Response{Error: NewAppError("UploadBrandImage", "model.client.set_profile_user.no_file.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if err = writer.Close(); err != nil {
		return false, &Response{Error: NewAppError("UploadBrandImage", "model.client.set_profile_user.writer.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetBrandRoute()+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("UploadBrandImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetBrandRoute()+"/image", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// Logs Section

// GetLogs page of logs as a string array.
func (c *Client4) GetLogs(page, perPage int) ([]string, *Response) {
	query := fmt.Sprintf("?page=%v&logs_per_page=%v", page, perPage)
	r, err := c.DoAPIGet("/logs"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

// PostLog is a convenience Web Service call so clients can log messages into
// the server-side logs. For example we typically log javascript error messages
// into the server-side. It returns the log message if the logging was successful.
func (c *Client4) PostLog(message map[string]string) (map[string]string, *Response) {
	r, err := c.DoAPIPost("/logs", MapToJSON(message))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body), BuildResponse(r)
}

// OAuth Section

// CreateOAuthApp will register a new OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) CreateOAuthApp(app *OAuthApp) (*OAuthApp, *Response) {
	r, err := c.DoAPIPost(c.GetOAuthAppsRoute(), app.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJSON(r.Body), BuildResponse(r)
}

// UpdateOAuthApp updates a page of registered OAuth 2.0 client applications with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) UpdateOAuthApp(app *OAuthApp) (*OAuthApp, *Response) {
	r, err := c.DoAPIPut(c.GetOAuthAppRoute(app.ID), app.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJSON(r.Body), BuildResponse(r)
}

// GetOAuthApps gets a page of registered OAuth 2.0 client applications with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthApps(page, perPage int) ([]*OAuthApp, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetOAuthAppsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppListFromJSON(r.Body), BuildResponse(r)
}

// GetOAuthApp gets a registered OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthApp(appID string) (*OAuthApp, *Response) {
	r, err := c.DoAPIGet(c.GetOAuthAppRoute(appID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJSON(r.Body), BuildResponse(r)
}

// GetOAuthAppInfo gets a sanitized version of a registered OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthAppInfo(appID string) (*OAuthApp, *Response) {
	r, err := c.DoAPIGet(c.GetOAuthAppRoute(appID)+"/info", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJSON(r.Body), BuildResponse(r)
}

// DeleteOAuthApp deletes a registered OAuth 2.0 client application.
func (c *Client4) DeleteOAuthApp(appID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetOAuthAppRoute(appID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RegenerateOAuthAppSecret regenerates the client secret for a registered OAuth 2.0 client application.
func (c *Client4) RegenerateOAuthAppSecret(appID string) (*OAuthApp, *Response) {
	r, err := c.DoAPIPost(c.GetOAuthAppRoute(appID)+"/regen_secret", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJSON(r.Body), BuildResponse(r)
}

// GetAuthorizedOAuthAppsForUser gets a page of OAuth 2.0 client applications the user has authorized to use access their account.
func (c *Client4) GetAuthorizedOAuthAppsForUser(userID string, page, perPage int) ([]*OAuthApp, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/oauth/apps/authorized"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppListFromJSON(r.Body), BuildResponse(r)
}

// AuthorizeOAuthApp will authorize an OAuth 2.0 client application to access a user's account and provide a redirect link to follow.
func (c *Client4) AuthorizeOAuthApp(authRequest *AuthorizeRequest) (string, *Response) {
	r, err := c.DoAPIRequest(http.MethodPost, c.URL+"/oauth/authorize", authRequest.ToJSON(), "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["redirect"], BuildResponse(r)
}

// DeauthorizeOAuthApp will deauthorize an OAuth 2.0 client application from accessing a user's account.
func (c *Client4) DeauthorizeOAuthApp(appID string) (bool, *Response) {
	requestData := map[string]string{"client_id": appID}
	r, err := c.DoAPIRequest(http.MethodPost, c.URL+"/oauth/deauthorize", MapToJSON(requestData), "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetOAuthAccessToken is a test helper function for the OAuth access token endpoint.
func (c *Client4) GetOAuthAccessToken(data url.Values) (*AccessResponse, *Response) {
	rq, err := http.NewRequest(http.MethodPost, c.URL+"/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, &Response{Error: NewAppError(c.URL+"/oauth/access_token", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.URL+"/oauth/access_token", "model.client.connecting.app_error", nil, err.Error(), 403)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return AccessResponseFromJSON(rp.Body), BuildResponse(rp)
}

// Elasticsearch Section

// TestElasticsearch will attempt to connect to the configured Elasticsearch server and return OK if configured.
// correctly.
func (c *Client4) TestElasticsearch() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetElasticsearchRoute()+"/test", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PurgeElasticsearchIndexes immediately deletes all Elasticsearch indexes.
func (c *Client4) PurgeElasticsearchIndexes() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetElasticsearchRoute()+"/purge_indexes", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Bleve Section

// PurgeBleveIndexes immediately deletes all Bleve indexes.
func (c *Client4) PurgeBleveIndexes() (bool, *Response) {
	r, err := c.DoAPIPost(c.GetBleveRoute()+"/purge_indexes", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Data Retention Section

// GetDataRetentionPolicy will get the current global data retention policy details.
func (c *Client4) GetDataRetentionPolicy() (*GlobalRetentionPolicy, *Response) {
	r, err := c.DoAPIGet(c.GetDataRetentionRoute()+"/policy", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return GlobalRetentionPolicyFromJSON(r.Body), BuildResponse(r)
}

// GetDataRetentionPolicyByID will get the details for the granular data retention policy with the specified ID.
func (c *Client4) GetDataRetentionPolicyByID(policyID string) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.DoAPIGet(c.GetDataRetentionPolicyRoute(policyID), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policy, err := RetentionPolicyWithTeamAndChannelCountsFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetDataRetentionPolicyByID", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return policy, BuildResponse(r)
}

// GetDataRetentionPoliciesCount will get the total number of granular data retention policies.
func (c *Client4) GetDataRetentionPoliciesCount() (int64, *Response) {
	type CountBody struct {
		TotalCount int64 `json:"total_count"`
	}
	r, appErr := c.DoAPIGet(c.GetDataRetentionRoute()+"/policies_count", "")
	if appErr != nil {
		return 0, BuildErrorResponse(r, appErr)
	}
	var countObj CountBody
	jsonErr := json.NewDecoder(r.Body).Decode(&countObj)
	if jsonErr != nil {
		return 0, BuildErrorResponse(r, NewAppError("Client4.GetDataRetentionPoliciesCount", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return countObj.TotalCount, BuildResponse(r)
}

// GetDataRetentionPolicies will get the current granular data retention policies' details.
func (c *Client4) GetDataRetentionPolicies(page, perPage int) (*RetentionPolicyWithTeamAndChannelCountsList, *Response) {
	query := fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	r, appErr := c.DoAPIGet(c.GetDataRetentionRoute()+"/policies"+query, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policies, err := RetentionPolicyWithTeamAndChannelCountsListFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetDataRetentionPolicies", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return policies, BuildResponse(r)
}

// CreateDataRetentionPolicy will create a new granular data retention policy which will be applied to
// the specified teams and channels. The Id field of `policy` must be empty.
func (c *Client4) CreateDataRetentionPolicy(policy *RetentionPolicyWithTeamAndChannelIDs) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.doAPIPostBytes(c.GetDataRetentionRoute()+"/policies", policy.ToJSON())
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	newPolicy, err := RetentionPolicyWithTeamAndChannelCountsFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.CreateDataRetentionPolicy", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return newPolicy, BuildResponse(r)
}

// DeleteDataRetentionPolicy will delete the granular data retention policy with the specified ID.
func (c *Client4) DeleteDataRetentionPolicy(policyID string) *Response {
	r, appErr := c.DoAPIDelete(c.GetDataRetentionPolicyRoute(policyID))
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// PatchDataRetentionPolicy will patch the granular data retention policy with the specified ID.
// The Id field of `patch` must be non-empty.
func (c *Client4) PatchDataRetentionPolicy(patch *RetentionPolicyWithTeamAndChannelIDs) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.doAPIPatchBytes(c.GetDataRetentionPolicyRoute(patch.ID), patch.ToJSON())
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policy, err := RetentionPolicyWithTeamAndChannelCountsFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.PatchDataRetentionPolicy", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return policy, BuildResponse(r)
}

// GetTeamsForRetentionPolicy will get the teams to which the specified policy is currently applied.
func (c *Client4) GetTeamsForRetentionPolicy(policyID string, page, perPage int) (*TeamsWithCount, *Response) {
	query := fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	r, appErr := c.DoAPIGet(c.GetDataRetentionPolicyRoute(policyID)+"/teams"+query, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var teams *TeamsWithCount
	jsonErr := json.NewDecoder(r.Body).Decode(&teams)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetTeamsForRetentionPolicy", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return teams, BuildResponse(r)
}

// SearchTeamsForRetentionPolicy will search the teams to which the specified policy is currently applied.
func (c *Client4) SearchTeamsForRetentionPolicy(policyID string, term string) ([]*Team, *Response) {
	body, _ := json.Marshal(map[string]interface{}{"term": term})
	r, appErr := c.doAPIPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams/search", body)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var teams []*Team
	jsonErr := json.NewDecoder(r.Body).Decode(&teams)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.SearchTeamsForRetentionPolicy", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return teams, BuildResponse(r)
}

// AddTeamsToRetentionPolicy will add the specified teams to the granular data retention policy
// with the specified ID.
func (c *Client4) AddTeamsToRetentionPolicy(policyID string, teamIDs []string) *Response {
	body, _ := json.Marshal(teamIDs)
	r, appErr := c.doAPIPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// RemoveTeamsFromRetentionPolicy will remove the specified teams from the granular data retention policy
// with the specified ID.
func (c *Client4) RemoveTeamsFromRetentionPolicy(policyID string, teamIDs []string) *Response {
	body, _ := json.Marshal(teamIDs)
	r, appErr := c.doAPIDeleteBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// GetChannelsForRetentionPolicy will get the channels to which the specified policy is currently applied.
func (c *Client4) GetChannelsForRetentionPolicy(policyID string, page, perPage int) (*ChannelsWithCount, *Response) {
	query := fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	r, appErr := c.DoAPIGet(c.GetDataRetentionPolicyRoute(policyID)+"/channels"+query, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var channels *ChannelsWithCount
	jsonErr := json.NewDecoder(r.Body).Decode(&channels)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetChannelsForRetentionPolicy", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return channels, BuildResponse(r)
}

// SearchChannelsForRetentionPolicy will search the channels to which the specified policy is currently applied.
func (c *Client4) SearchChannelsForRetentionPolicy(policyID string, term string) (ChannelListWithTeamData, *Response) {
	body, _ := json.Marshal(map[string]interface{}{"term": term})
	r, appErr := c.doAPIPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels/search", body)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var channels ChannelListWithTeamData
	jsonErr := json.NewDecoder(r.Body).Decode(&channels)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.SearchChannelsForRetentionPolicy", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return channels, BuildResponse(r)
}

// AddChannelsToRetentionPolicy will add the specified channels to the granular data retention policy
// with the specified ID.
func (c *Client4) AddChannelsToRetentionPolicy(policyID string, channelIDs []string) *Response {
	body, _ := json.Marshal(channelIDs)
	r, appErr := c.doAPIPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// RemoveChannelsFromRetentionPolicy will remove the specified channels from the granular data retention policy
// with the specified ID.
func (c *Client4) RemoveChannelsFromRetentionPolicy(policyID string, channelIDs []string) *Response {
	body, _ := json.Marshal(channelIDs)
	r, appErr := c.doAPIDeleteBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// GetTeamPoliciesForUser will get the data retention policies for the teams to which a user belongs.
func (c *Client4) GetTeamPoliciesForUser(userID string, offset, limit int) (*RetentionPolicyForTeamList, *Response) {
	r, appErr := c.DoAPIGet(c.GetUserRoute(userID)+"/data_retention/team_policies", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var teams RetentionPolicyForTeamList
	jsonErr := json.NewDecoder(r.Body).Decode(&teams)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetTeamPoliciesForUser", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return &teams, BuildResponse(r)
}

// GetChannelPoliciesForUser will get the data retention policies for the channels to which a user belongs.
func (c *Client4) GetChannelPoliciesForUser(userID string, offset, limit int) (*RetentionPolicyForChannelList, *Response) {
	r, appErr := c.DoAPIGet(c.GetUserRoute(userID)+"/data_retention/channel_policies", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	var channels RetentionPolicyForChannelList
	jsonErr := json.NewDecoder(r.Body).Decode(&channels)
	if jsonErr != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetChannelPoliciesForUser", "model.utils.decode_json.app_error", nil, jsonErr.Error(), r.StatusCode))
	}
	return &channels, BuildResponse(r)
}

// Commands Section

// CreateCommand will create a new command if the user have the right permissions.
func (c *Client4) CreateCommand(cmd *Command) (*Command, *Response) {
	r, err := c.DoAPIPost(c.GetCommandsRoute(), cmd.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJSON(r.Body), BuildResponse(r)
}

// UpdateCommand updates a command based on the provided Command struct.
func (c *Client4) UpdateCommand(cmd *Command) (*Command, *Response) {
	r, err := c.DoAPIPut(c.GetCommandRoute(cmd.ID), cmd.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJSON(r.Body), BuildResponse(r)
}

// MoveCommand moves a command to a different team.
func (c *Client4) MoveCommand(teamID string, commandID string) (bool, *Response) {
	cmr := CommandMoveRequest{TeamID: teamID}
	r, err := c.DoAPIPut(c.GetCommandMoveRoute(commandID), cmr.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteCommand deletes a command based on the provided command id string.
func (c *Client4) DeleteCommand(commandID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetCommandRoute(commandID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ListCommands will retrieve a list of commands available in the team.
func (c *Client4) ListCommands(teamID string, customOnly bool) ([]*Command, *Response) {
	query := fmt.Sprintf("?team_id=%v&custom_only=%v", teamID, customOnly)
	r, err := c.DoAPIGet(c.GetCommandsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandListFromJSON(r.Body), BuildResponse(r)
}

// ListCommandAutocompleteSuggestions will retrieve a list of suggestions for a userInput.
func (c *Client4) ListCommandAutocompleteSuggestions(userInput, teamID string) ([]AutocompleteSuggestion, *Response) {
	query := fmt.Sprintf("/commands/autocomplete_suggestions?user_input=%v", userInput)
	r, err := c.DoAPIGet(c.GetTeamRoute(teamID)+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AutocompleteSuggestionsFromJSON(r.Body), BuildResponse(r)
}

// GetCommandById will retrieve a command by id.
func (c *Client4) GetCommandByID(cmdID string) (*Command, *Response) {
	url := fmt.Sprintf("%s/%s", c.GetCommandsRoute(), cmdID)
	r, err := c.DoAPIGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJSON(r.Body), BuildResponse(r)
}

// ExecuteCommand executes a given slash command.
func (c *Client4) ExecuteCommand(channelID, command string) (*CommandResponse, *Response) {
	commandArgs := &CommandArgs{
		ChannelID: channelID,
		Command:   command,
	}
	r, err := c.DoAPIPost(c.GetCommandsRoute()+"/execute", commandArgs.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	response, _ := CommandResponseFromJSON(r.Body)
	return response, BuildResponse(r)
}

// ExecuteCommandWithTeam executes a given slash command against the specified team.
// Use this when executing slash commands in a DM/GM, since the team id cannot be inferred in that case.
func (c *Client4) ExecuteCommandWithTeam(channelID, teamID, command string) (*CommandResponse, *Response) {
	commandArgs := &CommandArgs{
		ChannelID: channelID,
		TeamID:    teamID,
		Command:   command,
	}
	r, err := c.DoAPIPost(c.GetCommandsRoute()+"/execute", commandArgs.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	response, _ := CommandResponseFromJSON(r.Body)
	return response, BuildResponse(r)
}

// ListAutocompleteCommands will retrieve a list of commands available in the team.
func (c *Client4) ListAutocompleteCommands(teamID string) ([]*Command, *Response) {
	r, err := c.DoAPIGet(c.GetTeamAutoCompleteCommandsRoute(teamID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandListFromJSON(r.Body), BuildResponse(r)
}

// RegenCommandToken will create a new token if the user have the right permissions.
func (c *Client4) RegenCommandToken(commandID string) (string, *Response) {
	r, err := c.DoAPIPut(c.GetCommandRoute(commandID)+"/regen_token", "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["token"], BuildResponse(r)
}

// Status Section

// GetUserStatus returns a user based on the provided user id string.
func (c *Client4) GetUserStatus(userID, etag string) (*Status, *Response) {
	r, err := c.DoAPIGet(c.GetUserStatusRoute(userID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusFromJSON(r.Body), BuildResponse(r)
}

// GetUsersStatusesByIds returns a list of users status based on the provided user ids.
func (c *Client4) GetUsersStatusesByIDs(userIDs []string) ([]*Status, *Response) {
	r, err := c.DoAPIPost(c.GetUserStatusesRoute()+"/ids", ArrayToJSON(userIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusListFromJSON(r.Body), BuildResponse(r)
}

// UpdateUserStatus sets a user's status based on the provided user id string.
func (c *Client4) UpdateUserStatus(userID string, userStatus *Status) (*Status, *Response) {
	r, err := c.DoAPIPut(c.GetUserStatusRoute(userID), userStatus.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusFromJSON(r.Body), BuildResponse(r)
}

// Emoji Section

// CreateEmoji will save an emoji to the server if the current user has permission
// to do so. If successful, the provided emoji will be returned with its Id field
// filled in. Otherwise, an error will be returned.
func (c *Client4) CreateEmoji(emoji *Emoji, image []byte, filename string) (*Emoji, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError("CreateEmoji", "model.client.create_emoji.image.app_error", nil, err.Error(), 0)}
	}

	if _, err := io.Copy(part, bytes.NewBuffer(image)); err != nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError("CreateEmoji", "model.client.create_emoji.image.app_error", nil, err.Error(), 0)}
	}

	if err := writer.WriteField("emoji", emoji.ToJSON()); err != nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError("CreateEmoji", "model.client.create_emoji.emoji.app_error", nil, err.Error(), 0)}
	}

	if err := writer.Close(); err != nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError("CreateEmoji", "model.client.create_emoji.writer.app_error", nil, err.Error(), 0)}
	}

	return c.DoEmojiUploadFile(c.GetEmojisRoute(), body.Bytes(), writer.FormDataContentType())
}

// GetEmojiList returns a page of custom emoji on the system.
func (c *Client4) GetEmojiList(page, perPage int) ([]*Emoji, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoAPIGet(c.GetEmojisRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJSON(r.Body), BuildResponse(r)
}

// GetSortedEmojiList returns a page of custom emoji on the system sorted based on the sort
// parameter, blank for no sorting and "name" to sort by emoji names.
func (c *Client4) GetSortedEmojiList(page, perPage int, sort string) ([]*Emoji, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&sort=%v", page, perPage, sort)
	r, err := c.DoAPIGet(c.GetEmojisRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJSON(r.Body), BuildResponse(r)
}

// DeleteEmoji delete an custom emoji on the provided emoji id string.
func (c *Client4) DeleteEmoji(emojiID string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetEmojiRoute(emojiID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetEmoji returns a custom emoji based on the emojiId string.
func (c *Client4) GetEmoji(emojiID string) (*Emoji, *Response) {
	r, err := c.DoAPIGet(c.GetEmojiRoute(emojiID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiFromJSON(r.Body), BuildResponse(r)
}

// GetEmojiByName returns a custom emoji based on the name string.
func (c *Client4) GetEmojiByName(name string) (*Emoji, *Response) {
	r, err := c.DoAPIGet(c.GetEmojiByNameRoute(name), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiFromJSON(r.Body), BuildResponse(r)
}

// GetEmojiImage returns the emoji image.
func (c *Client4) GetEmojiImage(emojiID string) ([]byte, *Response) {
	r, apErr := c.DoAPIGet(c.GetEmojiRoute(emojiID)+"/image", "")
	if apErr != nil {
		return nil, BuildErrorResponse(r, apErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetEmojiImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}

	return data, BuildResponse(r)
}

// SearchEmoji returns a list of emoji matching some search criteria.
func (c *Client4) SearchEmoji(search *EmojiSearch) ([]*Emoji, *Response) {
	r, err := c.DoAPIPost(c.GetEmojisRoute()+"/search", search.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJSON(r.Body), BuildResponse(r)
}

// AutocompleteEmoji returns a list of emoji starting with or matching name.
func (c *Client4) AutocompleteEmoji(name string, etag string) ([]*Emoji, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoAPIGet(c.GetEmojisRoute()+"/autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJSON(r.Body), BuildResponse(r)
}

// Reaction Section

// SaveReaction saves an emoji reaction for a post. Returns the saved reaction if successful, otherwise an error will be returned.
func (c *Client4) SaveReaction(reaction *Reaction) (*Reaction, *Response) {
	r, err := c.DoAPIPost(c.GetReactionsRoute(), reaction.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ReactionFromJSON(r.Body), BuildResponse(r)
}

// GetReactions returns a list of reactions to a post.
func (c *Client4) GetReactions(postID string) ([]*Reaction, *Response) {
	r, err := c.DoAPIGet(c.GetPostRoute(postID)+"/reactions", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ReactionsFromJSON(r.Body), BuildResponse(r)
}

// DeleteReaction deletes reaction of a user in a post.
func (c *Client4) DeleteReaction(reaction *Reaction) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetUserRoute(reaction.UserID) + c.GetPostRoute(reaction.PostID) + fmt.Sprintf("/reactions/%v", reaction.EmojiName))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// FetchBulkReactions returns a map of postIds and corresponding reactions
func (c *Client4) GetBulkReactions(postIDs []string) (map[string][]*Reaction, *Response) {
	r, err := c.DoAPIPost(c.GetPostsRoute()+"/ids/reactions", ArrayToJSON(postIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapPostIDToReactionsFromJSON(r.Body), BuildResponse(r)
}

// Timezone Section

// GetSupportedTimezone returns a page of supported timezones on the system.
func (c *Client4) GetSupportedTimezone() ([]string, *Response) {
	r, err := c.DoAPIGet(c.GetTimezonesRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	var timezones []string
	json.NewDecoder(r.Body).Decode(&timezones)
	return timezones, BuildResponse(r)
}

// Open Graph Metadata Section

// OpenGraph return the open graph metadata for a particular url if the site have the metadata.
func (c *Client4) OpenGraph(url string) (map[string]string, *Response) {
	requestBody := make(map[string]string)
	requestBody["url"] = url

	r, err := c.DoAPIPost(c.GetOpenGraphRoute(), MapToJSON(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body), BuildResponse(r)
}

// Jobs Section

// GetJob gets a single job.
func (c *Client4) GetJob(id string) (*Job, *Response) {
	r, err := c.DoAPIGet(c.GetJobsRoute()+fmt.Sprintf("/%v", id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobFromJSON(r.Body), BuildResponse(r)
}

// GetJobs gets all jobs, sorted with the job that was created most recently first.
func (c *Client4) GetJobs(page int, perPage int) ([]*Job, *Response) {
	r, err := c.DoAPIGet(c.GetJobsRoute()+fmt.Sprintf("?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobsFromJSON(r.Body), BuildResponse(r)
}

// GetJobsByType gets all jobs of a given type, sorted with the job that was created most recently first.
func (c *Client4) GetJobsByType(jobType string, page int, perPage int) ([]*Job, *Response) {
	r, err := c.DoAPIGet(c.GetJobsRoute()+fmt.Sprintf("/type/%v?page=%v&per_page=%v", jobType, page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobsFromJSON(r.Body), BuildResponse(r)
}

// CreateJob creates a job based on the provided job struct.
func (c *Client4) CreateJob(job *Job) (*Job, *Response) {
	r, err := c.DoAPIPost(c.GetJobsRoute(), job.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobFromJSON(r.Body), BuildResponse(r)
}

// CancelJob requests the cancellation of the job with the provided Id.
func (c *Client4) CancelJob(jobID string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetJobsRoute()+fmt.Sprintf("/%v/cancel", jobID), "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DownloadJob downloads the results of the job
func (c *Client4) DownloadJob(jobID string) ([]byte, *Response) {
	r, appErr := c.DoAPIGet(c.GetJobsRoute()+fmt.Sprintf("/%v/download", jobID), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetFile", "model.client.read_job_result_file.app_error", nil, err.Error(), r.StatusCode))
	}
	return data, BuildResponse(r)
}

// Roles Section

// GetRole gets a single role by ID.
func (c *Client4) GetRole(id string) (*Role, *Response) {
	r, err := c.DoAPIGet(c.GetRolesRoute()+fmt.Sprintf("/%v", id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJSON(r.Body), BuildResponse(r)
}

// GetRoleByName gets a single role by Name.
func (c *Client4) GetRoleByName(name string) (*Role, *Response) {
	r, err := c.DoAPIGet(c.GetRolesRoute()+fmt.Sprintf("/name/%v", name), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJSON(r.Body), BuildResponse(r)
}

// GetRolesByNames returns a list of roles based on the provided role names.
func (c *Client4) GetRolesByNames(roleNames []string) ([]*Role, *Response) {
	r, err := c.DoAPIPost(c.GetRolesRoute()+"/names", ArrayToJSON(roleNames))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleListFromJSON(r.Body), BuildResponse(r)
}

// PatchRole partially updates a role in the system. Any missing fields are not updated.
func (c *Client4) PatchRole(roleID string, patch *RolePatch) (*Role, *Response) {
	r, err := c.DoAPIPut(c.GetRolesRoute()+fmt.Sprintf("/%v/patch", roleID), patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJSON(r.Body), BuildResponse(r)
}

// Schemes Section

// CreateScheme creates a new Scheme.
func (c *Client4) CreateScheme(scheme *Scheme) (*Scheme, *Response) {
	r, err := c.DoAPIPost(c.GetSchemesRoute(), scheme.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJSON(r.Body), BuildResponse(r)
}

// GetScheme gets a single scheme by ID.
func (c *Client4) GetScheme(id string) (*Scheme, *Response) {
	r, err := c.DoAPIGet(c.GetSchemeRoute(id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJSON(r.Body), BuildResponse(r)
}

// GetSchemes gets all schemes, sorted with the most recently created first, optionally filtered by scope.
func (c *Client4) GetSchemes(scope string, page int, perPage int) ([]*Scheme, *Response) {
	r, err := c.DoAPIGet(c.GetSchemesRoute()+fmt.Sprintf("?scope=%v&page=%v&per_page=%v", scope, page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemesFromJSON(r.Body), BuildResponse(r)
}

// DeleteScheme deletes a single scheme by ID.
func (c *Client4) DeleteScheme(id string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetSchemeRoute(id))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PatchScheme partially updates a scheme in the system. Any missing fields are not updated.
func (c *Client4) PatchScheme(id string, patch *SchemePatch) (*Scheme, *Response) {
	r, err := c.DoAPIPut(c.GetSchemeRoute(id)+"/patch", patch.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJSON(r.Body), BuildResponse(r)
}

// GetTeamsForScheme gets the teams using this scheme, sorted alphabetically by display name.
func (c *Client4) GetTeamsForScheme(schemeID string, page int, perPage int) ([]*Team, *Response) {
	r, err := c.DoAPIGet(c.GetSchemeRoute(schemeID)+fmt.Sprintf("/teams?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJSON(r.Body), BuildResponse(r)
}

// GetChannelsForScheme gets the channels using this scheme, sorted alphabetically by display name.
func (c *Client4) GetChannelsForScheme(schemeID string, page int, perPage int) (ChannelList, *Response) {
	r, err := c.DoAPIGet(c.GetSchemeRoute(schemeID)+fmt.Sprintf("/channels?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return *ChannelListFromJSON(r.Body), BuildResponse(r)
}

// Plugin Section

// UploadPlugin takes an io.Reader stream pointing to the contents of a .tar.gz plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) UploadPlugin(file io.Reader) (*Manifest, *Response) {
	return c.uploadPlugin(file, false)
}

func (c *Client4) UploadPluginForced(file io.Reader) (*Manifest, *Response) {
	return c.uploadPlugin(file, true)
}

func (c *Client4) uploadPlugin(file io.Reader, force bool) (*Manifest, *Response) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if force {
		err := writer.WriteField("force", c.boolString(true))
		if err != nil {
			return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.writer.app_error", nil, err.Error(), 0)}
		}
	}

	part, err := writer.CreateFormFile("plugin", "plugin.tar.gz")
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.writer.app_error", nil, err.Error(), 0)}
	}

	if _, err = io.Copy(part, file); err != nil {
		return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.writer.app_error", nil, err.Error(), 0)}
	}

	if err = writer.Close(); err != nil {
		return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.writer.app_error", nil, err.Error(), 0)}
	}

	rq, err := http.NewRequest("POST", c.APIURL+c.GetPluginsRoute(), body)
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError("UploadPlugin", "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJSON(rp.Body))
	}

	return ManifestFromJSON(rp.Body), BuildResponse(rp)
}

func (c *Client4) InstallPluginFromURL(downloadURL string, force bool) (*Manifest, *Response) {
	forceStr := c.boolString(force)

	url := fmt.Sprintf("%s?plugin_download_url=%s&force=%s", c.GetPluginsRoute()+"/install_from_url", url.QueryEscape(downloadURL), forceStr)
	r, err := c.DoAPIPost(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ManifestFromJSON(r.Body), BuildResponse(r)
}

// InstallMarketplacePlugin will install marketplace plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) InstallMarketplacePlugin(request *InstallMarketplacePluginRequest) (*Manifest, *Response) {
	json, err := request.ToJSON()
	if err != nil {
		return nil, &Response{Error: NewAppError("InstallMarketplacePlugin", "model.client.plugin_request_to_json.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	r, appErr := c.DoAPIPost(c.GetPluginsRoute()+"/marketplace", json)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return ManifestFromJSON(r.Body), BuildResponse(r)
}

// GetPlugins will return a list of plugin manifests for currently active plugins.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetPlugins() (*PluginsResponse, *Response) {
	r, err := c.DoAPIGet(c.GetPluginsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PluginsResponseFromJSON(r.Body), BuildResponse(r)
}

// GetPluginStatuses will return the plugins installed on any server in the cluster, for reporting
// to the administrator via the system console.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetPluginStatuses() (PluginStatuses, *Response) {
	r, err := c.DoAPIGet(c.GetPluginsRoute()+"/statuses", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PluginStatusesFromJSON(r.Body), BuildResponse(r)
}

// RemovePlugin will disable and delete a plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) RemovePlugin(id string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetPluginRoute(id))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetWebappPlugins will return a list of plugins that the webapp should download.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetWebappPlugins() ([]*Manifest, *Response) {
	r, err := c.DoAPIGet(c.GetPluginsRoute()+"/webapp", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ManifestListFromJSON(r.Body), BuildResponse(r)
}

// EnablePlugin will enable an plugin installed.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) EnablePlugin(id string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPluginRoute(id)+"/enable", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DisablePlugin will disable an enabled plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) DisablePlugin(id string) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPluginRoute(id)+"/disable", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetMarketplacePlugins will return a list of plugins that an admin can install.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetMarketplacePlugins(filter *MarketplacePluginFilter) ([]*MarketplacePlugin, *Response) {
	route := c.GetPluginsRoute() + "/marketplace"
	u, parseErr := url.Parse(route)
	if parseErr != nil {
		return nil, &Response{Error: NewAppError("GetMarketplacePlugins", "model.client.parse_plugins.app_error", nil, parseErr.Error(), http.StatusBadRequest)}
	}

	filter.ApplyToURL(u)

	r, err := c.DoAPIGet(u.String(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	plugins, readerErr := MarketplacePluginsFromReader(r.Body)
	if readerErr != nil {
		return nil, BuildErrorResponse(r, NewAppError(route, "model.client.parse_plugins.app_error", nil, err.Error(), http.StatusBadRequest))
	}

	return plugins, BuildResponse(r)
}

// UpdateChannelScheme will update a channel's scheme.
func (c *Client4) UpdateChannelScheme(channelID, schemeID string) (bool, *Response) {
	sip := &SchemeIDPatch{SchemeID: &schemeID}
	r, err := c.DoAPIPut(c.GetChannelSchemeRoute(channelID), sip.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamScheme will update a team's scheme.
func (c *Client4) UpdateTeamScheme(teamID, schemeID string) (bool, *Response) {
	sip := &SchemeIDPatch{SchemeID: &schemeID}
	r, err := c.DoAPIPut(c.GetTeamSchemeRoute(teamID), sip.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetRedirectLocation retrieves the value of the 'Location' header of an HTTP response for a given URL.
func (c *Client4) GetRedirectLocation(urlParam, etag string) (string, *Response) {
	url := fmt.Sprintf("%s?url=%s", c.GetRedirectLocationRoute(), url.QueryEscape(urlParam))
	r, err := c.DoAPIGet(url, etag)
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJSON(r.Body)["location"], BuildResponse(r)
}

// SetServerBusy will mark the server as busy, which disables non-critical services for `secs` seconds.
func (c *Client4) SetServerBusy(secs int) (bool, *Response) {
	url := fmt.Sprintf("%s?seconds=%d", c.GetServerBusyRoute(), secs)
	r, err := c.DoAPIPost(url, "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ClearServerBusy will mark the server as not busy.
func (c *Client4) ClearServerBusy() (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetServerBusyRoute())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetServerBusy returns the current ServerBusyState including the time when a server marked busy
// will automatically have the flag cleared.
func (c *Client4) GetServerBusy() (*ServerBusyState, *Response) {
	r, err := c.DoAPIGet(c.GetServerBusyRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	sbs := ServerBusyStateFromJSON(r.Body)
	return sbs, BuildResponse(r)
}

// GetServerBusyExpires returns the time when a server marked busy
// will automatically have the flag cleared.
//
// Deprecated: Use GetServerBusy instead.
func (c *Client4) GetServerBusyExpires() (*time.Time, *Response) {
	r, err := c.DoAPIGet(c.GetServerBusyRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	sbs := ServerBusyStateFromJSON(r.Body)
	expires := time.Unix(sbs.Expires, 0)
	return &expires, BuildResponse(r)
}

// RegisterTermsOfServiceAction saves action performed by a user against a specific terms of service.
func (c *Client4) RegisterTermsOfServiceAction(userID, termsOfServiceID string, accepted bool) (*bool, *Response) {
	url := c.GetUserTermsOfServiceRoute(userID)
	data := map[string]interface{}{"termsOfServiceId": termsOfServiceID, "accepted": accepted}
	r, err := c.DoAPIPost(url, StringInterfaceToJSON(data))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return NewBool(CheckStatusOK(r)), BuildResponse(r)
}

// GetTermsOfService fetches the latest terms of service
func (c *Client4) GetTermsOfService(etag string) (*TermsOfService, *Response) {
	url := c.GetTermsOfServiceRoute()
	r, err := c.DoAPIGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TermsOfServiceFromJSON(r.Body), BuildResponse(r)
}

// GetUserTermsOfService fetches user's latest terms of service action if the latest action was for acceptance.
func (c *Client4) GetUserTermsOfService(userID, etag string) (*UserTermsOfService, *Response) {
	url := c.GetUserTermsOfServiceRoute(userID)
	r, err := c.DoAPIGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserTermsOfServiceFromJSON(r.Body), BuildResponse(r)
}

// CreateTermsOfService creates new terms of service.
func (c *Client4) CreateTermsOfService(text, userID string) (*TermsOfService, *Response) {
	url := c.GetTermsOfServiceRoute()
	data := map[string]interface{}{"text": text}
	r, err := c.DoAPIPost(url, StringInterfaceToJSON(data))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TermsOfServiceFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetGroup(groupID, etag string) (*Group, *Response) {
	r, appErr := c.DoAPIGet(c.GetGroupRoute(groupID), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) PatchGroup(groupID string, patch *GroupPatch) (*Group, *Response) {
	payload, _ := json.Marshal(patch)
	r, appErr := c.DoAPIPut(c.GetGroupRoute(groupID)+"/patch", string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) LinkGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, patch *GroupSyncablePatch) (*GroupSyncable, *Response) {
	payload, _ := json.Marshal(patch)
	url := fmt.Sprintf("%s/link", c.GetGroupSyncableRoute(groupID, syncableID, syncableType))
	r, appErr := c.DoAPIPost(url, string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) UnlinkGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType) *Response {
	url := fmt.Sprintf("%s/link", c.GetGroupSyncableRoute(groupID, syncableID, syncableType))
	r, appErr := c.DoAPIDelete(url)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

func (c *Client4) GetGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, etag string) (*GroupSyncable, *Response) {
	r, appErr := c.DoAPIGet(c.GetGroupSyncableRoute(groupID, syncableID, syncableType), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetGroupSyncables(groupID string, syncableType GroupSyncableType, etag string) ([]*GroupSyncable, *Response) {
	r, appErr := c.DoAPIGet(c.GetGroupSyncablesRoute(groupID, syncableType), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncablesFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) PatchGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, patch *GroupSyncablePatch) (*GroupSyncable, *Response) {
	payload, _ := json.Marshal(patch)
	r, appErr := c.DoAPIPut(c.GetGroupSyncableRoute(groupID, syncableID, syncableType)+"/patch", string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) TeamMembersMinusGroupMembers(teamID string, groupIDs []string, page, perPage int, etag string) ([]*UserWithGroups, int64, *Response) {
	groupIDStr := strings.Join(groupIDs, ",")
	query := fmt.Sprintf("?group_ids=%s&page=%d&per_page=%d", groupIDStr, page, perPage)
	r, err := c.DoAPIGet(c.GetTeamRoute(teamID)+"/members_minus_group_members"+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	ugc := UsersWithGroupsAndCountFromJSON(r.Body)
	return ugc.Users, ugc.Count, BuildResponse(r)
}

func (c *Client4) ChannelMembersMinusGroupMembers(channelID string, groupIDs []string, page, perPage int, etag string) ([]*UserWithGroups, int64, *Response) {
	groupIDStr := strings.Join(groupIDs, ",")
	query := fmt.Sprintf("?group_ids=%s&page=%d&per_page=%d", groupIDStr, page, perPage)
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/members_minus_group_members"+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	ugc := UsersWithGroupsAndCountFromJSON(r.Body)
	return ugc.Users, ugc.Count, BuildResponse(r)
}

func (c *Client4) PatchConfig(config *Config) (*Config, *Response) {
	r, err := c.DoAPIPut(c.GetConfigRoute()+"/patch", config.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetChannelModerations(channelID string, etag string) ([]*ChannelModeration, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/moderations", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelModerationsFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) PatchChannelModerations(channelID string, patch []*ChannelModerationPatch) ([]*ChannelModeration, *Response) {
	payload, _ := json.Marshal(patch)
	r, err := c.DoAPIPut(c.GetChannelRoute(channelID)+"/moderations/patch", string(payload))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelModerationsFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetKnownUsers() ([]string, *Response) {
	r, err := c.DoAPIGet(c.GetUsersRoute()+"/known", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	var userIDs []string
	json.NewDecoder(r.Body).Decode(&userIDs)
	return userIDs, BuildResponse(r)
}

// PublishUserTyping publishes a user is typing websocket event based on the provided TypingRequest.
func (c *Client4) PublishUserTyping(userID string, typingRequest TypingRequest) (bool, *Response) {
	r, err := c.DoAPIPost(c.GetPublishUserTypingRoute(userID), typingRequest.ToJSON())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

func (c *Client4) GetChannelMemberCountsByGroup(channelID string, includeTimezones bool, etag string) ([]*ChannelMemberCountByGroup, *Response) {
	r, err := c.DoAPIGet(c.GetChannelRoute(channelID)+"/member_counts_by_group?include_timezones="+strconv.FormatBool(includeTimezones), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberCountsByGroupFromJSON(r.Body), BuildResponse(r)
}

// RequestTrialLicense will request a trial license and install it in the server
func (c *Client4) RequestTrialLicense(users int) (bool, *Response) {
	b, _ := json.Marshal(map[string]interface{}{"users": users, "terms_accepted": true})
	r, err := c.DoAPIPost("/trial-license", string(b))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetGroupStats retrieves stats for a Mattermost Group
func (c *Client4) GetGroupStats(groupID string) (*GroupStats, *Response) {
	r, appErr := c.DoAPIGet(c.GetGroupRoute(groupID)+"/stats", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupStatsFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetSidebarCategoriesForTeamForUser(userID, teamID, etag string) (*OrderedSidebarCategories, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID)
	r, appErr := c.DoAPIGet(route, etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	cat, err := OrderedSidebarCategoriesFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return cat, BuildResponse(r)
}

func (c *Client4) CreateSidebarCategoryForTeamForUser(userID, teamID string, category *SidebarCategoryWithChannels) (*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(category)
	route := c.GetUserCategoryRoute(userID, teamID)
	r, appErr := c.doAPIPostBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.CreateSidebarCategoryForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return cat, BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoriesForTeamForUser(userID, teamID string, categories []*SidebarCategoryWithChannels) ([]*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(categories)
	route := c.GetUserCategoryRoute(userID, teamID)

	r, appErr := c.doAPIPutBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	categories, err := SidebarCategoriesFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return categories, BuildResponse(r)
}

func (c *Client4) GetSidebarCategoryOrderForTeamForUser(userID, teamID, etag string) ([]string, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID) + "/order"
	r, err := c.DoAPIGet(route, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoryOrderForTeamForUser(userID, teamID string, order []string) ([]string, *Response) {
	payload, _ := json.Marshal(order)
	route := c.GetUserCategoryRoute(userID, teamID) + "/order"
	r, err := c.doAPIPutBytes(route, payload)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) GetSidebarCategoryForTeamForUser(userID, teamID, categoryID, etag string) (*SidebarCategoryWithChannels, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID) + "/" + categoryID
	r, appErr := c.DoAPIGet(route, etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return cat, BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoryForTeamForUser(userID, teamID, categoryID string, category *SidebarCategoryWithChannels) (*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(category)
	route := c.GetUserCategoryRoute(userID, teamID) + "/" + categoryID
	r, appErr := c.doAPIPutBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJSON(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return cat, BuildResponse(r)
}

// CheckIntegrity performs a database integrity check.
func (c *Client4) CheckIntegrity() ([]IntegrityCheckResult, *Response) {
	r, err := c.DoAPIPost("/integrity", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	var results []IntegrityCheckResult
	if err := json.NewDecoder(r.Body).Decode(&results); err != nil {
		appErr := NewAppError("Api4.CheckIntegrity", "api.marshal_error", nil, err.Error(), http.StatusInternalServerError)
		return nil, BuildErrorResponse(r, appErr)
	}
	return results, BuildResponse(r)
}

func (c *Client4) GetNotices(lastViewed int64, teamID string, client NoticeClientType, clientVersion, locale, etag string) (NoticeMessages, *Response) {
	url := fmt.Sprintf("/system/notices/%s?lastViewed=%d&client=%s&clientVersion=%s&locale=%s", teamID, lastViewed, client, clientVersion, locale)
	r, appErr := c.DoAPIGet(url, etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	notices, err := UnmarshalProductNoticeMessages(r.Body)
	if err != nil {
		return nil, &Response{StatusCode: http.StatusBadRequest, Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	return notices, BuildResponse(r)
}

func (c *Client4) MarkNoticesViewed(ids []string) *Response {
	r, err := c.DoAPIPut("/system/notices/view", ArrayToJSON(ids))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// CreateUpload creates a new upload session.
func (c *Client4) CreateUpload(us *UploadSession) (*UploadSession, *Response) {
	r, err := c.DoAPIPost(c.GetUploadsRoute(), us.ToJSON())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionFromJSON(r.Body), BuildResponse(r)
}

// GetUpload returns the upload session for the specified uploadId.
func (c *Client4) GetUpload(uploadID string) (*UploadSession, *Response) {
	r, err := c.DoAPIGet(c.GetUploadRoute(uploadID), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionFromJSON(r.Body), BuildResponse(r)
}

// GetUploadsForUser returns the upload sessions created by the specified
// userId.
func (c *Client4) GetUploadsForUser(userID string) ([]*UploadSession, *Response) {
	r, err := c.DoAPIGet(c.GetUserRoute(userID)+"/uploads", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionsFromJSON(r.Body), BuildResponse(r)
}

// UploadData performs an upload. On success it returns
// a FileInfo object.
func (c *Client4) UploadData(uploadID string, data io.Reader) (*FileInfo, *Response) {
	url := c.GetUploadRoute(uploadID)
	r, err := c.doAPIRequestReader("POST", c.APIURL+url, data, nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) UpdatePassword(userID, currentPassword, newPassword string) *Response {
	requestBody := map[string]string{"current_password": currentPassword, "new_password": newPassword}
	r, err := c.DoAPIPut(c.GetUserRoute(userID)+"/password", MapToJSON(requestBody))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// Cloud Section

func (c *Client4) GetCloudProducts() ([]*Product, *Response) {
	r, appErr := c.DoAPIGet(c.GetCloudRoute()+"/products", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var cloudProducts []*Product
	json.NewDecoder(r.Body).Decode(&cloudProducts)

	return cloudProducts, BuildResponse(r)
}

func (c *Client4) CreateCustomerPayment() (*StripeSetupIntent, *Response) {
	r, appErr := c.DoAPIPost(c.GetCloudRoute()+"/payment", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var setupIntent *StripeSetupIntent
	json.NewDecoder(r.Body).Decode(&setupIntent)

	return setupIntent, BuildResponse(r)
}

func (c *Client4) ConfirmCustomerPayment(confirmRequest *ConfirmPaymentMethodRequest) *Response {
	json, _ := json.Marshal(confirmRequest)

	r, appErr := c.doAPIPostBytes(c.GetCloudRoute()+"/payment/confirm", json)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) GetCloudCustomer() (*CloudCustomer, *Response) {
	r, appErr := c.DoAPIGet(c.GetCloudRoute()+"/customer", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var cloudCustomer *CloudCustomer
	json.NewDecoder(r.Body).Decode(&cloudCustomer)

	return cloudCustomer, BuildResponse(r)
}

func (c *Client4) GetSubscription() (*Subscription, *Response) {
	r, appErr := c.DoAPIGet(c.GetCloudRoute()+"/subscription", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var subscription *Subscription
	json.NewDecoder(r.Body).Decode(&subscription)

	return subscription, BuildResponse(r)
}

func (c *Client4) GetSubscriptionStats() (*SubscriptionStats, *Response) {
	r, appErr := c.DoAPIGet(c.GetCloudRoute()+"/subscription/stats", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var stats *SubscriptionStats
	json.NewDecoder(r.Body).Decode(&stats)
	return stats, BuildResponse(r)
}

func (c *Client4) GetInvoicesForSubscription() ([]*Invoice, *Response) {
	r, appErr := c.DoAPIGet(c.GetCloudRoute()+"/subscription/invoices", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var invoices []*Invoice
	json.NewDecoder(r.Body).Decode(&invoices)

	return invoices, BuildResponse(r)
}

func (c *Client4) UpdateCloudCustomer(customerInfo *CloudCustomerInfo) (*CloudCustomer, *Response) {
	customerBytes, _ := json.Marshal(customerInfo)

	r, appErr := c.doAPIPutBytes(c.GetCloudRoute()+"/customer", customerBytes)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var customer *CloudCustomer
	json.NewDecoder(r.Body).Decode(&customer)

	return customer, BuildResponse(r)
}

func (c *Client4) UpdateCloudCustomerAddress(address *Address) (*CloudCustomer, *Response) {
	addressBytes, _ := json.Marshal(address)

	r, appErr := c.doAPIPutBytes(c.GetCloudRoute()+"/customer/address", addressBytes)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var customer *CloudCustomer
	json.NewDecoder(r.Body).Decode(&customer)

	return customer, BuildResponse(r)
}

func (c *Client4) ListImports() ([]string, *Response) {
	r, err := c.DoAPIGet(c.GetImportsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) ListExports() ([]string, *Response) {
	r, err := c.DoAPIGet(c.GetExportsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJSON(r.Body), BuildResponse(r)
}

func (c *Client4) DeleteExport(name string) (bool, *Response) {
	r, err := c.DoAPIDelete(c.GetExportRoute(name))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

func (c *Client4) DownloadExport(name string, wr io.Writer, offset int64) (int64, *Response) {
	var headers map[string]string
	if offset > 0 {
		headers = map[string]string{
			HeaderRange: fmt.Sprintf("bytes=%d-", offset),
		}
	}
	r, appErr := c.DoAPIRequestWithHeaders(http.MethodGet, c.APIURL+c.GetExportRoute(name), "", headers)
	if appErr != nil {
		return 0, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	n, err := io.Copy(wr, r.Body)
	if err != nil {
		return n, BuildErrorResponse(r, NewAppError("DownloadExport", "model.client.copy.app_error", nil, err.Error(), r.StatusCode))
	}
	return n, BuildResponse(r)
}

func (c *Client4) GetUserThreads(userID, teamID string, options GetUserThreadsOpts) (*Threads, *Response) {
	v := url.Values{}
	if options.Since != 0 {
		v.Set("since", fmt.Sprintf("%d", options.Since))
	}
	if options.Before != "" {
		v.Set("before", options.Before)
	}
	if options.After != "" {
		v.Set("after", options.After)
	}
	if options.PageSize != 0 {
		v.Set("pageSize", fmt.Sprintf("%d", options.PageSize))
	}
	if options.Extended {
		v.Set("extended", "true")
	}
	if options.Deleted {
		v.Set("deleted", "true")
	}
	if options.Unread {
		v.Set("unread", "true")
	}
	url := c.GetUserThreadsRoute(userID, teamID)
	if len(v) > 0 {
		url += "?" + v.Encode()
	}

	r, appErr := c.DoAPIGet(url, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var threads Threads
	json.NewDecoder(r.Body).Decode(&threads)

	return &threads, BuildResponse(r)
}

func (c *Client4) GetUserThread(userID, teamID, threadID string, extended bool) (*ThreadResponse, *Response) {
	url := c.GetUserThreadRoute(userID, teamID, threadID)
	if extended {
		url += "?extended=true"
	}
	r, appErr := c.DoAPIGet(url, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var thread ThreadResponse
	json.NewDecoder(r.Body).Decode(&thread)

	return &thread, BuildResponse(r)
}

func (c *Client4) UpdateThreadsReadForUser(userID, teamID string) *Response {
	r, appErr := c.DoAPIPut(fmt.Sprintf("%s/read", c.GetUserThreadsRoute(userID, teamID)), "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) UpdateThreadReadForUser(userID, teamID, threadID string, timestamp int64) (*ThreadResponse, *Response) {
	r, appErr := c.DoAPIPut(fmt.Sprintf("%s/read/%d", c.GetUserThreadRoute(userID, teamID, threadID), timestamp), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	var thread ThreadResponse
	json.NewDecoder(r.Body).Decode(&thread)

	return &thread, BuildResponse(r)
}

func (c *Client4) UpdateThreadFollowForUser(userID, teamID, threadID string, state bool) *Response {
	var appErr *AppError
	var r *http.Response
	if state {
		r, appErr = c.DoAPIPut(c.GetUserThreadRoute(userID, teamID, threadID)+"/following", "")
	} else {
		r, appErr = c.DoAPIDelete(c.GetUserThreadRoute(userID, teamID, threadID) + "/following")
	}
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) SendAdminUpgradeRequestEmail() *Response {
	r, appErr := c.DoAPIPost(c.GetCloudRoute()+"/subscription/limitreached/invite", "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) SendAdminUpgradeRequestEmailOnJoin() *Response {
	r, appErr := c.DoAPIPost(c.GetCloudRoute()+"/subscription/limitreached/join", "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) GetAllSharedChannels(teamID string, page, perPage int) ([]*SharedChannel, *Response) {
	url := fmt.Sprintf("%s/%s?page=%d&per_page=%d", c.GetSharedChannelsRoute(), teamID, page, perPage)
	r, appErr := c.DoAPIGet(url, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var channels []*SharedChannel
	json.NewDecoder(r.Body).Decode(&channels)

	return channels, BuildResponse(r)
}

func (c *Client4) GetRemoteClusterInfo(remoteID string) (RemoteClusterInfo, *Response) {
	url := fmt.Sprintf("%s/remote_info/%s", c.GetSharedChannelsRoute(), remoteID)
	r, appErr := c.DoAPIGet(url, "")
	if appErr != nil {
		return RemoteClusterInfo{}, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var rci RemoteClusterInfo
	json.NewDecoder(r.Body).Decode(&rci)

	return rci, BuildResponse(r)
}

func (c *Client4) GetAncillaryPermissions(subsectionPermissions []string) ([]string, *Response) {
	var returnedPermissions []string
	url := fmt.Sprintf("%s/ancillary?subsection_permissions=%s", c.GetPermissionsRoute(), strings.Join(subsectionPermissions, ","))
	r, appErr := c.DoAPIGet(url, "")
	if appErr != nil {
		return returnedPermissions, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	json.NewDecoder(r.Body).Decode(&returnedPermissions)
	return returnedPermissions, BuildResponse(r)
}
