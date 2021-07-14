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
	HEADER_REQUEST_ID          = "X-Request-ID"
	HEADER_VERSION_ID          = "X-Version-ID"
	HEADER_CLUSTER_ID          = "X-Cluster-ID"
	HEADER_ETAG_SERVER         = "ETag"
	HEADER_ETAG_CLIENT         = "If-None-Match"
	HEADER_FORWARDED           = "X-Forwarded-For"
	HEADER_REAL_IP             = "X-Real-IP"
	HEADER_FORWARDED_PROTO     = "X-Forwarded-Proto"
	HEADER_TOKEN               = "token"
	HEADER_CSRF_TOKEN          = "X-CSRF-Token"
	HEADER_BEARER              = "BEARER"
	HEADER_AUTH                = "Authorization"
	HEADER_CLOUD_TOKEN         = "X-Cloud-Token"
	HEADER_REMOTECLUSTER_TOKEN = "X-RemoteCluster-Token"
	HEADER_REMOTECLUSTER_ID    = "X-RemoteCluster-Id"
	HEADER_REQUESTED_WITH      = "X-Requested-With"
	HEADER_REQUESTED_WITH_XML  = "XMLHttpRequest"
	HEADER_RANGE               = "Range"
	STATUS                     = "status"
	STATUS_OK                  = "OK"
	STATUS_FAIL                = "FAIL"
	STATUS_UNHEALTHY           = "UNHEALTHY"
	STATUS_REMOVE              = "REMOVE"

	CLIENT_DIR = "client"

	API_URL_SUFFIX_V1 = "/api/v1"
	API_URL_SUFFIX_V4 = "/api/v4"
	API_URL_SUFFIX    = API_URL_SUFFIX_V4
)

type Response struct {
	StatusCode    int
	Error         *AppError
	RequestId     string
	Etag          string
	ServerVersion string
	Header        http.Header
}

type Client4 struct {
	Url        string       // The location of the server, for example  "http://localhost:8065"
	ApiUrl     string       // The api location of the server, for example "http://localhost:8065/api/v4"
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
	return &Client4{url, url + API_URL_SUFFIX, &http.Client{}, "", "", map[string]string{}, "", ""}
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
		RequestId:     r.Header.Get(HEADER_REQUEST_ID),
		Etag:          r.Header.Get(HEADER_ETAG_SERVER),
		ServerVersion: r.Header.Get(HEADER_VERSION_ID),
		Header:        r.Header,
	}
}

func (c *Client4) SetToken(token string) {
	c.AuthToken = token
	c.AuthType = HEADER_BEARER
}

// MockSession is deprecated in favour of SetToken
func (c *Client4) MockSession(token string) {
	c.SetToken(token)
}

func (c *Client4) SetOAuthToken(token string) {
	c.AuthToken = token
	c.AuthType = HEADER_TOKEN
}

func (c *Client4) ClearOAuthToken() {
	c.AuthToken = ""
	c.AuthType = HEADER_BEARER
}

func (c *Client4) GetUsersRoute() string {
	return "/users"
}

func (c *Client4) GetUserRoute(userId string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/%v", userId)
}

func (c *Client4) GetUserThreadsRoute(userID, teamID string) string {
	return c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/threads"
}

func (c *Client4) GetUserThreadRoute(userId, teamId, threadId string) string {
	return c.GetUserThreadsRoute(userId, teamId) + "/" + threadId
}

func (c *Client4) GetUserCategoryRoute(userID, teamID string) string {
	return c.GetUserRoute(userID) + c.GetTeamRoute(teamID) + "/channels/categories"
}

func (c *Client4) GetUserAccessTokensRoute() string {
	return fmt.Sprintf(c.GetUsersRoute() + "/tokens")
}

func (c *Client4) GetUserAccessTokenRoute(tokenId string) string {
	return fmt.Sprintf(c.GetUsersRoute()+"/tokens/%v", tokenId)
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

func (c *Client4) GetBotRoute(botUserId string) string {
	return fmt.Sprintf("%s/%s", c.GetBotsRoute(), botUserId)
}

func (c *Client4) GetTeamsRoute() string {
	return "/teams"
}

func (c *Client4) GetTeamRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v", teamId)
}

func (c *Client4) GetTeamAutoCompleteCommandsRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v/commands/autocomplete", teamId)
}

func (c *Client4) GetTeamByNameRoute(teamName string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/name/%v", teamName)
}

func (c *Client4) GetTeamMemberRoute(teamId, userId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId)+"/members/%v", userId)
}

func (c *Client4) GetTeamMembersRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId) + "/members")
}

func (c *Client4) GetTeamStatsRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId) + "/stats")
}

func (c *Client4) GetTeamImportRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId) + "/import")
}

func (c *Client4) GetChannelsRoute() string {
	return "/channels"
}

func (c *Client4) GetChannelsForTeamRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId) + "/channels")
}

func (c *Client4) GetChannelRoute(channelId string) string {
	return fmt.Sprintf(c.GetChannelsRoute()+"/%v", channelId)
}

func (c *Client4) GetChannelByNameRoute(channelName, teamId string) string {
	return fmt.Sprintf(c.GetTeamRoute(teamId)+"/channels/name/%v", channelName)
}

func (c *Client4) GetChannelsForTeamForUserRoute(teamId, userId string, includeDeleted bool) string {
	route := fmt.Sprintf(c.GetUserRoute(userId) + c.GetTeamRoute(teamId) + "/channels")
	if includeDeleted {
		query := fmt.Sprintf("?include_deleted=%v", includeDeleted)
		return route + query
	}
	return route
}

func (c *Client4) GetChannelByNameForTeamNameRoute(channelName, teamName string) string {
	return fmt.Sprintf(c.GetTeamByNameRoute(teamName)+"/channels/name/%v", channelName)
}

func (c *Client4) GetChannelMembersRoute(channelId string) string {
	return fmt.Sprintf(c.GetChannelRoute(channelId) + "/members")
}

func (c *Client4) GetChannelMemberRoute(channelId, userId string) string {
	return fmt.Sprintf(c.GetChannelMembersRoute(channelId)+"/%v", userId)
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

func (c *Client4) GetPostRoute(postId string) string {
	return fmt.Sprintf(c.GetPostsRoute()+"/%v", postId)
}

func (c *Client4) GetPostsByIDsRoute() string {
	return c.GetPostsRoute() + "/ids"
}

func (c *Client4) GetFilesRoute() string {
	return "/files"
}

func (c *Client4) GetFileRoute(fileId string) string {
	return fmt.Sprintf(c.GetFilesRoute()+"/%v", fileId)
}

func (c *Client4) GetUploadsRoute() string {
	return "/uploads"
}

func (c *Client4) GetUploadRoute(uploadId string) string {
	return fmt.Sprintf("%s/%s", c.GetUploadsRoute(), uploadId)
}

func (c *Client4) GetPluginsRoute() string {
	return "/plugins"
}

func (c *Client4) GetPluginRoute(pluginId string) string {
	return fmt.Sprintf(c.GetPluginsRoute()+"/%v", pluginId)
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

func (c *Client4) GetComplianceReportRoute(reportId string) string {
	return fmt.Sprintf("%s/%s", c.GetComplianceReportsRoute(), reportId)
}

func (c *Client4) GetComplianceReportDownloadRoute(reportId string) string {
	return fmt.Sprintf("%s/%s/download", c.GetComplianceReportsRoute(), reportId)
}

func (c *Client4) GetOutgoingWebhooksRoute() string {
	return "/hooks/outgoing"
}

func (c *Client4) GetOutgoingWebhookRoute(hookID string) string {
	return fmt.Sprintf(c.GetOutgoingWebhooksRoute()+"/%v", hookID)
}

func (c *Client4) GetPreferencesRoute(userId string) string {
	return fmt.Sprintf(c.GetUserRoute(userId) + "/preferences")
}

func (c *Client4) GetUserStatusRoute(userId string) string {
	return fmt.Sprintf(c.GetUserRoute(userId) + "/status")
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

func (c *Client4) GetCommandRoute(commandId string) string {
	return fmt.Sprintf(c.GetCommandsRoute()+"/%v", commandId)
}

func (c *Client4) GetCommandMoveRoute(commandId string) string {
	return fmt.Sprintf(c.GetCommandsRoute()+"/%v/move", commandId)
}

func (c *Client4) GetEmojisRoute() string {
	return "/emoji"
}

func (c *Client4) GetEmojiRoute(emojiId string) string {
	return fmt.Sprintf(c.GetEmojisRoute()+"/%v", emojiId)
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

func (c *Client4) GetOAuthAppRoute(appId string) string {
	return fmt.Sprintf("/oauth/apps/%v", appId)
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

func (c *Client4) GetChannelSchemeRoute(channelId string) string {
	return fmt.Sprintf(c.GetChannelsRoute()+"/%v/scheme", channelId)
}

func (c *Client4) GetTeamSchemeRoute(teamId string) string {
	return fmt.Sprintf(c.GetTeamsRoute()+"/%v/scheme", teamId)
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

func (c *Client4) GetUserTermsOfServiceRoute(userId string) string {
	return c.GetUserRoute(userId) + "/terms_of_service"
}

func (c *Client4) GetTermsOfServiceRoute() string {
	return "/terms_of_service"
}

func (c *Client4) GetGroupsRoute() string {
	return "/groups"
}

func (c *Client4) GetPublishUserTypingRoute(userId string) string {
	return c.GetUserRoute(userId) + "/typing"
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

func (c *Client4) DoApiGet(url string, etag string) (*http.Response, *AppError) {
	return c.DoApiRequest(http.MethodGet, c.ApiUrl+url, "", etag)
}

func (c *Client4) DoApiPost(url string, data string) (*http.Response, *AppError) {
	return c.DoApiRequest(http.MethodPost, c.ApiUrl+url, data, "")
}

func (c *Client4) doApiDeleteBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doApiRequestBytes(http.MethodDelete, c.ApiUrl+url, data, "")
}

func (c *Client4) doApiPatchBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doApiRequestBytes(http.MethodPatch, c.ApiUrl+url, data, "")
}

func (c *Client4) doApiPostBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doApiRequestBytes(http.MethodPost, c.ApiUrl+url, data, "")
}

func (c *Client4) DoApiPut(url string, data string) (*http.Response, *AppError) {
	return c.DoApiRequest(http.MethodPut, c.ApiUrl+url, data, "")
}

func (c *Client4) doApiPutBytes(url string, data []byte) (*http.Response, *AppError) {
	return c.doApiRequestBytes(http.MethodPut, c.ApiUrl+url, data, "")
}

func (c *Client4) DoApiDelete(url string) (*http.Response, *AppError) {
	return c.DoApiRequest(http.MethodDelete, c.ApiUrl+url, "", "")
}

func (c *Client4) DoApiRequest(method, url, data, etag string) (*http.Response, *AppError) {
	return c.doApiRequestReader(method, url, strings.NewReader(data), map[string]string{HEADER_ETAG_CLIENT: etag})
}

func (c *Client4) DoApiRequestWithHeaders(method, url, data string, headers map[string]string) (*http.Response, *AppError) {
	return c.doApiRequestReader(method, url, strings.NewReader(data), headers)
}

func (c *Client4) doApiRequestBytes(method, url string, data []byte, etag string) (*http.Response, *AppError) {
	return c.doApiRequestReader(method, url, bytes.NewReader(data), map[string]string{HEADER_ETAG_CLIENT: etag})
}

func (c *Client4) doApiRequestReader(method, url string, data io.Reader, headers map[string]string) (*http.Response, *AppError) {
	rq, err := http.NewRequest(method, url, data)
	if err != nil {
		return nil, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	for k, v := range headers {
		rq.Header.Set(k, v)
	}

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
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
		return rp, AppErrorFromJson(rp.Body)
	}

	return rp, nil
}

func (c *Client4) DoUploadFile(url string, data []byte, contentType string) (*FileUploadResponse, *Response) {
	return c.doUploadFile(url, bytes.NewReader(data), contentType, 0)
}

func (c *Client4) doUploadFile(url string, body io.Reader, contentType string, contentLength int64) (*FileUploadResponse, *Response) {
	rq, err := http.NewRequest("POST", c.ApiUrl+url, body)
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	if contentLength != 0 {
		rq.ContentLength = contentLength
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return FileUploadResponseFromJson(rp.Body), BuildResponse(rp)
}

func (c *Client4) DoEmojiUploadFile(url string, data []byte, contentType string) (*Emoji, *Response) {
	rq, err := http.NewRequest("POST", c.ApiUrl+url, bytes.NewReader(data))
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return EmojiFromJson(rp.Body), BuildResponse(rp)
}

func (c *Client4) DoUploadImportTeam(url string, data []byte, contentType string) (map[string]string, *Response) {
	rq, err := http.NewRequest("POST", c.ApiUrl+url, bytes.NewReader(data))
	if err != nil {
		return nil, &Response{Error: NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", contentType)

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return MapFromJson(rp.Body), BuildResponse(rp)
}

// CheckStatusOK is a convenience function for checking the standard OK response
// from the web service.
func CheckStatusOK(r *http.Response) bool {
	m := MapFromJson(r.Body)
	defer closeBody(r)

	if m != nil && m[STATUS] == STATUS_OK {
		return true
	}

	return false
}

// Authentication Section

// LoginById authenticates a user by user id and password.
func (c *Client4) LoginById(id string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["id"] = id
	m["password"] = password
	return c.login(m)
}

// Login authenticates a user by login id, which can be username, email or some sort
// of SSO identifier based on server configuration, and a password.
func (c *Client4) Login(loginId string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginId
	m["password"] = password
	return c.login(m)
}

// LoginByLdap authenticates a user by LDAP id and password.
func (c *Client4) LoginByLdap(loginId string, password string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginId
	m["password"] = password
	m["ldap_only"] = c.boolString(true)
	return c.login(m)
}

// LoginWithDevice authenticates a user by login id (username, email or some sort
// of SSO identifier based on configuration), password and attaches a device id to
// the session.
func (c *Client4) LoginWithDevice(loginId string, password string, deviceId string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginId
	m["password"] = password
	m["device_id"] = deviceId
	return c.login(m)
}

// LoginWithMFA logs a user in with a MFA token
func (c *Client4) LoginWithMFA(loginId, password, mfaToken string) (*User, *Response) {
	m := make(map[string]string)
	m["login_id"] = loginId
	m["password"] = password
	m["token"] = mfaToken
	return c.login(m)
}

func (c *Client4) login(m map[string]string) (*User, *Response) {
	r, err := c.DoApiPost("/users/login", MapToJson(m))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	c.AuthToken = r.Header.Get(HEADER_TOKEN)
	c.AuthType = HEADER_BEARER
	return UserFromJson(r.Body), BuildResponse(r)
}

// Logout terminates the current user's session.
func (c *Client4) Logout() (bool, *Response) {
	r, err := c.DoApiPost("/users/logout", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	c.AuthToken = ""
	c.AuthType = HEADER_BEARER
	return CheckStatusOK(r), BuildResponse(r)
}

// SwitchAccountType changes a user's login type from one type to another.
func (c *Client4) SwitchAccountType(switchRequest *SwitchRequest) (string, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/login/switch", switchRequest.ToJson())
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["follow_link"], BuildResponse(r)
}

// User Section

// CreateUser creates a user in the system based on the provided user struct.
func (c *Client4) CreateUser(user *User) (*User, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute(), user.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// CreateUserWithToken creates a user in the system based on the provided tokenId.
func (c *Client4) CreateUserWithToken(user *User, tokenId string) (*User, *Response) {
	if tokenId == "" {
		err := NewAppError("MissingHashOrData", "api.user.create_user.missing_token.app_error", nil, "", http.StatusBadRequest)
		return nil, &Response{StatusCode: err.StatusCode, Error: err}
	}

	query := fmt.Sprintf("?t=%v", tokenId)
	r, err := c.DoApiPost(c.GetUsersRoute()+query, user.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return UserFromJson(r.Body), BuildResponse(r)
}

// CreateUserWithInviteId creates a user in the system based on the provided invited id.
func (c *Client4) CreateUserWithInviteId(user *User, inviteId string) (*User, *Response) {
	if inviteId == "" {
		err := NewAppError("MissingInviteId", "api.user.create_user.missing_invite_id.app_error", nil, "", http.StatusBadRequest)
		return nil, &Response{StatusCode: err.StatusCode, Error: err}
	}

	query := fmt.Sprintf("?iid=%v", url.QueryEscape(inviteId))
	r, err := c.DoApiPost(c.GetUsersRoute()+query, user.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return UserFromJson(r.Body), BuildResponse(r)
}

// GetMe returns the logged in user.
func (c *Client4) GetMe(etag string) (*User, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(ME), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// GetUser returns a user based on the provided user id string.
func (c *Client4) GetUser(userId, etag string) (*User, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// GetUserByUsername returns a user based on the provided user name string.
func (c *Client4) GetUserByUsername(userName, etag string) (*User, *Response) {
	r, err := c.DoApiGet(c.GetUserByUsernameRoute(userName), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// GetUserByEmail returns a user based on the provided user email string.
func (c *Client4) GetUserByEmail(email, etag string) (*User, *Response) {
	r, err := c.DoApiGet(c.GetUserByEmailRoute(email), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// AutocompleteUsersInTeam returns the users on a team based on search term.
func (c *Client4) AutocompleteUsersInTeam(teamId string, username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?in_team=%v&name=%v&limit=%d", teamId, username, limit)
	r, err := c.DoApiGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJson(r.Body), BuildResponse(r)
}

// AutocompleteUsersInChannel returns the users in a channel based on search term.
func (c *Client4) AutocompleteUsersInChannel(teamId string, channelId string, username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?in_team=%v&in_channel=%v&name=%v&limit=%d", teamId, channelId, username, limit)
	r, err := c.DoApiGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJson(r.Body), BuildResponse(r)
}

// AutocompleteUsers returns the users in the system based on search term.
func (c *Client4) AutocompleteUsers(username string, limit int, etag string) (*UserAutocomplete, *Response) {
	query := fmt.Sprintf("?name=%v&limit=%d", username, limit)
	r, err := c.DoApiGet(c.GetUsersRoute()+"/autocomplete"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAutocompleteFromJson(r.Body), BuildResponse(r)
}

// GetDefaultProfileImage gets the default user's profile image. Must be logged in.
func (c *Client4) GetDefaultProfileImage(userId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetUserRoute(userId)+"/image/default", "")
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
func (c *Client4) GetProfileImage(userId, etag string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetUserRoute(userId)+"/image", etag)
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
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetUsersInTeam(teamId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_team=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetNewUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetNewUsersInTeam(teamId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?sort=create_at&in_team=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetRecentlyActiveUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetRecentlyActiveUsersInTeam(teamId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?sort=last_activity_at&in_team=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetActiveUsersInTeam returns a page of users on a team. Page counting starts at 0.
func (c *Client4) GetActiveUsersInTeam(teamId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?active=true&in_team=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersNotInTeam returns a page of users who are not in a team. Page counting starts at 0.
func (c *Client4) GetUsersNotInTeam(teamId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?not_in_team=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersInChannel returns a page of users in a channel. Page counting starts at 0.
func (c *Client4) GetUsersInChannel(channelId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_channel=%v&page=%v&per_page=%v", channelId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersInChannelByStatus returns a page of users in a channel. Page counting starts at 0. Sorted by Status
func (c *Client4) GetUsersInChannelByStatus(channelId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_channel=%v&page=%v&per_page=%v&sort=status", channelId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersNotInChannel returns a page of users not in a channel. Page counting starts at 0.
func (c *Client4) GetUsersNotInChannel(teamId, channelId string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_team=%v&not_in_channel=%v&page=%v&per_page=%v", teamId, channelId, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersWithoutTeam returns a page of users on the system that aren't on any teams. Page counting starts at 0.
func (c *Client4) GetUsersWithoutTeam(page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?without_team=1&page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersInGroup returns a page of users in a group. Page counting starts at 0.
func (c *Client4) GetUsersInGroup(groupID string, page int, perPage int, etag string) ([]*User, *Response) {
	query := fmt.Sprintf("?in_group=%v&page=%v&per_page=%v", groupID, page, perPage)
	r, err := c.DoApiGet(c.GetUsersRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersByIds returns a list of users based on the provided user ids.
func (c *Client4) GetUsersByIds(userIds []string) ([]*User, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/ids", ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersByIds returns a list of users based on the provided user ids.
func (c *Client4) GetUsersByIdsWithOptions(userIds []string, options *UserGetByIdsOptions) ([]*User, *Response) {
	v := url.Values{}
	if options.Since != 0 {
		v.Set("since", fmt.Sprintf("%d", options.Since))
	}

	url := c.GetUsersRoute() + "/ids"
	if len(v) > 0 {
		url += "?" + v.Encode()
	}

	r, err := c.DoApiPost(url, ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersByUsernames returns a list of users based on the provided usernames.
func (c *Client4) GetUsersByUsernames(usernames []string) ([]*User, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/usernames", ArrayToJson(usernames))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// GetUsersByGroupChannelIds returns a map with channel ids as keys
// and a list of users as values based on the provided user ids.
func (c *Client4) GetUsersByGroupChannelIds(groupChannelIds []string) (map[string][]*User, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/group_channels", ArrayToJson(groupChannelIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	usersByChannelId := map[string][]*User{}
	json.NewDecoder(r.Body).Decode(&usersByChannelId)
	return usersByChannelId, BuildResponse(r)
}

// SearchUsers returns a list of users based on some search criteria.
func (c *Client4) SearchUsers(search *UserSearch) ([]*User, *Response) {
	r, err := c.doApiPostBytes(c.GetUsersRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserListFromJson(r.Body), BuildResponse(r)
}

// UpdateUser updates a user in the system based on the provided user struct.
func (c *Client4) UpdateUser(user *User) (*User, *Response) {
	r, err := c.DoApiPut(c.GetUserRoute(user.Id), user.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// PatchUser partially updates a user in the system. Any missing fields are not updated.
func (c *Client4) PatchUser(userId string, patch *UserPatch) (*User, *Response) {
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/patch", patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// UpdateUserAuth updates a user AuthData (uthData, authService and password) in the system.
func (c *Client4) UpdateUserAuth(userId string, userAuth *UserAuth) (*UserAuth, *Response) {
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/auth", userAuth.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAuthFromJson(r.Body), BuildResponse(r)
}

// UpdateUserMfa activates multi-factor authentication for a user if activate
// is true and a valid code is provided. If activate is false, then code is not
// required and multi-factor authentication is disabled for the user.
func (c *Client4) UpdateUserMfa(userId, code string, activate bool) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["activate"] = activate
	requestBody["code"] = code

	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/mfa", StringInterfaceToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// CheckUserMfa checks whether a user has MFA active on their account or not based on the
// provided login id.
// Deprecated: Clients should use Login method and check for MFA Error
func (c *Client4) CheckUserMfa(loginId string) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["login_id"] = loginId
	r, err := c.DoApiPost(c.GetUsersRoute()+"/mfa", StringInterfaceToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	data := StringInterfaceFromJson(r.Body)
	mfaRequired, ok := data["mfa_required"].(bool)
	if !ok {
		return false, BuildResponse(r)
	}
	return mfaRequired, BuildResponse(r)
}

// GenerateMfaSecret will generate a new MFA secret for a user and return it as a string and
// as a base64 encoded image QR code.
func (c *Client4) GenerateMfaSecret(userId string) (*MfaSecret, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/mfa/generate", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MfaSecretFromJson(r.Body), BuildResponse(r)
}

// UpdateUserPassword updates a user's password. Must be logged in as the user or be a system administrator.
func (c *Client4) UpdateUserPassword(userId, currentPassword, newPassword string) (bool, *Response) {
	requestBody := map[string]string{"current_password": currentPassword, "new_password": newPassword}
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/password", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserHashedPassword updates a user's password with an already-hashed password. Must be a system administrator.
func (c *Client4) UpdateUserHashedPassword(userId, newHashedPassword string) (bool, *Response) {
	requestBody := map[string]string{"already_hashed": "true", "new_password": newHashedPassword}
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/password", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PromoteGuestToUser convert a guest into a regular user
func (c *Client4) PromoteGuestToUser(guestId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(guestId)+"/promote", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DemoteUserToGuest convert a regular user into a guest
func (c *Client4) DemoteUserToGuest(guestId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(guestId)+"/demote", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserRoles updates a user's roles in the system. A user can have "system_user" and "system_admin" roles.
func (c *Client4) UpdateUserRoles(userId, roles string) (bool, *Response) {
	requestBody := map[string]string{"roles": roles}
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/roles", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateUserActive updates status of a user whether active or not.
func (c *Client4) UpdateUserActive(userId string, active bool) (bool, *Response) {
	requestBody := make(map[string]interface{})
	requestBody["active"] = active
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/active", StringInterfaceToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteUser deactivates a user in the system based on the provided user id string.
func (c *Client4) DeleteUser(userId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetUserRoute(userId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteUser deletes a user in the system based on the provided user id string.
func (c *Client4) PermanentDeleteUser(userId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetUserRoute(userId) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ConvertUserToBot converts a user to a bot user.
func (c *Client4) ConvertUserToBot(userId string) (*Bot, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/convert_to_bot", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// ConvertBotToUser converts a bot user to a user.
func (c *Client4) ConvertBotToUser(userId string, userPatch *UserPatch, setSystemAdmin bool) (*User, *Response) {
	var query string
	if setSystemAdmin {
		query = "?set_system_admin=true"
	}
	r, err := c.DoApiPost(c.GetBotRoute(userId)+"/convert_to_user"+query, userPatch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// PermanentDeleteAll permanently deletes all users in the system. This is a local only endpoint
func (c *Client4) PermanentDeleteAllUsers() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetUsersRoute())
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
	r, err := c.DoApiPost(c.GetUsersRoute()+"/password/reset/send", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ResetPassword uses a recovery code to update reset a user's password.
func (c *Client4) ResetPassword(token, newPassword string) (bool, *Response) {
	requestBody := map[string]string{"token": token, "new_password": newPassword}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/password/reset", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetSessions returns a list of sessions based on the provided user id string.
func (c *Client4) GetSessions(userId, etag string) ([]*Session, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/sessions", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SessionsFromJson(r.Body), BuildResponse(r)
}

// RevokeSession revokes a user session based on the provided user id and session id strings.
func (c *Client4) RevokeSession(userId, sessionId string) (bool, *Response) {
	requestBody := map[string]string{"session_id": sessionId}
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/sessions/revoke", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RevokeAllSessions revokes all sessions for the provided user id string.
func (c *Client4) RevokeAllSessions(userId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/sessions/revoke/all", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RevokeAllSessions revokes all sessions for all the users.
func (c *Client4) RevokeSessionsFromAllUsers() (bool, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/sessions/revoke/all", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AttachDeviceId attaches a mobile device ID to the current session.
func (c *Client4) AttachDeviceId(deviceId string) (bool, *Response) {
	requestBody := map[string]string{"device_id": deviceId}
	r, err := c.DoApiPut(c.GetUsersRoute()+"/sessions/device", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamsUnreadForUser will return an array with TeamUnread objects that contain the amount
// of unread messages and mentions the current user has for the teams it belongs to.
// An optional team ID can be set to exclude that team from the results. Must be authenticated.
func (c *Client4) GetTeamsUnreadForUser(userId, teamIdToExclude string) ([]*TeamUnread, *Response) {
	var optional string
	if teamIdToExclude != "" {
		optional += fmt.Sprintf("?exclude_team=%s", url.QueryEscape(teamIdToExclude))
	}

	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/teams/unread"+optional, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamsUnreadFromJson(r.Body), BuildResponse(r)
}

// GetUserAudits returns a list of audit based on the provided user id string.
func (c *Client4) GetUserAudits(userId string, page int, perPage int, etag string) (Audits, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/audits"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AuditsFromJson(r.Body), BuildResponse(r)
}

// VerifyUserEmail will verify a user's email using the supplied token.
func (c *Client4) VerifyUserEmail(token string) (bool, *Response) {
	requestBody := map[string]string{"token": token}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/email/verify", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// VerifyUserEmailWithoutToken will verify a user's email by its Id. (Requires manage system role)
func (c *Client4) VerifyUserEmailWithoutToken(userId string) (*User, *Response) {
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/email/verify/member", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserFromJson(r.Body), BuildResponse(r)
}

// SendVerificationEmail will send an email to the user with the provided email address, if
// that user exists. The email will contain a link that can be used to verify the user's
// email address.
func (c *Client4) SendVerificationEmail(email string) (bool, *Response) {
	requestBody := map[string]string{"email": email}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/email/verify/send", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SetDefaultProfileImage resets the profile image to a default generated one.
func (c *Client4) SetDefaultProfileImage(userId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetUserRoute(userId) + "/image")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	return CheckStatusOK(r), BuildResponse(r)
}

// SetProfileImage sets profile image of the user.
func (c *Client4) SetProfileImage(userId string, data []byte) (bool, *Response) {
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetUserRoute(userId)+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetProfileImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetUserRoute(userId)+"/image", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// CreateUserAccessToken will generate a user access token that can be used in place
// of a session token to access the REST API. Must have the 'create_user_access_token'
// permission and if generating for another user, must have the 'edit_other_users'
// permission. A non-blank description is required.
func (c *Client4) CreateUserAccessToken(userId, description string) (*UserAccessToken, *Response) {
	requestBody := map[string]string{"description": description}
	r, err := c.DoApiPost(c.GetUserRoute(userId)+"/tokens", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenFromJson(r.Body), BuildResponse(r)
}

// GetUserAccessTokens will get a page of access tokens' id, description, is_active
// and the user_id in the system. The actual token will not be returned. Must have
// the 'manage_system' permission.
func (c *Client4) GetUserAccessTokens(page int, perPage int) ([]*UserAccessToken, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUserAccessTokensRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJson(r.Body), BuildResponse(r)
}

// GetUserAccessToken will get a user access tokens' id, description, is_active
// and the user_id of the user it is for. The actual token will not be returned.
// Must have the 'read_user_access_token' permission and if getting for another
// user, must have the 'edit_other_users' permission.
func (c *Client4) GetUserAccessToken(tokenId string) (*UserAccessToken, *Response) {
	r, err := c.DoApiGet(c.GetUserAccessTokenRoute(tokenId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenFromJson(r.Body), BuildResponse(r)
}

// GetUserAccessTokensForUser will get a paged list of user access tokens showing id,
// description and user_id for each. The actual tokens will not be returned. Must have
// the 'read_user_access_token' permission and if getting for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) GetUserAccessTokensForUser(userId string, page, perPage int) ([]*UserAccessToken, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/tokens"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJson(r.Body), BuildResponse(r)
}

// RevokeUserAccessToken will revoke a user access token by id. Must have the
// 'revoke_user_access_token' permission and if revoking for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) RevokeUserAccessToken(tokenId string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenId}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/tokens/revoke", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// SearchUserAccessTokens returns user access tokens matching the provided search term.
func (c *Client4) SearchUserAccessTokens(search *UserAccessTokenSearch) ([]*UserAccessToken, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/tokens/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserAccessTokenListFromJson(r.Body), BuildResponse(r)
}

// DisableUserAccessToken will disable a user access token by id. Must have the
// 'revoke_user_access_token' permission and if disabling for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) DisableUserAccessToken(tokenId string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenId}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/tokens/disable", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// EnableUserAccessToken will enable a user access token by id. Must have the
// 'create_user_access_token' permission and if enabling for another user, must have the
// 'edit_other_users' permission.
func (c *Client4) EnableUserAccessToken(tokenId string) (bool, *Response) {
	requestBody := map[string]string{"token_id": tokenId}
	r, err := c.DoApiPost(c.GetUsersRoute()+"/tokens/enable", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Bots section

// CreateBot creates a bot in the system based on the provided bot struct.
func (c *Client4) CreateBot(bot *Bot) (*Bot, *Response) {
	r, err := c.doApiPostBytes(c.GetBotsRoute(), bot.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// PatchBot partially updates a bot. Any missing fields are not updated.
func (c *Client4) PatchBot(userId string, patch *BotPatch) (*Bot, *Response) {
	r, err := c.doApiPutBytes(c.GetBotRoute(userId), patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// GetBot fetches the given, undeleted bot.
func (c *Client4) GetBot(userId string, etag string) (*Bot, *Response) {
	r, err := c.DoApiGet(c.GetBotRoute(userId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// GetBot fetches the given bot, even if it is deleted.
func (c *Client4) GetBotIncludeDeleted(userId string, etag string) (*Bot, *Response) {
	r, err := c.DoApiGet(c.GetBotRoute(userId)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// GetBots fetches the given page of bots, excluding deleted.
func (c *Client4) GetBots(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJson(r.Body), BuildResponse(r)
}

// GetBotsIncludeDeleted fetches the given page of bots, including deleted.
func (c *Client4) GetBotsIncludeDeleted(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_deleted="+c.boolString(true), page, perPage)
	r, err := c.DoApiGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJson(r.Body), BuildResponse(r)
}

// GetBotsOrphaned fetches the given page of bots, only including orphanded bots.
func (c *Client4) GetBotsOrphaned(page, perPage int, etag string) ([]*Bot, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&only_orphaned="+c.boolString(true), page, perPage)
	r, err := c.DoApiGet(c.GetBotsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotListFromJson(r.Body), BuildResponse(r)
}

// DisableBot disables the given bot in the system.
func (c *Client4) DisableBot(botUserId string) (*Bot, *Response) {
	r, err := c.doApiPostBytes(c.GetBotRoute(botUserId)+"/disable", nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// EnableBot disables the given bot in the system.
func (c *Client4) EnableBot(botUserId string) (*Bot, *Response) {
	r, err := c.doApiPostBytes(c.GetBotRoute(botUserId)+"/enable", nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// AssignBot assigns the given bot to the given user
func (c *Client4) AssignBot(botUserId, newOwnerId string) (*Bot, *Response) {
	r, err := c.doApiPostBytes(c.GetBotRoute(botUserId)+"/assign/"+newOwnerId, nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BotFromJson(r.Body), BuildResponse(r)
}

// SetBotIconImage sets LHS bot icon image.
func (c *Client4) SetBotIconImage(botUserId string, data []byte) (bool, *Response) {
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetBotRoute(botUserId)+"/icon", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetBotIconImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetBotRoute(botUserId)+"/icon", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// GetBotIconImage gets LHS bot icon image. Must be logged in.
func (c *Client4) GetBotIconImage(botUserId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetBotRoute(botUserId)+"/icon", "")
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
func (c *Client4) DeleteBotIconImage(botUserId string) (bool, *Response) {
	r, appErr := c.DoApiDelete(c.GetBotRoute(botUserId) + "/icon")
	if appErr != nil {
		return false, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Team Section

// CreateTeam creates a team in the system based on the provided team struct.
func (c *Client4) CreateTeam(team *Team) (*Team, *Response) {
	r, err := c.DoApiPost(c.GetTeamsRoute(), team.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// GetTeam returns a team based on the provided team id string.
func (c *Client4) GetTeam(teamId, etag string) (*Team, *Response) {
	r, err := c.DoApiGet(c.GetTeamRoute(teamId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// GetAllTeams returns all teams based on permissions.
func (c *Client4) GetAllTeams(etag string, page int, perPage int) ([]*Team, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJson(r.Body), BuildResponse(r)
}

// GetAllTeamsWithTotalCount returns all teams based on permissions.
func (c *Client4) GetAllTeamsWithTotalCount(etag string, page int, perPage int) ([]*Team, int64, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_total_count="+c.boolString(true), page, perPage)
	r, err := c.DoApiGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	teamsListWithCount := TeamsWithCountFromJson(r.Body)
	return teamsListWithCount.Teams, teamsListWithCount.TotalCount, BuildResponse(r)
}

// GetAllTeamsExcludePolicyConstrained returns all teams which are not part of a data retention policy.
// Must be a system administrator.
func (c *Client4) GetAllTeamsExcludePolicyConstrained(etag string, page int, perPage int) ([]*Team, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&exclude_policy_constrained=%v", page, perPage, true)
	r, err := c.DoApiGet(c.GetTeamsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJson(r.Body), BuildResponse(r)
}

// GetTeamByName returns a team based on the provided team name string.
func (c *Client4) GetTeamByName(name, etag string) (*Team, *Response) {
	r, err := c.DoApiGet(c.GetTeamByNameRoute(name), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// SearchTeams returns teams matching the provided search term.
func (c *Client4) SearchTeams(search *TeamSearch) ([]*Team, *Response) {
	r, err := c.DoApiPost(c.GetTeamsRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJson(r.Body), BuildResponse(r)
}

// SearchTeamsPaged returns a page of teams and the total count matching the provided search term.
func (c *Client4) SearchTeamsPaged(search *TeamSearch) ([]*Team, int64, *Response) {
	if search.Page == nil {
		search.Page = NewInt(0)
	}
	if search.PerPage == nil {
		search.PerPage = NewInt(100)
	}
	r, err := c.DoApiPost(c.GetTeamsRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	twc := TeamsWithCountFromJson(r.Body)
	return twc.Teams, twc.TotalCount, BuildResponse(r)
}

// TeamExists returns true or false if the team exist or not.
func (c *Client4) TeamExists(name, etag string) (bool, *Response) {
	r, err := c.DoApiGet(c.GetTeamByNameRoute(name)+"/exists", etag)
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapBoolFromJson(r.Body)["exists"], BuildResponse(r)
}

// GetTeamsForUser returns a list of teams a user is on. Must be logged in as the user
// or be a system administrator.
func (c *Client4) GetTeamsForUser(userId, etag string) ([]*Team, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/teams", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJson(r.Body), BuildResponse(r)
}

// GetTeamMember returns a team member based on the provided team and user id strings.
func (c *Client4) GetTeamMember(teamId, userId, etag string) (*TeamMember, *Response) {
	r, err := c.DoApiGet(c.GetTeamMemberRoute(teamId, userId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJson(r.Body), BuildResponse(r)
}

// UpdateTeamMemberRoles will update the roles on a team for a user.
func (c *Client4) UpdateTeamMemberRoles(teamId, userId, newRoles string) (bool, *Response) {
	requestBody := map[string]string{"roles": newRoles}
	r, err := c.DoApiPut(c.GetTeamMemberRoute(teamId, userId)+"/roles", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamMemberSchemeRoles will update the scheme-derived roles on a team for a user.
func (c *Client4) UpdateTeamMemberSchemeRoles(teamId string, userId string, schemeRoles *SchemeRoles) (bool, *Response) {
	r, err := c.DoApiPut(c.GetTeamMemberRoute(teamId, userId)+"/schemeRoles", schemeRoles.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeam will update a team.
func (c *Client4) UpdateTeam(team *Team) (*Team, *Response) {
	r, err := c.DoApiPut(c.GetTeamRoute(team.Id), team.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// PatchTeam partially updates a team. Any missing fields are not updated.
func (c *Client4) PatchTeam(teamId string, patch *TeamPatch) (*Team, *Response) {
	r, err := c.DoApiPut(c.GetTeamRoute(teamId)+"/patch", patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// RestoreTeam restores a previously deleted team.
func (c *Client4) RestoreTeam(teamId string) (*Team, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/restore", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// RegenerateTeamInviteId requests a new invite ID to be generated.
func (c *Client4) RegenerateTeamInviteId(teamId string) (*Team, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/regenerate_invite_id", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// SoftDeleteTeam deletes the team softly (archive only, not permanent delete).
func (c *Client4) SoftDeleteTeam(teamId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetTeamRoute(teamId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteTeam deletes the team, should only be used when needed for
// compliance and the like.
func (c *Client4) PermanentDeleteTeam(teamId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetTeamRoute(teamId) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamPrivacy modifies the team type (model.TEAM_OPEN <--> model.TEAM_INVITE) and sets
// the corresponding AllowOpenInvite appropriately.
func (c *Client4) UpdateTeamPrivacy(teamId string, privacy string) (*Team, *Response) {
	requestBody := map[string]string{"privacy": privacy}
	r, err := c.DoApiPut(c.GetTeamRoute(teamId)+"/privacy", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// GetTeamMembers returns team members based on the provided team id string.
func (c *Client4) GetTeamMembers(teamId string, page int, perPage int, etag string) ([]*TeamMember, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetTeamMembersRoute(teamId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJson(r.Body), BuildResponse(r)
}

// GetTeamMembersWithoutDeletedUsers returns team members based on the provided team id string. Additional parameters of sort and exclude_deleted_users accepted as well
// Could not add it to above function due to it be a breaking change.
func (c *Client4) GetTeamMembersSortAndWithoutDeletedUsers(teamId string, page int, perPage int, sort string, exclude_deleted_users bool, etag string) ([]*TeamMember, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&sort=%v&exclude_deleted_users=%v", page, perPage, sort, exclude_deleted_users)
	r, err := c.DoApiGet(c.GetTeamMembersRoute(teamId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJson(r.Body), BuildResponse(r)
}

// GetTeamMembersForUser returns the team members for a user.
func (c *Client4) GetTeamMembersForUser(userId string, etag string) ([]*TeamMember, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/teams/members", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJson(r.Body), BuildResponse(r)
}

// GetTeamMembersByIds will return an array of team members based on the
// team id and a list of user ids provided. Must be authenticated.
func (c *Client4) GetTeamMembersByIds(teamId string, userIds []string) ([]*TeamMember, *Response) {
	r, err := c.DoApiPost(fmt.Sprintf("/teams/%v/members/ids", teamId), ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJson(r.Body), BuildResponse(r)
}

// AddTeamMember adds user to a team and return a team member.
func (c *Client4) AddTeamMember(teamId, userId string) (*TeamMember, *Response) {
	member := &TeamMember{TeamId: teamId, UserId: userId}
	r, err := c.DoApiPost(c.GetTeamMembersRoute(teamId), member.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJson(r.Body), BuildResponse(r)
}

// AddTeamMemberFromInvite adds a user to a team and return a team member using an invite id
// or an invite token/data pair.
func (c *Client4) AddTeamMemberFromInvite(token, inviteId string) (*TeamMember, *Response) {
	var query string

	if inviteId != "" {
		query += fmt.Sprintf("?invite_id=%v", inviteId)
	}

	if token != "" {
		query += fmt.Sprintf("?token=%v", token)
	}

	r, err := c.DoApiPost(c.GetTeamsRoute()+"/members/invite"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMemberFromJson(r.Body), BuildResponse(r)
}

// AddTeamMembers adds a number of users to a team and returns the team members.
func (c *Client4) AddTeamMembers(teamId string, userIds []string) ([]*TeamMember, *Response) {
	var members []*TeamMember
	for _, userId := range userIds {
		member := &TeamMember{TeamId: teamId, UserId: userId}
		members = append(members, member)
	}

	r, err := c.DoApiPost(c.GetTeamMembersRoute(teamId)+"/batch", TeamMembersToJson(members))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersFromJson(r.Body), BuildResponse(r)
}

// AddTeamMembers adds a number of users to a team and returns the team members.
func (c *Client4) AddTeamMembersGracefully(teamId string, userIds []string) ([]*TeamMemberWithError, *Response) {
	var members []*TeamMember
	for _, userId := range userIds {
		member := &TeamMember{TeamId: teamId, UserId: userId}
		members = append(members, member)
	}

	r, err := c.DoApiPost(c.GetTeamMembersRoute(teamId)+"/batch?graceful="+c.boolString(true), TeamMembersToJson(members))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamMembersWithErrorFromJson(r.Body), BuildResponse(r)
}

// RemoveTeamMember will remove a user from a team.
func (c *Client4) RemoveTeamMember(teamId, userId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetTeamMemberRoute(teamId, userId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamStats returns a team stats based on the team id string.
// Must be authenticated.
func (c *Client4) GetTeamStats(teamId, etag string) (*TeamStats, *Response) {
	r, err := c.DoApiGet(c.GetTeamStatsRoute(teamId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamStatsFromJson(r.Body), BuildResponse(r)
}

// GetTotalUsersStats returns a total system user stats.
// Must be authenticated.
func (c *Client4) GetTotalUsersStats(etag string) (*UsersStats, *Response) {
	r, err := c.DoApiGet(c.GetTotalUsersStatsRoute(), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UsersStatsFromJson(r.Body), BuildResponse(r)
}

// GetTeamUnread will return a TeamUnread object that contains the amount of
// unread messages and mentions the user has for the specified team.
// Must be authenticated.
func (c *Client4) GetTeamUnread(teamId, userId string) (*TeamUnread, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+c.GetTeamRoute(teamId)+"/unread", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamUnreadFromJson(r.Body), BuildResponse(r)
}

// ImportTeam will import an exported team from other app into a existing team.
func (c *Client4) ImportTeam(data []byte, filesize int, importFrom, filename, teamId string) (map[string]string, *Response) {
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

	return c.DoUploadImportTeam(c.GetTeamImportRoute(teamId), body.Bytes(), writer.FormDataContentType())
}

// InviteUsersToTeam invite users by email to the team.
func (c *Client4) InviteUsersToTeam(teamId string, userEmails []string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/invite/email", ArrayToJson(userEmails))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InviteGuestsToTeam invite guest by email to some channels in a team.
func (c *Client4) InviteGuestsToTeam(teamId string, userEmails []string, channels []string, message string) (bool, *Response) {
	guestsInvite := GuestsInvite{
		Emails:   userEmails,
		Channels: channels,
		Message:  message,
	}
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/invite-guests/email", guestsInvite.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InviteUsersToTeam invite users by email to the team.
func (c *Client4) InviteUsersToTeamGracefully(teamId string, userEmails []string) ([]*EmailInviteWithError, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/invite/email?graceful="+c.boolString(true), ArrayToJson(userEmails))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmailInviteWithErrorFromJson(r.Body), BuildResponse(r)
}

// InviteGuestsToTeam invite guest by email to some channels in a team.
func (c *Client4) InviteGuestsToTeamGracefully(teamId string, userEmails []string, channels []string, message string) ([]*EmailInviteWithError, *Response) {
	guestsInvite := GuestsInvite{
		Emails:   userEmails,
		Channels: channels,
		Message:  message,
	}
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/invite-guests/email?graceful="+c.boolString(true), guestsInvite.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmailInviteWithErrorFromJson(r.Body), BuildResponse(r)
}

// InvalidateEmailInvites will invalidate active email invitations that have not been accepted by the user.
func (c *Client4) InvalidateEmailInvites() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetTeamsRoute() + "/invites/email")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetTeamInviteInfo returns a team object from an invite id containing sanitized information.
func (c *Client4) GetTeamInviteInfo(inviteId string) (*Team, *Response) {
	r, err := c.DoApiGet(c.GetTeamsRoute()+"/invite/"+inviteId, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamFromJson(r.Body), BuildResponse(r)
}

// SetTeamIcon sets team icon of the team.
func (c *Client4) SetTeamIcon(teamId string, data []byte) (bool, *Response) {
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetTeamRoute(teamId)+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("SetTeamIcon", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		// set to http.StatusForbidden(403)
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetTeamRoute(teamId)+"/image", "model.client.connecting.app_error", nil, err.Error(), 403)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// GetTeamIcon gets the team icon of the team.
func (c *Client4) GetTeamIcon(teamId, etag string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetTeamRoute(teamId)+"/image", etag)
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
func (c *Client4) RemoveTeamIcon(teamId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetTeamRoute(teamId) + "/image")
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
	r, err := c.DoApiGet(c.GetChannelsRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListWithTeamDataFromJson(r.Body), BuildResponse(r)
}

// GetAllChannelsWithCount get all the channels including the total count. Must be a system administrator.
func (c *Client4) GetAllChannelsWithCount(page int, perPage int, etag string) (*ChannelListWithTeamData, int64, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&include_total_count="+c.boolString(true), page, perPage)
	r, err := c.DoApiGet(c.GetChannelsRoute()+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	cwc := ChannelsWithCountFromJson(r.Body)
	return cwc.Channels, cwc.TotalCount, BuildResponse(r)
}

// CreateChannel creates a channel based on the provided channel struct.
func (c *Client4) CreateChannel(channel *Channel) (*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsRoute(), channel.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// UpdateChannel updates a channel based on the provided channel struct.
func (c *Client4) UpdateChannel(channel *Channel) (*Channel, *Response) {
	r, err := c.DoApiPut(c.GetChannelRoute(channel.Id), channel.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// PatchChannel partially updates a channel. Any missing fields are not updated.
func (c *Client4) PatchChannel(channelId string, patch *ChannelPatch) (*Channel, *Response) {
	r, err := c.DoApiPut(c.GetChannelRoute(channelId)+"/patch", patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// ConvertChannelToPrivate converts public to private channel.
func (c *Client4) ConvertChannelToPrivate(channelId string) (*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelRoute(channelId)+"/convert", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// UpdateChannelPrivacy updates channel privacy
func (c *Client4) UpdateChannelPrivacy(channelId string, privacy string) (*Channel, *Response) {
	requestBody := map[string]string{"privacy": privacy}
	r, err := c.DoApiPut(c.GetChannelRoute(channelId)+"/privacy", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// RestoreChannel restores a previously deleted channel. Any missing fields are not updated.
func (c *Client4) RestoreChannel(channelId string) (*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelRoute(channelId)+"/restore", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// CreateDirectChannel creates a direct message channel based on the two user
// ids provided.
func (c *Client4) CreateDirectChannel(userId1, userId2 string) (*Channel, *Response) {
	requestBody := []string{userId1, userId2}
	r, err := c.DoApiPost(c.GetChannelsRoute()+"/direct", ArrayToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// CreateGroupChannel creates a group message channel based on userIds provided.
func (c *Client4) CreateGroupChannel(userIds []string) (*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsRoute()+"/group", ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannel returns a channel based on the provided channel id string.
func (c *Client4) GetChannel(channelId, etag string) (*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelStats returns statistics for a channel.
func (c *Client4) GetChannelStats(channelId string, etag string) (*ChannelStats, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/stats", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelStatsFromJson(r.Body), BuildResponse(r)
}

// GetChannelMembersTimezones gets a list of timezones for a channel.
func (c *Client4) GetChannelMembersTimezones(channelId string) ([]string, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/timezones", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

// GetPinnedPosts gets a list of pinned posts.
func (c *Client4) GetPinnedPosts(channelId string, etag string) (*PostList, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/pinned", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPrivateChannelsForTeam returns a list of private channels based on the provided team id string.
func (c *Client4) GetPrivateChannelsForTeam(teamId string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("/private?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetChannelsForTeamRoute(teamId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// GetPublicChannelsForTeam returns a list of public channels based on the provided team id string.
func (c *Client4) GetPublicChannelsForTeam(teamId string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetChannelsForTeamRoute(teamId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// GetDeletedChannelsForTeam returns a list of public channels based on the provided team id string.
func (c *Client4) GetDeletedChannelsForTeam(teamId string, page int, perPage int, etag string) ([]*Channel, *Response) {
	query := fmt.Sprintf("/deleted?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetChannelsForTeamRoute(teamId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// GetPublicChannelsByIdsForTeam returns a list of public channels based on provided team id string.
func (c *Client4) GetPublicChannelsByIdsForTeam(teamId string, channelIds []string) ([]*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsForTeamRoute(teamId)+"/ids", ArrayToJson(channelIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// GetChannelsForTeamForUser returns a list channels of on a team for a user.
func (c *Client4) GetChannelsForTeamForUser(teamId, userId string, includeDeleted bool, etag string) ([]*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelsForTeamForUserRoute(teamId, userId, includeDeleted), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// GetChannelsForTeamAndUserWithLastDeleteAt returns a list channels of a team for a user, additionally filtered with lastDeleteAt. This does not have any effect if includeDeleted is set to false.
func (c *Client4) GetChannelsForTeamAndUserWithLastDeleteAt(teamId, userId string, includeDeleted bool, lastDeleteAt int, etag string) ([]*Channel, *Response) {
	route := fmt.Sprintf(c.GetUserRoute(userId) + c.GetTeamRoute(teamId) + "/channels")
	route += fmt.Sprintf("?include_deleted=%v&last_delete_at=%d", includeDeleted, lastDeleteAt)
	r, err := c.DoApiGet(route, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// SearchChannels returns the channels on a team matching the provided search term.
func (c *Client4) SearchChannels(teamId string, search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsForTeamRoute(teamId)+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// SearchArchivedChannels returns the archived channels on a team matching the provided search term.
func (c *Client4) SearchArchivedChannels(teamId string, search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsForTeamRoute(teamId)+"/search_archived", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// SearchAllChannels search in all the channels. Must be a system administrator.
func (c *Client4) SearchAllChannels(search *ChannelSearch) (*ChannelListWithTeamData, *Response) {
	r, err := c.DoApiPost(c.GetChannelsRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListWithTeamDataFromJson(r.Body), BuildResponse(r)
}

// SearchAllChannelsPaged searches all the channels and returns the results paged with the total count.
func (c *Client4) SearchAllChannelsPaged(search *ChannelSearch) (*ChannelsWithCount, *Response) {
	r, err := c.DoApiPost(c.GetChannelsRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelsWithCountFromJson(r.Body), BuildResponse(r)
}

// SearchGroupChannels returns the group channels of the user whose members' usernames match the search term.
func (c *Client4) SearchGroupChannels(search *ChannelSearch) ([]*Channel, *Response) {
	r, err := c.DoApiPost(c.GetChannelsRoute()+"/group/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelSliceFromJson(r.Body), BuildResponse(r)
}

// DeleteChannel deletes channel based on the provided channel id string.
func (c *Client4) DeleteChannel(channelId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetChannelRoute(channelId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PermanentDeleteChannel deletes a channel based on the provided channel id string.
func (c *Client4) PermanentDeleteChannel(channelId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetChannelRoute(channelId) + "?permanent=" + c.boolString(true))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// MoveChannel moves the channel to the destination team.
func (c *Client4) MoveChannel(channelId, teamId string, force bool) (*Channel, *Response) {
	requestBody := map[string]interface{}{
		"team_id": teamId,
		"force":   force,
	}
	r, err := c.DoApiPost(c.GetChannelRoute(channelId)+"/move", StringInterfaceToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelByName returns a channel based on the provided channel name and team id strings.
func (c *Client4) GetChannelByName(channelName, teamId string, etag string) (*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelByNameRoute(channelName, teamId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelByNameIncludeDeleted returns a channel based on the provided channel name and team id strings. Other then GetChannelByName it will also return deleted channels.
func (c *Client4) GetChannelByNameIncludeDeleted(channelName, teamId string, etag string) (*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelByNameRoute(channelName, teamId)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelByNameForTeamName returns a channel based on the provided channel name and team name strings.
func (c *Client4) GetChannelByNameForTeamName(channelName, teamName string, etag string) (*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelByNameForTeamNameRoute(channelName, teamName), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelByNameForTeamNameIncludeDeleted returns a channel based on the provided channel name and team name strings. Other then GetChannelByNameForTeamName it will also return deleted channels.
func (c *Client4) GetChannelByNameForTeamNameIncludeDeleted(channelName, teamName string, etag string) (*Channel, *Response) {
	r, err := c.DoApiGet(c.GetChannelByNameForTeamNameRoute(channelName, teamName)+"?include_deleted="+c.boolString(true), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelFromJson(r.Body), BuildResponse(r)
}

// GetChannelMembers gets a page of channel members.
func (c *Client4) GetChannelMembers(channelId string, page, perPage int, etag string) (*ChannelMembers, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetChannelMembersRoute(channelId)+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJson(r.Body), BuildResponse(r)
}

// GetChannelMembersByIds gets the channel members in a channel for a list of user ids.
func (c *Client4) GetChannelMembersByIds(channelId string, userIds []string) (*ChannelMembers, *Response) {
	r, err := c.DoApiPost(c.GetChannelMembersRoute(channelId)+"/ids", ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJson(r.Body), BuildResponse(r)
}

// GetChannelMember gets a channel member.
func (c *Client4) GetChannelMember(channelId, userId, etag string) (*ChannelMember, *Response) {
	r, err := c.DoApiGet(c.GetChannelMemberRoute(channelId, userId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJson(r.Body), BuildResponse(r)
}

// GetChannelMembersForUser gets all the channel members for a user on a team.
func (c *Client4) GetChannelMembersForUser(userId, teamId, etag string) (*ChannelMembers, *Response) {
	r, err := c.DoApiGet(fmt.Sprintf(c.GetUserRoute(userId)+"/teams/%v/channels/members", teamId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMembersFromJson(r.Body), BuildResponse(r)
}

// ViewChannel performs a view action for a user. Synonymous with switching channels or marking channels as read by a user.
func (c *Client4) ViewChannel(userId string, view *ChannelView) (*ChannelViewResponse, *Response) {
	url := fmt.Sprintf(c.GetChannelsRoute()+"/members/%v/view", userId)
	r, err := c.DoApiPost(url, view.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelViewResponseFromJson(r.Body), BuildResponse(r)
}

// GetChannelUnread will return a ChannelUnread object that contains the number of
// unread messages and mentions for a user.
func (c *Client4) GetChannelUnread(channelId, userId string) (*ChannelUnread, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+c.GetChannelRoute(channelId)+"/unread", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelUnreadFromJson(r.Body), BuildResponse(r)
}

// UpdateChannelRoles will update the roles on a channel for a user.
func (c *Client4) UpdateChannelRoles(channelId, userId, roles string) (bool, *Response) {
	requestBody := map[string]string{"roles": roles}
	r, err := c.DoApiPut(c.GetChannelMemberRoute(channelId, userId)+"/roles", MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateChannelMemberSchemeRoles will update the scheme-derived roles on a channel for a user.
func (c *Client4) UpdateChannelMemberSchemeRoles(channelId string, userId string, schemeRoles *SchemeRoles) (bool, *Response) {
	r, err := c.DoApiPut(c.GetChannelMemberRoute(channelId, userId)+"/schemeRoles", schemeRoles.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateChannelNotifyProps will update the notification properties on a channel for a user.
func (c *Client4) UpdateChannelNotifyProps(channelId, userId string, props map[string]string) (bool, *Response) {
	r, err := c.DoApiPut(c.GetChannelMemberRoute(channelId, userId)+"/notify_props", MapToJson(props))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AddChannelMember adds user to channel and return a channel member.
func (c *Client4) AddChannelMember(channelId, userId string) (*ChannelMember, *Response) {
	requestBody := map[string]string{"user_id": userId}
	r, err := c.DoApiPost(c.GetChannelMembersRoute(channelId)+"", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJson(r.Body), BuildResponse(r)
}

// AddChannelMemberWithRootId adds user to channel and return a channel member. Post add to channel message has the postRootId.
func (c *Client4) AddChannelMemberWithRootId(channelId, userId, postRootId string) (*ChannelMember, *Response) {
	requestBody := map[string]string{"user_id": userId, "post_root_id": postRootId}
	r, err := c.DoApiPost(c.GetChannelMembersRoute(channelId)+"", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberFromJson(r.Body), BuildResponse(r)
}

// RemoveUserFromChannel will delete the channel member object for a user, effectively removing the user from a channel.
func (c *Client4) RemoveUserFromChannel(channelId, userId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetChannelMemberRoute(channelId, userId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// AutocompleteChannelsForTeam will return an ordered list of channels autocomplete suggestions.
func (c *Client4) AutocompleteChannelsForTeam(teamId, name string) (*ChannelList, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoApiGet(c.GetChannelsForTeamRoute(teamId)+"/autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListFromJson(r.Body), BuildResponse(r)
}

// AutocompleteChannelsForTeamForSearch will return an ordered list of your channels autocomplete suggestions.
func (c *Client4) AutocompleteChannelsForTeamForSearch(teamId, name string) (*ChannelList, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoApiGet(c.GetChannelsForTeamRoute(teamId)+"/search_autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelListFromJson(r.Body), BuildResponse(r)
}

// Post Section

// CreatePost creates a post based on the provided post struct.
func (c *Client4) CreatePost(post *Post) (*Post, *Response) {
	r, err := c.DoApiPost(c.GetPostsRoute(), post.ToUnsanitizedJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJson(r.Body), BuildResponse(r)
}

// CreatePostEphemeral creates a ephemeral post based on the provided post struct which is send to the given user id.
func (c *Client4) CreatePostEphemeral(post *PostEphemeral) (*Post, *Response) {
	r, err := c.DoApiPost(c.GetPostsEphemeralRoute(), post.ToUnsanitizedJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJson(r.Body), BuildResponse(r)
}

// UpdatePost updates a post based on the provided post struct.
func (c *Client4) UpdatePost(postId string, post *Post) (*Post, *Response) {
	r, err := c.DoApiPut(c.GetPostRoute(postId), post.ToUnsanitizedJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJson(r.Body), BuildResponse(r)
}

// PatchPost partially updates a post. Any missing fields are not updated.
func (c *Client4) PatchPost(postId string, patch *PostPatch) (*Post, *Response) {
	r, err := c.DoApiPut(c.GetPostRoute(postId)+"/patch", patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJson(r.Body), BuildResponse(r)
}

// SetPostUnread marks channel where post belongs as unread on the time of the provided post.
func (c *Client4) SetPostUnread(userId string, postId string, collapsedThreadsSupported bool) *Response {
	b, _ := json.Marshal(map[string]bool{"collapsed_threads_supported": collapsedThreadsSupported})
	r, err := c.DoApiPost(c.GetUserRoute(userId)+c.GetPostRoute(postId)+"/set_unread", string(b))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// PinPost pin a post based on provided post id string.
func (c *Client4) PinPost(postId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPostRoute(postId)+"/pin", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UnpinPost unpin a post based on provided post id string.
func (c *Client4) UnpinPost(postId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPostRoute(postId)+"/unpin", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetPost gets a single post.
func (c *Client4) GetPost(postId string, etag string) (*Post, *Response) {
	r, err := c.DoApiGet(c.GetPostRoute(postId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostFromJson(r.Body), BuildResponse(r)
}

// GetPosts gets multiple posts by ID.
func (c *Client4) GetPosts(postIDs []string) ([]*Post, *Response) {
	jsonIDs, _ := json.Marshal(postIDs)
	r, err := c.DoApiPost(c.GetPostsByIDsRoute(), string(jsonIDs))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	var posts []*Post
	json.NewDecoder(r.Body).Decode(&posts)
	return posts, BuildResponse(r)
}

// DeletePost deletes a post from the provided post id string.
func (c *Client4) DeletePost(postId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetPostRoute(postId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetPostThread gets a post with all the other posts in the same thread.
func (c *Client4) GetPostThread(postId string, etag string, collapsedThreads bool) (*PostList, *Response) {
	url := c.GetPostRoute(postId) + "/thread"
	if collapsedThreads {
		url += "?collapsedThreads=true"
	}
	r, err := c.DoApiGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPostsForChannel gets a page of posts with an array for ordering for a channel.
func (c *Client4) GetPostsForChannel(channelId string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUser returns flagged posts of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUser(userId string, page int, perPage int) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUserInTeam returns flagged posts in team of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUserInTeam(userId string, teamId string, page int, perPage int) (*PostList, *Response) {
	if !IsValidId(teamId) {
		return nil, &Response{StatusCode: http.StatusBadRequest, Error: NewAppError("GetFlaggedPostsForUserInTeam", "model.client.get_flagged_posts_in_team.missing_parameter.app_error", nil, "", http.StatusBadRequest)}
	}

	query := fmt.Sprintf("?team_id=%v&page=%v&per_page=%v", teamId, page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetFlaggedPostsForUserInChannel returns flagged posts in channel of a user based on user id string.
func (c *Client4) GetFlaggedPostsForUserInChannel(userId string, channelId string, page int, perPage int) (*PostList, *Response) {
	if !IsValidId(channelId) {
		return nil, &Response{StatusCode: http.StatusBadRequest, Error: NewAppError("GetFlaggedPostsForUserInChannel", "model.client.get_flagged_posts_in_channel.missing_parameter.app_error", nil, "", http.StatusBadRequest)}
	}

	query := fmt.Sprintf("?channel_id=%v&page=%v&per_page=%v", channelId, page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/posts/flagged"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPostsSince gets posts created after a specified time as Unix time in milliseconds.
func (c *Client4) GetPostsSince(channelId string, time int64, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?since=%v", time)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/posts"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPostsAfter gets a page of posts that were posted after the post provided.
func (c *Client4) GetPostsAfter(channelId, postId string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&after=%v", page, perPage, postId)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPostsBefore gets a page of posts that were posted before the post provided.
func (c *Client4) GetPostsBefore(channelId, postId string, page, perPage int, etag string, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&before=%v", page, perPage, postId)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoApiGet(c.GetChannelRoute(channelId)+"/posts"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// GetPostsAroundLastUnread gets a list of posts around last unread post by a user in a channel.
func (c *Client4) GetPostsAroundLastUnread(userId, channelId string, limitBefore, limitAfter int, collapsedThreads bool) (*PostList, *Response) {
	query := fmt.Sprintf("?limit_before=%v&limit_after=%v", limitBefore, limitAfter)
	if collapsedThreads {
		query += "&collapsedThreads=true"
	}
	r, err := c.DoApiGet(c.GetUserRoute(userId)+c.GetChannelRoute(channelId)+"/posts/unread"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// SearchFiles returns any posts with matching terms string.
func (c *Client4) SearchFiles(teamId string, terms string, isOrSearch bool) (*FileInfoList, *Response) {
	params := SearchParameter{
		Terms:      &terms,
		IsOrSearch: &isOrSearch,
	}
	return c.SearchFilesWithParams(teamId, &params)
}

// SearchFilesWithParams returns any posts with matching terms string.
func (c *Client4) SearchFilesWithParams(teamId string, params *SearchParameter) (*FileInfoList, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/files/search", params.SearchParameterToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoListFromJson(r.Body), BuildResponse(r)
}

// SearchPosts returns any posts with matching terms string.
func (c *Client4) SearchPosts(teamId string, terms string, isOrSearch bool) (*PostList, *Response) {
	params := SearchParameter{
		Terms:      &terms,
		IsOrSearch: &isOrSearch,
	}
	return c.SearchPostsWithParams(teamId, &params)
}

// SearchPostsWithParams returns any posts with matching terms string.
func (c *Client4) SearchPostsWithParams(teamId string, params *SearchParameter) (*PostList, *Response) {
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/posts/search", params.SearchParameterToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostListFromJson(r.Body), BuildResponse(r)
}

// SearchPostsWithMatches returns any posts with matching terms string, including.
func (c *Client4) SearchPostsWithMatches(teamId string, terms string, isOrSearch bool) (*PostSearchResults, *Response) {
	requestBody := map[string]interface{}{"terms": terms, "is_or_search": isOrSearch}
	r, err := c.DoApiPost(c.GetTeamRoute(teamId)+"/posts/search", StringInterfaceToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PostSearchResultsFromJson(r.Body), BuildResponse(r)
}

// DoPostAction performs a post action.
func (c *Client4) DoPostAction(postId, actionId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPostRoute(postId)+"/actions/"+actionId, "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DoPostActionWithCookie performs a post action with extra arguments
func (c *Client4) DoPostActionWithCookie(postId, actionId, selected, cookieStr string) (bool, *Response) {
	var body []byte
	if selected != "" || cookieStr != "" {
		body, _ = json.Marshal(DoPostActionRequest{
			SelectedOption: selected,
			Cookie:         cookieStr,
		})
	}
	r, err := c.DoApiPost(c.GetPostRoute(postId)+"/actions/"+actionId, string(body))
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
	r, err := c.DoApiPost("/actions/dialogs/open", string(b))
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
	r, err := c.DoApiPost("/actions/dialogs/submit", string(b))
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
func (c *Client4) UploadFile(data []byte, channelId string, filename string) (*FileUploadResponse, *Response) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormField("channel_id")
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPostAttachment", "model.client.upload_post_attachment.channel_id.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, err = io.Copy(part, strings.NewReader(channelId))
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
func (c *Client4) UploadFileAsRequestBody(data []byte, channelId string, filename string) (*FileUploadResponse, *Response) {
	return c.DoUploadFile(c.GetFilesRoute()+fmt.Sprintf("?channel_id=%v&filename=%v", url.QueryEscape(channelId), url.QueryEscape(filename)), data, http.DetectContentType(data))
}

// GetFile gets the bytes for a file by id.
func (c *Client4) GetFile(fileId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId), "")
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
func (c *Client4) DownloadFile(fileId string, download bool) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId)+fmt.Sprintf("?download=%v", download), "")
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
func (c *Client4) GetFileThumbnail(fileId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId)+"/thumbnail", "")
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
func (c *Client4) DownloadFileThumbnail(fileId string, download bool) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId)+fmt.Sprintf("/thumbnail?download=%v", download), "")
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
func (c *Client4) GetFileLink(fileId string) (string, *Response) {
	r, err := c.DoApiGet(c.GetFileRoute(fileId)+"/link", "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["link"], BuildResponse(r)
}

// GetFilePreview gets the bytes for a file by id.
func (c *Client4) GetFilePreview(fileId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId)+"/preview", "")
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
func (c *Client4) DownloadFilePreview(fileId string, download bool) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetFileRoute(fileId)+fmt.Sprintf("/preview?download=%v", download), "")
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
func (c *Client4) GetFileInfo(fileId string) (*FileInfo, *Response) {
	r, err := c.DoApiGet(c.GetFileRoute(fileId)+"/info", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoFromJson(r.Body), BuildResponse(r)
}

// GetFileInfosForPost gets all the file info objects attached to a post.
func (c *Client4) GetFileInfosForPost(postId string, etag string) ([]*FileInfo, *Response) {
	r, err := c.DoApiGet(c.GetPostRoute(postId)+"/files/info", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfosFromJson(r.Body), BuildResponse(r)
}

// General/System Section

// GenerateSupportPacket downloads the generated support packet
func (c *Client4) GenerateSupportPacket() ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetSystemRoute()+"/support_packet", "")
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
	r, err := c.DoApiGet(c.GetSystemRoute()+"/ping", "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return STATUS_UNHEALTHY, BuildErrorResponse(r, err)
	}
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["status"], BuildResponse(r)
}

// GetPingWithServerStatus will return ok if several basic server health checks
// all pass successfully.
func (c *Client4) GetPingWithServerStatus() (string, *Response) {
	r, err := c.DoApiGet(c.GetSystemRoute()+"/ping?get_server_status="+c.boolString(true), "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return STATUS_UNHEALTHY, BuildErrorResponse(r, err)
	}
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["status"], BuildResponse(r)
}

// GetPingWithFullServerStatus will return the full status if several basic server
// health checks all pass successfully.
func (c *Client4) GetPingWithFullServerStatus() (map[string]string, *Response) {
	r, err := c.DoApiGet(c.GetSystemRoute()+"/ping?get_server_status="+c.boolString(true), "")
	if r != nil && r.StatusCode == 500 {
		defer r.Body.Close()
		return map[string]string{"status": STATUS_UNHEALTHY}, BuildErrorResponse(r, err)
	}
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body), BuildResponse(r)
}

// TestEmail will attempt to connect to the configured SMTP server.
func (c *Client4) TestEmail(config *Config) (bool, *Response) {
	r, err := c.DoApiPost(c.GetTestEmailRoute(), config.ToJson())
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
	r, err := c.DoApiPost(c.GetTestSiteURLRoute(), MapToJson(requestBody))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// TestS3Connection will attempt to connect to the AWS S3.
func (c *Client4) TestS3Connection(config *Config) (bool, *Response) {
	r, err := c.DoApiPost(c.GetTestS3Route(), config.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetConfig will retrieve the server config with some sanitized items.
func (c *Client4) GetConfig() (*Config, *Response) {
	r, err := c.DoApiGet(c.GetConfigRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJson(r.Body), BuildResponse(r)
}

// ReloadConfig will reload the server configuration.
func (c *Client4) ReloadConfig() (bool, *Response) {
	r, err := c.DoApiPost(c.GetConfigRoute()+"/reload", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetOldClientConfig will retrieve the parts of the server configuration needed by the
// client, formatted in the old format.
func (c *Client4) GetOldClientConfig(etag string) (map[string]string, *Response) {
	r, err := c.DoApiGet(c.GetConfigRoute()+"/client?format=old", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body), BuildResponse(r)
}

// GetEnvironmentConfig will retrieve a map mirroring the server configuration where fields
// are set to true if the corresponding config setting is set through an environment variable.
// Settings that haven't been set through environment variables will be missing from the map.
func (c *Client4) GetEnvironmentConfig() (map[string]interface{}, *Response) {
	r, err := c.DoApiGet(c.GetConfigRoute()+"/environment", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StringInterfaceFromJson(r.Body), BuildResponse(r)
}

// GetOldClientLicense will retrieve the parts of the server license needed by the
// client, formatted in the old format.
func (c *Client4) GetOldClientLicense(etag string) (map[string]string, *Response) {
	r, err := c.DoApiGet(c.GetLicenseRoute()+"/client?format=old", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body), BuildResponse(r)
}

// DatabaseRecycle will recycle the connections. Discard current connection and get new one.
func (c *Client4) DatabaseRecycle() (bool, *Response) {
	r, err := c.DoApiPost(c.GetDatabaseRoute()+"/recycle", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// InvalidateCaches will purge the cache and can affect the performance while is cleaning.
func (c *Client4) InvalidateCaches() (bool, *Response) {
	r, err := c.DoApiPost(c.GetCacheRoute()+"/invalidate", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateConfig will update the server configuration.
func (c *Client4) UpdateConfig(config *Config) (*Config, *Response) {
	r, err := c.DoApiPut(c.GetConfigRoute(), config.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJson(r.Body), BuildResponse(r)
}

// MigrateConfig will migrate existing config to the new one.
func (c *Client4) MigrateConfig(from, to string) (bool, *Response) {
	m := make(map[string]string, 2)
	m["from"] = from
	m["to"] = to
	r, err := c.DoApiPost(c.GetConfigRoute()+"/migrate", MapToJson(m))
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetLicenseRoute(), bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLicenseFile", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetLicenseRoute(), "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// RemoveLicenseFile will remove the server license it exists. Note that this will
// disable all enterprise features.
func (c *Client4) RemoveLicenseFile() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetLicenseRoute())
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
func (c *Client4) GetAnalyticsOld(name, teamId string) (AnalyticsRows, *Response) {
	query := fmt.Sprintf("?name=%v&team_id=%v", name, teamId)
	r, err := c.DoApiGet(c.GetAnalyticsRoute()+"/old"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AnalyticsRowsFromJson(r.Body), BuildResponse(r)
}

// Webhooks Section

// CreateIncomingWebhook creates an incoming webhook for a channel.
func (c *Client4) CreateIncomingWebhook(hook *IncomingWebhook) (*IncomingWebhook, *Response) {
	r, err := c.DoApiPost(c.GetIncomingWebhooksRoute(), hook.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJson(r.Body), BuildResponse(r)
}

// UpdateIncomingWebhook updates an incoming webhook for a channel.
func (c *Client4) UpdateIncomingWebhook(hook *IncomingWebhook) (*IncomingWebhook, *Response) {
	r, err := c.DoApiPut(c.GetIncomingWebhookRoute(hook.Id), hook.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJson(r.Body), BuildResponse(r)
}

// GetIncomingWebhooks returns a page of incoming webhooks on the system. Page counting starts at 0.
func (c *Client4) GetIncomingWebhooks(page int, perPage int, etag string) ([]*IncomingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetIncomingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookListFromJson(r.Body), BuildResponse(r)
}

// GetIncomingWebhooksForTeam returns a page of incoming webhooks for a team. Page counting starts at 0.
func (c *Client4) GetIncomingWebhooksForTeam(teamId string, page int, perPage int, etag string) ([]*IncomingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&team_id=%v", page, perPage, teamId)
	r, err := c.DoApiGet(c.GetIncomingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookListFromJson(r.Body), BuildResponse(r)
}

// GetIncomingWebhook returns an Incoming webhook given the hook ID.
func (c *Client4) GetIncomingWebhook(hookID string, etag string) (*IncomingWebhook, *Response) {
	r, err := c.DoApiGet(c.GetIncomingWebhookRoute(hookID), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return IncomingWebhookFromJson(r.Body), BuildResponse(r)
}

// DeleteIncomingWebhook deletes and Incoming Webhook given the hook ID.
func (c *Client4) DeleteIncomingWebhook(hookID string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetIncomingWebhookRoute(hookID))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// CreateOutgoingWebhook creates an outgoing webhook for a team or channel.
func (c *Client4) CreateOutgoingWebhook(hook *OutgoingWebhook) (*OutgoingWebhook, *Response) {
	r, err := c.DoApiPost(c.GetOutgoingWebhooksRoute(), hook.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJson(r.Body), BuildResponse(r)
}

// UpdateOutgoingWebhook creates an outgoing webhook for a team or channel.
func (c *Client4) UpdateOutgoingWebhook(hook *OutgoingWebhook) (*OutgoingWebhook, *Response) {
	r, err := c.DoApiPut(c.GetOutgoingWebhookRoute(hook.Id), hook.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJson(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooks returns a page of outgoing webhooks on the system. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooks(page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJson(r.Body), BuildResponse(r)
}

// GetOutgoingWebhook outgoing webhooks on the system requested by Hook Id.
func (c *Client4) GetOutgoingWebhook(hookId string) (*OutgoingWebhook, *Response) {
	r, err := c.DoApiGet(c.GetOutgoingWebhookRoute(hookId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJson(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooksForChannel returns a page of outgoing webhooks for a channel. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooksForChannel(channelId string, page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&channel_id=%v", page, perPage, channelId)
	r, err := c.DoApiGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJson(r.Body), BuildResponse(r)
}

// GetOutgoingWebhooksForTeam returns a page of outgoing webhooks for a team. Page counting starts at 0.
func (c *Client4) GetOutgoingWebhooksForTeam(teamId string, page int, perPage int, etag string) ([]*OutgoingWebhook, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&team_id=%v", page, perPage, teamId)
	r, err := c.DoApiGet(c.GetOutgoingWebhooksRoute()+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookListFromJson(r.Body), BuildResponse(r)
}

// RegenOutgoingHookToken regenerate the outgoing webhook token.
func (c *Client4) RegenOutgoingHookToken(hookId string) (*OutgoingWebhook, *Response) {
	r, err := c.DoApiPost(c.GetOutgoingWebhookRoute(hookId)+"/regen_token", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OutgoingWebhookFromJson(r.Body), BuildResponse(r)
}

// DeleteOutgoingWebhook delete the outgoing webhook on the system requested by Hook Id.
func (c *Client4) DeleteOutgoingWebhook(hookId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetOutgoingWebhookRoute(hookId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Preferences Section

// GetPreferences returns the user's preferences.
func (c *Client4) GetPreferences(userId string) (Preferences, *Response) {
	r, err := c.DoApiGet(c.GetPreferencesRoute(userId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	preferences, _ := PreferencesFromJson(r.Body)
	return preferences, BuildResponse(r)
}

// UpdatePreferences saves the user's preferences.
func (c *Client4) UpdatePreferences(userId string, preferences *Preferences) (bool, *Response) {
	r, err := c.DoApiPut(c.GetPreferencesRoute(userId), preferences.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return true, BuildResponse(r)
}

// DeletePreferences deletes the user's preferences.
func (c *Client4) DeletePreferences(userId string, preferences *Preferences) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPreferencesRoute(userId)+"/delete", preferences.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return true, BuildResponse(r)
}

// GetPreferencesByCategory returns the user's preferences from the provided category string.
func (c *Client4) GetPreferencesByCategory(userId string, category string) (Preferences, *Response) {
	url := fmt.Sprintf(c.GetPreferencesRoute(userId)+"/%s", category)
	r, err := c.DoApiGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	preferences, _ := PreferencesFromJson(r.Body)
	return preferences, BuildResponse(r)
}

// GetPreferenceByCategoryAndName returns the user's preferences from the provided category and preference name string.
func (c *Client4) GetPreferenceByCategoryAndName(userId string, category string, preferenceName string) (*Preference, *Response) {
	url := fmt.Sprintf(c.GetPreferencesRoute(userId)+"/%s/name/%v", category, preferenceName)
	r, err := c.DoApiGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PreferenceFromJson(r.Body), BuildResponse(r)
}

// SAML Section

// GetSamlMetadata returns metadata for the SAML configuration.
func (c *Client4) GetSamlMetadata() (string, *Response) {
	r, err := c.DoApiGet(c.GetSamlRoute()+"/metadata", "")
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
func (c *Client4) UploadSamlIdpCertificate(data []byte, filename string) (bool, *Response) {
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
func (c *Client4) DeleteSamlIdpCertificate() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetSamlRoute() + "/certificate/idp")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteSamlPublicCertificate deletes the SAML IDP certificate from the server and updates the config to not use it and disable SAML.
func (c *Client4) DeleteSamlPublicCertificate() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetSamlRoute() + "/certificate/public")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteSamlPrivateCertificate deletes the SAML IDP certificate from the server and updates the config to not use it and disable SAML.
func (c *Client4) DeleteSamlPrivateCertificate() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetSamlRoute() + "/certificate/private")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetSamlCertificateStatus returns metadata for the SAML configuration.
func (c *Client4) GetSamlCertificateStatus() (*SamlCertificateStatus, *Response) {
	r, err := c.DoApiGet(c.GetSamlRoute()+"/certificate/status", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SamlCertificateStatusFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetSamlMetadataFromIdp(samlMetadataURL string) (*SamlMetadataResponse, *Response) {
	requestBody := make(map[string]string)
	requestBody["saml_metadata_url"] = samlMetadataURL
	r, err := c.DoApiPost(c.GetSamlRoute()+"/metadatafromidp", MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}

	defer closeBody(r)
	return SamlMetadataResponseFromJson(r.Body), BuildResponse(r)
}

// ResetSamlAuthDataToEmail resets the AuthData field of SAML users to their Email.
func (c *Client4) ResetSamlAuthDataToEmail(includeDeleted bool, dryRun bool, userIDs []string) (int64, *Response) {
	params := map[string]interface{}{
		"include_deleted": includeDeleted,
		"dry_run":         dryRun,
		"user_ids":        userIDs,
	}
	b, _ := json.Marshal(params)
	r, err := c.doApiPostBytes(c.GetSamlRoute()+"/reset_auth_data", b)
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
	r, err := c.DoApiPost(c.GetComplianceReportsRoute(), report.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ComplianceFromJson(r.Body), BuildResponse(r)
}

// GetComplianceReports returns list of compliance reports.
func (c *Client4) GetComplianceReports(page, perPage int) (Compliances, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetComplianceReportsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CompliancesFromJson(r.Body), BuildResponse(r)
}

// GetComplianceReport returns a compliance report.
func (c *Client4) GetComplianceReport(reportId string) (*Compliance, *Response) {
	r, err := c.DoApiGet(c.GetComplianceReportRoute(reportId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ComplianceFromJson(r.Body), BuildResponse(r)
}

// DownloadComplianceReport returns a full compliance report as a file.
func (c *Client4) DownloadComplianceReport(reportId string) ([]byte, *Response) {
	rq, err := http.NewRequest("GET", c.ApiUrl+c.GetComplianceReportDownloadRoute(reportId), nil)
	if err != nil {
		return nil, &Response{Error: NewAppError("DownloadComplianceReport", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, "BEARER "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, &Response{Error: NewAppError("DownloadComplianceReport", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
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
	r, err := c.DoApiGet(c.GetClusterRoute()+"/status", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ClusterInfosFromJson(r.Body), BuildResponse(r)
}

// LDAP Section

// SyncLdap will force a sync with the configured LDAP server.
// If includeRemovedMembers is true, then group members who left or were removed from a
// synced team/channel will be re-joined; otherwise, they will be excluded.
func (c *Client4) SyncLdap(includeRemovedMembers bool) (bool, *Response) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"include_removed_members": includeRemovedMembers,
	})
	r, err := c.doApiPostBytes(c.GetLdapRoute()+"/sync", reqBody)
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// TestLdap will attempt to connect to the configured LDAP server and return OK if configured
// correctly.
func (c *Client4) TestLdap() (bool, *Response) {
	r, err := c.DoApiPost(c.GetLdapRoute()+"/test", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetLdapGroups retrieves the immediate child groups of the given parent group.
func (c *Client4) GetLdapGroups() ([]*Group, *Response) {
	path := fmt.Sprintf("%s/groups", c.GetLdapRoute())

	r, appErr := c.DoApiGet(path, "")
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

	r, appErr := c.DoApiPost(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupFromJson(r.Body), BuildResponse(r)
}

// UnlinkLdapGroup deletes the Mattermost group associated with the given LDAP group DN.
func (c *Client4) UnlinkLdapGroup(dn string) (*Group, *Response) {
	path := fmt.Sprintf("%s/groups/%s/link", c.GetLdapRoute(), dn)

	r, appErr := c.DoApiDelete(path)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupFromJson(r.Body), BuildResponse(r)
}

// MigrateIdLdap migrates the LDAP enabled users to given attribute
func (c *Client4) MigrateIdLdap(toAttribute string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetLdapRoute()+"/migrateid", MapToJson(map[string]string{
		"toAttribute": toAttribute,
	}))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetGroupsByChannel retrieves the Mattermost Groups associated with a given channel
func (c *Client4) GetGroupsByChannel(channelId string, opts GroupSearchOpts) ([]*GroupWithSchemeAdmin, int, *Response) {
	path := fmt.Sprintf("%s/groups?q=%v&include_member_count=%v&filter_allow_reference=%v", c.GetChannelRoute(channelId), opts.Q, opts.IncludeMemberCount, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoApiGet(path, "")
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
func (c *Client4) GetGroupsByTeam(teamId string, opts GroupSearchOpts) ([]*GroupWithSchemeAdmin, int, *Response) {
	path := fmt.Sprintf("%s/groups?q=%v&include_member_count=%v&filter_allow_reference=%v", c.GetTeamRoute(teamId), opts.Q, opts.IncludeMemberCount, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoApiGet(path, "")
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
func (c *Client4) GetGroupsAssociatedToChannelsByTeam(teamId string, opts GroupSearchOpts) (map[string][]*GroupWithSchemeAdmin, *Response) {
	path := fmt.Sprintf("%s/groups_by_channels?q=%v&filter_allow_reference=%v", c.GetTeamRoute(teamId), opts.Q, opts.FilterAllowReference)
	if opts.PageOpts != nil {
		path = fmt.Sprintf("%s&page=%v&per_page=%v", path, opts.PageOpts.Page, opts.PageOpts.PerPage)
	}
	r, appErr := c.DoApiGet(path, "")
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
	r, appErr := c.DoApiGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return GroupsFromJson(r.Body), BuildResponse(r)
}

// GetGroupsByUserId retrieves Mattermost Groups for a user
func (c *Client4) GetGroupsByUserId(userId string) ([]*Group, *Response) {
	path := fmt.Sprintf(
		"%s/%v/groups",
		c.GetUsersRoute(),
		userId,
	)

	r, appErr := c.DoApiGet(path, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupsFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) MigrateAuthToLdap(fromAuthService string, matchField string, force bool) (bool, *Response) {
	r, err := c.DoApiPost(c.GetUsersRoute()+"/migrate_auth/ldap", StringInterfaceToJson(map[string]interface{}{
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
	r, err := c.DoApiPost(c.GetUsersRoute()+"/migrate_auth/saml", StringInterfaceToJson(map[string]interface{}{
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
	body, writer, err := fileToMultipart(data, LDAP_PUBLIC_CERTIFICATE_NAME)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLdapPublicCertificate", "model.client.upload_ldap_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetLdapRoute()+"/certificate/public", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// UploadLdapPrivateCertificate will upload a private key for LDAP and set the config to use it.
func (c *Client4) UploadLdapPrivateCertificate(data []byte) (bool, *Response) {
	body, writer, err := fileToMultipart(data, LDAP_PRIVATE_KEY_NAME)
	if err != nil {
		return false, &Response{Error: NewAppError("UploadLdapPrivateCertificate", "model.client.upload_Ldap_cert.app_error", nil, err.Error(), http.StatusBadRequest)}
	}

	_, resp := c.DoUploadFile(c.GetLdapRoute()+"/certificate/private", body, writer.FormDataContentType())
	return resp.Error == nil, resp
}

// DeleteLdapPublicCertificate deletes the LDAP IDP certificate from the server and updates the config to not use it and disable LDAP.
func (c *Client4) DeleteLdapPublicCertificate() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetLdapRoute() + "/certificate/public")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteLDAPPrivateCertificate deletes the LDAP IDP certificate from the server and updates the config to not use it and disable LDAP.
func (c *Client4) DeleteLdapPrivateCertificate() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetLdapRoute() + "/certificate/private")
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
	r, err := c.DoApiGet("/audits"+query, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AuditsFromJson(r.Body), BuildResponse(r)
}

// Brand Section

// GetBrandImage retrieves the previously uploaded brand image.
func (c *Client4) GetBrandImage() ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetBrandRoute()+"/image", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	if r.StatusCode >= 300 {
		return nil, BuildErrorResponse(r, AppErrorFromJson(r.Body))
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("GetBrandImage", "model.client.read_file.app_error", nil, err.Error(), r.StatusCode))
	}

	return data, BuildResponse(r)
}

// DeleteBrandImage deletes the brand image for the system.
func (c *Client4) DeleteBrandImage() *Response {
	r, err := c.DoApiDelete(c.GetBrandRoute() + "/image")
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetBrandRoute()+"/image", bytes.NewReader(body.Bytes()))
	if err != nil {
		return false, &Response{Error: NewAppError("UploadBrandImage", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return false, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.GetBrandRoute()+"/image", "model.client.connecting.app_error", nil, err.Error(), http.StatusForbidden)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return false, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return CheckStatusOK(rp), BuildResponse(rp)
}

// Logs Section

// GetLogs page of logs as a string array.
func (c *Client4) GetLogs(page, perPage int) ([]string, *Response) {
	query := fmt.Sprintf("?page=%v&logs_per_page=%v", page, perPage)
	r, err := c.DoApiGet("/logs"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

// PostLog is a convenience Web Service call so clients can log messages into
// the server-side logs. For example we typically log javascript error messages
// into the server-side. It returns the log message if the logging was successful.
func (c *Client4) PostLog(message map[string]string) (map[string]string, *Response) {
	r, err := c.DoApiPost("/logs", MapToJson(message))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body), BuildResponse(r)
}

// OAuth Section

// CreateOAuthApp will register a new OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) CreateOAuthApp(app *OAuthApp) (*OAuthApp, *Response) {
	r, err := c.DoApiPost(c.GetOAuthAppsRoute(), app.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJson(r.Body), BuildResponse(r)
}

// UpdateOAuthApp updates a page of registered OAuth 2.0 client applications with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) UpdateOAuthApp(app *OAuthApp) (*OAuthApp, *Response) {
	r, err := c.DoApiPut(c.GetOAuthAppRoute(app.Id), app.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJson(r.Body), BuildResponse(r)
}

// GetOAuthApps gets a page of registered OAuth 2.0 client applications with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthApps(page, perPage int) ([]*OAuthApp, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetOAuthAppsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppListFromJson(r.Body), BuildResponse(r)
}

// GetOAuthApp gets a registered OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthApp(appId string) (*OAuthApp, *Response) {
	r, err := c.DoApiGet(c.GetOAuthAppRoute(appId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJson(r.Body), BuildResponse(r)
}

// GetOAuthAppInfo gets a sanitized version of a registered OAuth 2.0 client application with Mattermost acting as an OAuth 2.0 service provider.
func (c *Client4) GetOAuthAppInfo(appId string) (*OAuthApp, *Response) {
	r, err := c.DoApiGet(c.GetOAuthAppRoute(appId)+"/info", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJson(r.Body), BuildResponse(r)
}

// DeleteOAuthApp deletes a registered OAuth 2.0 client application.
func (c *Client4) DeleteOAuthApp(appId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetOAuthAppRoute(appId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// RegenerateOAuthAppSecret regenerates the client secret for a registered OAuth 2.0 client application.
func (c *Client4) RegenerateOAuthAppSecret(appId string) (*OAuthApp, *Response) {
	r, err := c.DoApiPost(c.GetOAuthAppRoute(appId)+"/regen_secret", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppFromJson(r.Body), BuildResponse(r)
}

// GetAuthorizedOAuthAppsForUser gets a page of OAuth 2.0 client applications the user has authorized to use access their account.
func (c *Client4) GetAuthorizedOAuthAppsForUser(userId string, page, perPage int) ([]*OAuthApp, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v", page, perPage)
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/oauth/apps/authorized"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return OAuthAppListFromJson(r.Body), BuildResponse(r)
}

// AuthorizeOAuthApp will authorize an OAuth 2.0 client application to access a user's account and provide a redirect link to follow.
func (c *Client4) AuthorizeOAuthApp(authRequest *AuthorizeRequest) (string, *Response) {
	r, err := c.DoApiRequest(http.MethodPost, c.Url+"/oauth/authorize", authRequest.ToJson(), "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["redirect"], BuildResponse(r)
}

// DeauthorizeOAuthApp will deauthorize an OAuth 2.0 client application from accessing a user's account.
func (c *Client4) DeauthorizeOAuthApp(appId string) (bool, *Response) {
	requestData := map[string]string{"client_id": appId}
	r, err := c.DoApiRequest(http.MethodPost, c.Url+"/oauth/deauthorize", MapToJson(requestData), "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetOAuthAccessToken is a test helper function for the OAuth access token endpoint.
func (c *Client4) GetOAuthAccessToken(data url.Values) (*AccessResponse, *Response) {
	rq, err := http.NewRequest(http.MethodPost, c.Url+"/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, &Response{Error: NewAppError(c.Url+"/oauth/access_token", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, &Response{StatusCode: http.StatusForbidden, Error: NewAppError(c.Url+"/oauth/access_token", "model.client.connecting.app_error", nil, err.Error(), 403)}
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return AccessResponseFromJson(rp.Body), BuildResponse(rp)
}

// Elasticsearch Section

// TestElasticsearch will attempt to connect to the configured Elasticsearch server and return OK if configured.
// correctly.
func (c *Client4) TestElasticsearch() (bool, *Response) {
	r, err := c.DoApiPost(c.GetElasticsearchRoute()+"/test", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PurgeElasticsearchIndexes immediately deletes all Elasticsearch indexes.
func (c *Client4) PurgeElasticsearchIndexes() (bool, *Response) {
	r, err := c.DoApiPost(c.GetElasticsearchRoute()+"/purge_indexes", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Bleve Section

// PurgeBleveIndexes immediately deletes all Bleve indexes.
func (c *Client4) PurgeBleveIndexes() (bool, *Response) {
	r, err := c.DoApiPost(c.GetBleveRoute()+"/purge_indexes", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// Data Retention Section

// GetDataRetentionPolicy will get the current global data retention policy details.
func (c *Client4) GetDataRetentionPolicy() (*GlobalRetentionPolicy, *Response) {
	r, err := c.DoApiGet(c.GetDataRetentionRoute()+"/policy", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return GlobalRetentionPolicyFromJson(r.Body), BuildResponse(r)
}

// GetDataRetentionPolicyByID will get the details for the granular data retention policy with the specified ID.
func (c *Client4) GetDataRetentionPolicyByID(policyID string) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.DoApiGet(c.GetDataRetentionPolicyRoute(policyID), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policy, err := RetentionPolicyWithTeamAndChannelCountsFromJson(r.Body)
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
	r, appErr := c.DoApiGet(c.GetDataRetentionRoute()+"/policies_count", "")
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
	r, appErr := c.DoApiGet(c.GetDataRetentionRoute()+"/policies"+query, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policies, err := RetentionPolicyWithTeamAndChannelCountsListFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetDataRetentionPolicies", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return policies, BuildResponse(r)
}

// CreateDataRetentionPolicy will create a new granular data retention policy which will be applied to
// the specified teams and channels. The Id field of `policy` must be empty.
func (c *Client4) CreateDataRetentionPolicy(policy *RetentionPolicyWithTeamAndChannelIDs) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.doApiPostBytes(c.GetDataRetentionRoute()+"/policies", policy.ToJson())
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	newPolicy, err := RetentionPolicyWithTeamAndChannelCountsFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.CreateDataRetentionPolicy", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return newPolicy, BuildResponse(r)
}

// DeleteDataRetentionPolicy will delete the granular data retention policy with the specified ID.
func (c *Client4) DeleteDataRetentionPolicy(policyID string) *Response {
	r, appErr := c.DoApiDelete(c.GetDataRetentionPolicyRoute(policyID))
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// PatchDataRetentionPolicy will patch the granular data retention policy with the specified ID.
// The Id field of `patch` must be non-empty.
func (c *Client4) PatchDataRetentionPolicy(patch *RetentionPolicyWithTeamAndChannelIDs) (*RetentionPolicyWithTeamAndChannelCounts, *Response) {
	r, appErr := c.doApiPatchBytes(c.GetDataRetentionPolicyRoute(patch.ID), patch.ToJson())
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	policy, err := RetentionPolicyWithTeamAndChannelCountsFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.PatchDataRetentionPolicy", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return policy, BuildResponse(r)
}

// GetTeamsForRetentionPolicy will get the teams to which the specified policy is currently applied.
func (c *Client4) GetTeamsForRetentionPolicy(policyID string, page, perPage int) (*TeamsWithCount, *Response) {
	query := fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	r, appErr := c.DoApiGet(c.GetDataRetentionPolicyRoute(policyID)+"/teams"+query, "")
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
	r, appErr := c.doApiPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams/search", body)
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
	r, appErr := c.doApiPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams", body)
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
	r, appErr := c.doApiDeleteBytes(c.GetDataRetentionPolicyRoute(policyID)+"/teams", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// GetChannelsForRetentionPolicy will get the channels to which the specified policy is currently applied.
func (c *Client4) GetChannelsForRetentionPolicy(policyID string, page, perPage int) (*ChannelsWithCount, *Response) {
	query := fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	r, appErr := c.DoApiGet(c.GetDataRetentionPolicyRoute(policyID)+"/channels"+query, "")
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
	r, appErr := c.doApiPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels/search", body)
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
	r, appErr := c.doApiPostBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels", body)
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
	r, appErr := c.doApiDeleteBytes(c.GetDataRetentionPolicyRoute(policyID)+"/channels", body)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// GetTeamPoliciesForUser will get the data retention policies for the teams to which a user belongs.
func (c *Client4) GetTeamPoliciesForUser(userID string, offset, limit int) (*RetentionPolicyForTeamList, *Response) {
	r, appErr := c.DoApiGet(c.GetUserRoute(userID)+"/data_retention/team_policies", "")
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
	r, appErr := c.DoApiGet(c.GetUserRoute(userID)+"/data_retention/channel_policies", "")
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
	r, err := c.DoApiPost(c.GetCommandsRoute(), cmd.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJson(r.Body), BuildResponse(r)
}

// UpdateCommand updates a command based on the provided Command struct.
func (c *Client4) UpdateCommand(cmd *Command) (*Command, *Response) {
	r, err := c.DoApiPut(c.GetCommandRoute(cmd.Id), cmd.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJson(r.Body), BuildResponse(r)
}

// MoveCommand moves a command to a different team.
func (c *Client4) MoveCommand(teamId string, commandId string) (bool, *Response) {
	cmr := CommandMoveRequest{TeamId: teamId}
	r, err := c.DoApiPut(c.GetCommandMoveRoute(commandId), cmr.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DeleteCommand deletes a command based on the provided command id string.
func (c *Client4) DeleteCommand(commandId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetCommandRoute(commandId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ListCommands will retrieve a list of commands available in the team.
func (c *Client4) ListCommands(teamId string, customOnly bool) ([]*Command, *Response) {
	query := fmt.Sprintf("?team_id=%v&custom_only=%v", teamId, customOnly)
	r, err := c.DoApiGet(c.GetCommandsRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandListFromJson(r.Body), BuildResponse(r)
}

// ListCommandAutocompleteSuggestions will retrieve a list of suggestions for a userInput.
func (c *Client4) ListCommandAutocompleteSuggestions(userInput, teamId string) ([]AutocompleteSuggestion, *Response) {
	query := fmt.Sprintf("/commands/autocomplete_suggestions?user_input=%v", userInput)
	r, err := c.DoApiGet(c.GetTeamRoute(teamId)+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return AutocompleteSuggestionsFromJSON(r.Body), BuildResponse(r)
}

// GetCommandById will retrieve a command by id.
func (c *Client4) GetCommandById(cmdId string) (*Command, *Response) {
	url := fmt.Sprintf("%s/%s", c.GetCommandsRoute(), cmdId)
	r, err := c.DoApiGet(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandFromJson(r.Body), BuildResponse(r)
}

// ExecuteCommand executes a given slash command.
func (c *Client4) ExecuteCommand(channelId, command string) (*CommandResponse, *Response) {
	commandArgs := &CommandArgs{
		ChannelId: channelId,
		Command:   command,
	}
	r, err := c.DoApiPost(c.GetCommandsRoute()+"/execute", commandArgs.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	response, _ := CommandResponseFromJson(r.Body)
	return response, BuildResponse(r)
}

// ExecuteCommandWithTeam executes a given slash command against the specified team.
// Use this when executing slash commands in a DM/GM, since the team id cannot be inferred in that case.
func (c *Client4) ExecuteCommandWithTeam(channelId, teamId, command string) (*CommandResponse, *Response) {
	commandArgs := &CommandArgs{
		ChannelId: channelId,
		TeamId:    teamId,
		Command:   command,
	}
	r, err := c.DoApiPost(c.GetCommandsRoute()+"/execute", commandArgs.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	response, _ := CommandResponseFromJson(r.Body)
	return response, BuildResponse(r)
}

// ListAutocompleteCommands will retrieve a list of commands available in the team.
func (c *Client4) ListAutocompleteCommands(teamId string) ([]*Command, *Response) {
	r, err := c.DoApiGet(c.GetTeamAutoCompleteCommandsRoute(teamId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CommandListFromJson(r.Body), BuildResponse(r)
}

// RegenCommandToken will create a new token if the user have the right permissions.
func (c *Client4) RegenCommandToken(commandId string) (string, *Response) {
	r, err := c.DoApiPut(c.GetCommandRoute(commandId)+"/regen_token", "")
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["token"], BuildResponse(r)
}

// Status Section

// GetUserStatus returns a user based on the provided user id string.
func (c *Client4) GetUserStatus(userId, etag string) (*Status, *Response) {
	r, err := c.DoApiGet(c.GetUserStatusRoute(userId), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusFromJson(r.Body), BuildResponse(r)
}

// GetUsersStatusesByIds returns a list of users status based on the provided user ids.
func (c *Client4) GetUsersStatusesByIds(userIds []string) ([]*Status, *Response) {
	r, err := c.DoApiPost(c.GetUserStatusesRoute()+"/ids", ArrayToJson(userIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusListFromJson(r.Body), BuildResponse(r)
}

// UpdateUserStatus sets a user's status based on the provided user id string.
func (c *Client4) UpdateUserStatus(userId string, userStatus *Status) (*Status, *Response) {
	r, err := c.DoApiPut(c.GetUserStatusRoute(userId), userStatus.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return StatusFromJson(r.Body), BuildResponse(r)
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

	if err := writer.WriteField("emoji", emoji.ToJson()); err != nil {
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
	r, err := c.DoApiGet(c.GetEmojisRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJson(r.Body), BuildResponse(r)
}

// GetSortedEmojiList returns a page of custom emoji on the system sorted based on the sort
// parameter, blank for no sorting and "name" to sort by emoji names.
func (c *Client4) GetSortedEmojiList(page, perPage int, sort string) ([]*Emoji, *Response) {
	query := fmt.Sprintf("?page=%v&per_page=%v&sort=%v", page, perPage, sort)
	r, err := c.DoApiGet(c.GetEmojisRoute()+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJson(r.Body), BuildResponse(r)
}

// DeleteEmoji delete an custom emoji on the provided emoji id string.
func (c *Client4) DeleteEmoji(emojiId string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetEmojiRoute(emojiId))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetEmoji returns a custom emoji based on the emojiId string.
func (c *Client4) GetEmoji(emojiId string) (*Emoji, *Response) {
	r, err := c.DoApiGet(c.GetEmojiRoute(emojiId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiFromJson(r.Body), BuildResponse(r)
}

// GetEmojiByName returns a custom emoji based on the name string.
func (c *Client4) GetEmojiByName(name string) (*Emoji, *Response) {
	r, err := c.DoApiGet(c.GetEmojiByNameRoute(name), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiFromJson(r.Body), BuildResponse(r)
}

// GetEmojiImage returns the emoji image.
func (c *Client4) GetEmojiImage(emojiId string) ([]byte, *Response) {
	r, apErr := c.DoApiGet(c.GetEmojiRoute(emojiId)+"/image", "")
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
	r, err := c.DoApiPost(c.GetEmojisRoute()+"/search", search.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJson(r.Body), BuildResponse(r)
}

// AutocompleteEmoji returns a list of emoji starting with or matching name.
func (c *Client4) AutocompleteEmoji(name string, etag string) ([]*Emoji, *Response) {
	query := fmt.Sprintf("?name=%v", name)
	r, err := c.DoApiGet(c.GetEmojisRoute()+"/autocomplete"+query, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return EmojiListFromJson(r.Body), BuildResponse(r)
}

// Reaction Section

// SaveReaction saves an emoji reaction for a post. Returns the saved reaction if successful, otherwise an error will be returned.
func (c *Client4) SaveReaction(reaction *Reaction) (*Reaction, *Response) {
	r, err := c.DoApiPost(c.GetReactionsRoute(), reaction.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ReactionFromJson(r.Body), BuildResponse(r)
}

// GetReactions returns a list of reactions to a post.
func (c *Client4) GetReactions(postId string) ([]*Reaction, *Response) {
	r, err := c.DoApiGet(c.GetPostRoute(postId)+"/reactions", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ReactionsFromJson(r.Body), BuildResponse(r)
}

// DeleteReaction deletes reaction of a user in a post.
func (c *Client4) DeleteReaction(reaction *Reaction) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetUserRoute(reaction.UserId) + c.GetPostRoute(reaction.PostId) + fmt.Sprintf("/reactions/%v", reaction.EmojiName))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// FetchBulkReactions returns a map of postIds and corresponding reactions
func (c *Client4) GetBulkReactions(postIds []string) (map[string][]*Reaction, *Response) {
	r, err := c.DoApiPost(c.GetPostsRoute()+"/ids/reactions", ArrayToJson(postIds))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapPostIdToReactionsFromJson(r.Body), BuildResponse(r)
}

// Timezone Section

// GetSupportedTimezone returns a page of supported timezones on the system.
func (c *Client4) GetSupportedTimezone() ([]string, *Response) {
	r, err := c.DoApiGet(c.GetTimezonesRoute(), "")
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

	r, err := c.DoApiPost(c.GetOpenGraphRoute(), MapToJson(requestBody))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body), BuildResponse(r)
}

// Jobs Section

// GetJob gets a single job.
func (c *Client4) GetJob(id string) (*Job, *Response) {
	r, err := c.DoApiGet(c.GetJobsRoute()+fmt.Sprintf("/%v", id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobFromJson(r.Body), BuildResponse(r)
}

// GetJobs gets all jobs, sorted with the job that was created most recently first.
func (c *Client4) GetJobs(page int, perPage int) ([]*Job, *Response) {
	r, err := c.DoApiGet(c.GetJobsRoute()+fmt.Sprintf("?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobsFromJson(r.Body), BuildResponse(r)
}

// GetJobsByType gets all jobs of a given type, sorted with the job that was created most recently first.
func (c *Client4) GetJobsByType(jobType string, page int, perPage int) ([]*Job, *Response) {
	r, err := c.DoApiGet(c.GetJobsRoute()+fmt.Sprintf("/type/%v?page=%v&per_page=%v", jobType, page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobsFromJson(r.Body), BuildResponse(r)
}

// CreateJob creates a job based on the provided job struct.
func (c *Client4) CreateJob(job *Job) (*Job, *Response) {
	r, err := c.DoApiPost(c.GetJobsRoute(), job.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return JobFromJson(r.Body), BuildResponse(r)
}

// CancelJob requests the cancellation of the job with the provided Id.
func (c *Client4) CancelJob(jobId string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetJobsRoute()+fmt.Sprintf("/%v/cancel", jobId), "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DownloadJob downloads the results of the job
func (c *Client4) DownloadJob(jobId string) ([]byte, *Response) {
	r, appErr := c.DoApiGet(c.GetJobsRoute()+fmt.Sprintf("/%v/download", jobId), "")
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
	r, err := c.DoApiGet(c.GetRolesRoute()+fmt.Sprintf("/%v", id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJson(r.Body), BuildResponse(r)
}

// GetRoleByName gets a single role by Name.
func (c *Client4) GetRoleByName(name string) (*Role, *Response) {
	r, err := c.DoApiGet(c.GetRolesRoute()+fmt.Sprintf("/name/%v", name), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJson(r.Body), BuildResponse(r)
}

// GetRolesByNames returns a list of roles based on the provided role names.
func (c *Client4) GetRolesByNames(roleNames []string) ([]*Role, *Response) {
	r, err := c.DoApiPost(c.GetRolesRoute()+"/names", ArrayToJson(roleNames))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleListFromJson(r.Body), BuildResponse(r)
}

// PatchRole partially updates a role in the system. Any missing fields are not updated.
func (c *Client4) PatchRole(roleId string, patch *RolePatch) (*Role, *Response) {
	r, err := c.DoApiPut(c.GetRolesRoute()+fmt.Sprintf("/%v/patch", roleId), patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return RoleFromJson(r.Body), BuildResponse(r)
}

// Schemes Section

// CreateScheme creates a new Scheme.
func (c *Client4) CreateScheme(scheme *Scheme) (*Scheme, *Response) {
	r, err := c.DoApiPost(c.GetSchemesRoute(), scheme.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJson(r.Body), BuildResponse(r)
}

// GetScheme gets a single scheme by ID.
func (c *Client4) GetScheme(id string) (*Scheme, *Response) {
	r, err := c.DoApiGet(c.GetSchemeRoute(id), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJson(r.Body), BuildResponse(r)
}

// GetSchemes gets all schemes, sorted with the most recently created first, optionally filtered by scope.
func (c *Client4) GetSchemes(scope string, page int, perPage int) ([]*Scheme, *Response) {
	r, err := c.DoApiGet(c.GetSchemesRoute()+fmt.Sprintf("?scope=%v&page=%v&per_page=%v", scope, page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemesFromJson(r.Body), BuildResponse(r)
}

// DeleteScheme deletes a single scheme by ID.
func (c *Client4) DeleteScheme(id string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetSchemeRoute(id))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// PatchScheme partially updates a scheme in the system. Any missing fields are not updated.
func (c *Client4) PatchScheme(id string, patch *SchemePatch) (*Scheme, *Response) {
	r, err := c.DoApiPut(c.GetSchemeRoute(id)+"/patch", patch.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return SchemeFromJson(r.Body), BuildResponse(r)
}

// GetTeamsForScheme gets the teams using this scheme, sorted alphabetically by display name.
func (c *Client4) GetTeamsForScheme(schemeId string, page int, perPage int) ([]*Team, *Response) {
	r, err := c.DoApiGet(c.GetSchemeRoute(schemeId)+fmt.Sprintf("/teams?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TeamListFromJson(r.Body), BuildResponse(r)
}

// GetChannelsForScheme gets the channels using this scheme, sorted alphabetically by display name.
func (c *Client4) GetChannelsForScheme(schemeId string, page int, perPage int) (ChannelList, *Response) {
	r, err := c.DoApiGet(c.GetSchemeRoute(schemeId)+fmt.Sprintf("/channels?page=%v&per_page=%v", page, perPage), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return *ChannelListFromJson(r.Body), BuildResponse(r)
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

	rq, err := http.NewRequest("POST", c.ApiUrl+c.GetPluginsRoute(), body)
	if err != nil {
		return nil, &Response{Error: NewAppError("UploadPlugin", "model.client.connecting.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	rq.Header.Set("Content-Type", writer.FormDataContentType())

	if c.AuthToken != "" {
		rq.Header.Set(HEADER_AUTH, c.AuthType+" "+c.AuthToken)
	}

	rp, err := c.HttpClient.Do(rq)
	if err != nil || rp == nil {
		return nil, BuildErrorResponse(rp, NewAppError("UploadPlugin", "model.client.connecting.app_error", nil, err.Error(), 0))
	}
	defer closeBody(rp)

	if rp.StatusCode >= 300 {
		return nil, BuildErrorResponse(rp, AppErrorFromJson(rp.Body))
	}

	return ManifestFromJson(rp.Body), BuildResponse(rp)
}

func (c *Client4) InstallPluginFromUrl(downloadUrl string, force bool) (*Manifest, *Response) {
	forceStr := c.boolString(force)

	url := fmt.Sprintf("%s?plugin_download_url=%s&force=%s", c.GetPluginsRoute()+"/install_from_url", url.QueryEscape(downloadUrl), forceStr)
	r, err := c.DoApiPost(url, "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ManifestFromJson(r.Body), BuildResponse(r)
}

// InstallMarketplacePlugin will install marketplace plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) InstallMarketplacePlugin(request *InstallMarketplacePluginRequest) (*Manifest, *Response) {
	json, err := request.ToJson()
	if err != nil {
		return nil, &Response{Error: NewAppError("InstallMarketplacePlugin", "model.client.plugin_request_to_json.app_error", nil, err.Error(), http.StatusBadRequest)}
	}
	r, appErr := c.DoApiPost(c.GetPluginsRoute()+"/marketplace", json)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return ManifestFromJson(r.Body), BuildResponse(r)
}

// GetPlugins will return a list of plugin manifests for currently active plugins.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetPlugins() (*PluginsResponse, *Response) {
	r, err := c.DoApiGet(c.GetPluginsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PluginsResponseFromJson(r.Body), BuildResponse(r)
}

// GetPluginStatuses will return the plugins installed on any server in the cluster, for reporting
// to the administrator via the system console.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetPluginStatuses() (PluginStatuses, *Response) {
	r, err := c.DoApiGet(c.GetPluginsRoute()+"/statuses", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return PluginStatusesFromJson(r.Body), BuildResponse(r)
}

// RemovePlugin will disable and delete a plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) RemovePlugin(id string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetPluginRoute(id))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetWebappPlugins will return a list of plugins that the webapp should download.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) GetWebappPlugins() ([]*Manifest, *Response) {
	r, err := c.DoApiGet(c.GetPluginsRoute()+"/webapp", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ManifestListFromJson(r.Body), BuildResponse(r)
}

// EnablePlugin will enable an plugin installed.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) EnablePlugin(id string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPluginRoute(id)+"/enable", "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// DisablePlugin will disable an enabled plugin.
// WARNING: PLUGINS ARE STILL EXPERIMENTAL. THIS FUNCTION IS SUBJECT TO CHANGE.
func (c *Client4) DisablePlugin(id string) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPluginRoute(id)+"/disable", "")
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

	r, err := c.DoApiGet(u.String(), "")
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
func (c *Client4) UpdateChannelScheme(channelId, schemeId string) (bool, *Response) {
	sip := &SchemeIDPatch{SchemeID: &schemeId}
	r, err := c.DoApiPut(c.GetChannelSchemeRoute(channelId), sip.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// UpdateTeamScheme will update a team's scheme.
func (c *Client4) UpdateTeamScheme(teamId, schemeId string) (bool, *Response) {
	sip := &SchemeIDPatch{SchemeID: &schemeId}
	r, err := c.DoApiPut(c.GetTeamSchemeRoute(teamId), sip.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetRedirectLocation retrieves the value of the 'Location' header of an HTTP response for a given URL.
func (c *Client4) GetRedirectLocation(urlParam, etag string) (string, *Response) {
	url := fmt.Sprintf("%s?url=%s", c.GetRedirectLocationRoute(), url.QueryEscape(urlParam))
	r, err := c.DoApiGet(url, etag)
	if err != nil {
		return "", BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return MapFromJson(r.Body)["location"], BuildResponse(r)
}

// SetServerBusy will mark the server as busy, which disables non-critical services for `secs` seconds.
func (c *Client4) SetServerBusy(secs int) (bool, *Response) {
	url := fmt.Sprintf("%s?seconds=%d", c.GetServerBusyRoute(), secs)
	r, err := c.DoApiPost(url, "")
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// ClearServerBusy will mark the server as not busy.
func (c *Client4) ClearServerBusy() (bool, *Response) {
	r, err := c.DoApiDelete(c.GetServerBusyRoute())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetServerBusy returns the current ServerBusyState including the time when a server marked busy
// will automatically have the flag cleared.
func (c *Client4) GetServerBusy() (*ServerBusyState, *Response) {
	r, err := c.DoApiGet(c.GetServerBusyRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	sbs := ServerBusyStateFromJson(r.Body)
	return sbs, BuildResponse(r)
}

// GetServerBusyExpires returns the time when a server marked busy
// will automatically have the flag cleared.
//
// Deprecated: Use GetServerBusy instead.
func (c *Client4) GetServerBusyExpires() (*time.Time, *Response) {
	r, err := c.DoApiGet(c.GetServerBusyRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)

	sbs := ServerBusyStateFromJson(r.Body)
	expires := time.Unix(sbs.Expires, 0)
	return &expires, BuildResponse(r)
}

// RegisterTermsOfServiceAction saves action performed by a user against a specific terms of service.
func (c *Client4) RegisterTermsOfServiceAction(userId, termsOfServiceId string, accepted bool) (*bool, *Response) {
	url := c.GetUserTermsOfServiceRoute(userId)
	data := map[string]interface{}{"termsOfServiceId": termsOfServiceId, "accepted": accepted}
	r, err := c.DoApiPost(url, StringInterfaceToJson(data))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return NewBool(CheckStatusOK(r)), BuildResponse(r)
}

// GetTermsOfService fetches the latest terms of service
func (c *Client4) GetTermsOfService(etag string) (*TermsOfService, *Response) {
	url := c.GetTermsOfServiceRoute()
	r, err := c.DoApiGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TermsOfServiceFromJson(r.Body), BuildResponse(r)
}

// GetUserTermsOfService fetches user's latest terms of service action if the latest action was for acceptance.
func (c *Client4) GetUserTermsOfService(userId, etag string) (*UserTermsOfService, *Response) {
	url := c.GetUserTermsOfServiceRoute(userId)
	r, err := c.DoApiGet(url, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UserTermsOfServiceFromJson(r.Body), BuildResponse(r)
}

// CreateTermsOfService creates new terms of service.
func (c *Client4) CreateTermsOfService(text, userId string) (*TermsOfService, *Response) {
	url := c.GetTermsOfServiceRoute()
	data := map[string]interface{}{"text": text}
	r, err := c.DoApiPost(url, StringInterfaceToJson(data))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return TermsOfServiceFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetGroup(groupID, etag string) (*Group, *Response) {
	r, appErr := c.DoApiGet(c.GetGroupRoute(groupID), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) PatchGroup(groupID string, patch *GroupPatch) (*Group, *Response) {
	payload, _ := json.Marshal(patch)
	r, appErr := c.DoApiPut(c.GetGroupRoute(groupID)+"/patch", string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) LinkGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, patch *GroupSyncablePatch) (*GroupSyncable, *Response) {
	payload, _ := json.Marshal(patch)
	url := fmt.Sprintf("%s/link", c.GetGroupSyncableRoute(groupID, syncableID, syncableType))
	r, appErr := c.DoApiPost(url, string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) UnlinkGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType) *Response {
	url := fmt.Sprintf("%s/link", c.GetGroupSyncableRoute(groupID, syncableID, syncableType))
	r, appErr := c.DoApiDelete(url)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

func (c *Client4) GetGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, etag string) (*GroupSyncable, *Response) {
	r, appErr := c.DoApiGet(c.GetGroupSyncableRoute(groupID, syncableID, syncableType), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetGroupSyncables(groupID string, syncableType GroupSyncableType, etag string) ([]*GroupSyncable, *Response) {
	r, appErr := c.DoApiGet(c.GetGroupSyncablesRoute(groupID, syncableType), etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncablesFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) PatchGroupSyncable(groupID, syncableID string, syncableType GroupSyncableType, patch *GroupSyncablePatch) (*GroupSyncable, *Response) {
	payload, _ := json.Marshal(patch)
	r, appErr := c.DoApiPut(c.GetGroupSyncableRoute(groupID, syncableID, syncableType)+"/patch", string(payload))
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupSyncableFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) TeamMembersMinusGroupMembers(teamID string, groupIDs []string, page, perPage int, etag string) ([]*UserWithGroups, int64, *Response) {
	groupIDStr := strings.Join(groupIDs, ",")
	query := fmt.Sprintf("?group_ids=%s&page=%d&per_page=%d", groupIDStr, page, perPage)
	r, err := c.DoApiGet(c.GetTeamRoute(teamID)+"/members_minus_group_members"+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	ugc := UsersWithGroupsAndCountFromJson(r.Body)
	return ugc.Users, ugc.Count, BuildResponse(r)
}

func (c *Client4) ChannelMembersMinusGroupMembers(channelID string, groupIDs []string, page, perPage int, etag string) ([]*UserWithGroups, int64, *Response) {
	groupIDStr := strings.Join(groupIDs, ",")
	query := fmt.Sprintf("?group_ids=%s&page=%d&per_page=%d", groupIDStr, page, perPage)
	r, err := c.DoApiGet(c.GetChannelRoute(channelID)+"/members_minus_group_members"+query, etag)
	if err != nil {
		return nil, 0, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	ugc := UsersWithGroupsAndCountFromJson(r.Body)
	return ugc.Users, ugc.Count, BuildResponse(r)
}

func (c *Client4) PatchConfig(config *Config) (*Config, *Response) {
	r, err := c.DoApiPut(c.GetConfigRoute()+"/patch", config.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ConfigFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetChannelModerations(channelID string, etag string) ([]*ChannelModeration, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelID)+"/moderations", etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelModerationsFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) PatchChannelModerations(channelID string, patch []*ChannelModerationPatch) ([]*ChannelModeration, *Response) {
	payload, _ := json.Marshal(patch)
	r, err := c.DoApiPut(c.GetChannelRoute(channelID)+"/moderations/patch", string(payload))
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelModerationsFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetKnownUsers() ([]string, *Response) {
	r, err := c.DoApiGet(c.GetUsersRoute()+"/known", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	var userIds []string
	json.NewDecoder(r.Body).Decode(&userIds)
	return userIds, BuildResponse(r)
}

// PublishUserTyping publishes a user is typing websocket event based on the provided TypingRequest.
func (c *Client4) PublishUserTyping(userID string, typingRequest TypingRequest) (bool, *Response) {
	r, err := c.DoApiPost(c.GetPublishUserTypingRoute(userID), typingRequest.ToJson())
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

func (c *Client4) GetChannelMemberCountsByGroup(channelID string, includeTimezones bool, etag string) ([]*ChannelMemberCountByGroup, *Response) {
	r, err := c.DoApiGet(c.GetChannelRoute(channelID)+"/member_counts_by_group?include_timezones="+strconv.FormatBool(includeTimezones), etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ChannelMemberCountsByGroupFromJson(r.Body), BuildResponse(r)
}

// RequestTrialLicense will request a trial license and install it in the server
func (c *Client4) RequestTrialLicense(users int) (bool, *Response) {
	b, _ := json.Marshal(map[string]interface{}{"users": users, "terms_accepted": true})
	r, err := c.DoApiPost("/trial-license", string(b))
	if err != nil {
		return false, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return CheckStatusOK(r), BuildResponse(r)
}

// GetGroupStats retrieves stats for a Mattermost Group
func (c *Client4) GetGroupStats(groupID string) (*GroupStats, *Response) {
	r, appErr := c.DoApiGet(c.GetGroupRoute(groupID)+"/stats", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	return GroupStatsFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetSidebarCategoriesForTeamForUser(userID, teamID, etag string) (*OrderedSidebarCategories, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID)
	r, appErr := c.DoApiGet(route, etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	cat, err := OrderedSidebarCategoriesFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.GetSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return cat, BuildResponse(r)
}

func (c *Client4) CreateSidebarCategoryForTeamForUser(userID, teamID string, category *SidebarCategoryWithChannels) (*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(category)
	route := c.GetUserCategoryRoute(userID, teamID)
	r, appErr := c.doApiPostBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.CreateSidebarCategoryForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}
	return cat, BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoriesForTeamForUser(userID, teamID string, categories []*SidebarCategoryWithChannels) ([]*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(categories)
	route := c.GetUserCategoryRoute(userID, teamID)

	r, appErr := c.doApiPutBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	categories, err := SidebarCategoriesFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return categories, BuildResponse(r)
}

func (c *Client4) GetSidebarCategoryOrderForTeamForUser(userID, teamID, etag string) ([]string, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID) + "/order"
	r, err := c.DoApiGet(route, etag)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoryOrderForTeamForUser(userID, teamID string, order []string) ([]string, *Response) {
	payload, _ := json.Marshal(order)
	route := c.GetUserCategoryRoute(userID, teamID) + "/order"
	r, err := c.doApiPutBytes(route, payload)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) GetSidebarCategoryForTeamForUser(userID, teamID, categoryID, etag string) (*SidebarCategoryWithChannels, *Response) {
	route := c.GetUserCategoryRoute(userID, teamID) + "/" + categoryID
	r, appErr := c.DoApiGet(route, etag)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return cat, BuildResponse(r)
}

func (c *Client4) UpdateSidebarCategoryForTeamForUser(userID, teamID, categoryID string, category *SidebarCategoryWithChannels) (*SidebarCategoryWithChannels, *Response) {
	payload, _ := json.Marshal(category)
	route := c.GetUserCategoryRoute(userID, teamID) + "/" + categoryID
	r, appErr := c.doApiPutBytes(route, payload)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	cat, err := SidebarCategoryFromJson(r.Body)
	if err != nil {
		return nil, BuildErrorResponse(r, NewAppError("Client4.UpdateSidebarCategoriesForTeamForUser", "model.utils.decode_json.app_error", nil, err.Error(), r.StatusCode))
	}

	return cat, BuildResponse(r)
}

// CheckIntegrity performs a database integrity check.
func (c *Client4) CheckIntegrity() ([]IntegrityCheckResult, *Response) {
	r, err := c.DoApiPost("/integrity", "")
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

func (c *Client4) GetNotices(lastViewed int64, teamId string, client NoticeClientType, clientVersion, locale, etag string) (NoticeMessages, *Response) {
	url := fmt.Sprintf("/system/notices/%s?lastViewed=%d&client=%s&clientVersion=%s&locale=%s", teamId, lastViewed, client, clientVersion, locale)
	r, appErr := c.DoApiGet(url, etag)
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
	r, err := c.DoApiPut("/system/notices/view", ArrayToJson(ids))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// CreateUpload creates a new upload session.
func (c *Client4) CreateUpload(us *UploadSession) (*UploadSession, *Response) {
	r, err := c.DoApiPost(c.GetUploadsRoute(), us.ToJson())
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionFromJson(r.Body), BuildResponse(r)
}

// GetUpload returns the upload session for the specified uploadId.
func (c *Client4) GetUpload(uploadId string) (*UploadSession, *Response) {
	r, err := c.DoApiGet(c.GetUploadRoute(uploadId), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionFromJson(r.Body), BuildResponse(r)
}

// GetUploadsForUser returns the upload sessions created by the specified
// userId.
func (c *Client4) GetUploadsForUser(userId string) ([]*UploadSession, *Response) {
	r, err := c.DoApiGet(c.GetUserRoute(userId)+"/uploads", "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return UploadSessionsFromJson(r.Body), BuildResponse(r)
}

// UploadData performs an upload. On success it returns
// a FileInfo object.
func (c *Client4) UploadData(uploadId string, data io.Reader) (*FileInfo, *Response) {
	url := c.GetUploadRoute(uploadId)
	r, err := c.doApiRequestReader("POST", c.ApiUrl+url, data, nil)
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return FileInfoFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) UpdatePassword(userId, currentPassword, newPassword string) *Response {
	requestBody := map[string]string{"current_password": currentPassword, "new_password": newPassword}
	r, err := c.DoApiPut(c.GetUserRoute(userId)+"/password", MapToJson(requestBody))
	if err != nil {
		return BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return BuildResponse(r)
}

// Cloud Section

func (c *Client4) GetCloudProducts() ([]*Product, *Response) {
	r, appErr := c.DoApiGet(c.GetCloudRoute()+"/products", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var cloudProducts []*Product
	json.NewDecoder(r.Body).Decode(&cloudProducts)

	return cloudProducts, BuildResponse(r)
}

func (c *Client4) CreateCustomerPayment() (*StripeSetupIntent, *Response) {
	r, appErr := c.DoApiPost(c.GetCloudRoute()+"/payment", "")
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

	r, appErr := c.doApiPostBytes(c.GetCloudRoute()+"/payment/confirm", json)
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) GetCloudCustomer() (*CloudCustomer, *Response) {
	r, appErr := c.DoApiGet(c.GetCloudRoute()+"/customer", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var cloudCustomer *CloudCustomer
	json.NewDecoder(r.Body).Decode(&cloudCustomer)

	return cloudCustomer, BuildResponse(r)
}

func (c *Client4) GetSubscription() (*Subscription, *Response) {
	r, appErr := c.DoApiGet(c.GetCloudRoute()+"/subscription", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var subscription *Subscription
	json.NewDecoder(r.Body).Decode(&subscription)

	return subscription, BuildResponse(r)
}

func (c *Client4) GetSubscriptionStats() (*SubscriptionStats, *Response) {
	r, appErr := c.DoApiGet(c.GetCloudRoute()+"/subscription/stats", "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var stats *SubscriptionStats
	json.NewDecoder(r.Body).Decode(&stats)
	return stats, BuildResponse(r)
}

func (c *Client4) GetInvoicesForSubscription() ([]*Invoice, *Response) {
	r, appErr := c.DoApiGet(c.GetCloudRoute()+"/subscription/invoices", "")
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

	r, appErr := c.doApiPutBytes(c.GetCloudRoute()+"/customer", customerBytes)
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

	r, appErr := c.doApiPutBytes(c.GetCloudRoute()+"/customer/address", addressBytes)
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var customer *CloudCustomer
	json.NewDecoder(r.Body).Decode(&customer)

	return customer, BuildResponse(r)
}

func (c *Client4) ListImports() ([]string, *Response) {
	r, err := c.DoApiGet(c.GetImportsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) ListExports() ([]string, *Response) {
	r, err := c.DoApiGet(c.GetExportsRoute(), "")
	if err != nil {
		return nil, BuildErrorResponse(r, err)
	}
	defer closeBody(r)
	return ArrayFromJson(r.Body), BuildResponse(r)
}

func (c *Client4) DeleteExport(name string) (bool, *Response) {
	r, err := c.DoApiDelete(c.GetExportRoute(name))
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
			HEADER_RANGE: fmt.Sprintf("bytes=%d-", offset),
		}
	}
	r, appErr := c.DoApiRequestWithHeaders(http.MethodGet, c.ApiUrl+c.GetExportRoute(name), "", headers)
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

func (c *Client4) GetUserThreads(userId, teamId string, options GetUserThreadsOpts) (*Threads, *Response) {
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
	url := c.GetUserThreadsRoute(userId, teamId)
	if len(v) > 0 {
		url += "?" + v.Encode()
	}

	r, appErr := c.DoApiGet(url, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var threads Threads
	json.NewDecoder(r.Body).Decode(&threads)

	return &threads, BuildResponse(r)
}

func (c *Client4) GetUserThread(userId, teamId, threadId string, extended bool) (*ThreadResponse, *Response) {
	url := c.GetUserThreadRoute(userId, teamId, threadId)
	if extended {
		url += "?extended=true"
	}
	r, appErr := c.DoApiGet(url, "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	var thread ThreadResponse
	json.NewDecoder(r.Body).Decode(&thread)

	return &thread, BuildResponse(r)
}

func (c *Client4) UpdateThreadsReadForUser(userId, teamId string) *Response {
	r, appErr := c.DoApiPut(fmt.Sprintf("%s/read", c.GetUserThreadsRoute(userId, teamId)), "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) UpdateThreadReadForUser(userId, teamId, threadId string, timestamp int64) (*ThreadResponse, *Response) {
	r, appErr := c.DoApiPut(fmt.Sprintf("%s/read/%d", c.GetUserThreadRoute(userId, teamId, threadId), timestamp), "")
	if appErr != nil {
		return nil, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)
	var thread ThreadResponse
	json.NewDecoder(r.Body).Decode(&thread)

	return &thread, BuildResponse(r)
}

func (c *Client4) UpdateThreadFollowForUser(userId, teamId, threadId string, state bool) *Response {
	var appErr *AppError
	var r *http.Response
	if state {
		r, appErr = c.DoApiPut(c.GetUserThreadRoute(userId, teamId, threadId)+"/following", "")
	} else {
		r, appErr = c.DoApiDelete(c.GetUserThreadRoute(userId, teamId, threadId) + "/following")
	}
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) SendAdminUpgradeRequestEmail() *Response {
	r, appErr := c.DoApiPost(c.GetCloudRoute()+"/subscription/limitreached/invite", "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) SendAdminUpgradeRequestEmailOnJoin() *Response {
	r, appErr := c.DoApiPost(c.GetCloudRoute()+"/subscription/limitreached/join", "")
	if appErr != nil {
		return BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	return BuildResponse(r)
}

func (c *Client4) GetAllSharedChannels(teamID string, page, perPage int) ([]*SharedChannel, *Response) {
	url := fmt.Sprintf("%s/%s?page=%d&per_page=%d", c.GetSharedChannelsRoute(), teamID, page, perPage)
	r, appErr := c.DoApiGet(url, "")
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
	r, appErr := c.DoApiGet(url, "")
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
	r, appErr := c.DoApiGet(url, "")
	if appErr != nil {
		return returnedPermissions, BuildErrorResponse(r, appErr)
	}
	defer closeBody(r)

	json.NewDecoder(r.Body).Decode(&returnedPermissions)
	return returnedPermissions, BuildResponse(r)
}
