// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	ConnSecurityNone     = ""
	ConnSecurityPlain    = "PLAIN"
	ConnSecurityTls      = "TLS"
	ConnSecurityStarttls = "STARTTLS"

	ImageDriverLocal = "local"
	ImageDriverS3    = "amazons3"

	DatabaseDriverMysql    = "mysql"
	DatabaseDriverPostgres = "postgres"

	SearchengineElasticsearch = "elasticsearch"

	MinioAccessKey = "minioaccesskey"
	MinioSecretKey = "miniosecretkey"
	MinioBucket    = "mattermost-test"

	PasswordMaximumLength = 64
	PasswordMinimumLength = 5

	ServiceGitlab    = "gitlab"
	ServiceGoogle    = "google"
	ServiceOffice365 = "office365"
	ServiceOpenid    = "openid"

	GenericNoChannelNotification = "generic_no_channel"
	GenericNotification          = "generic"
	GenericNotificationServer    = "https://push-test.mattermost.com"
	MmSupportAdvisorAddress      = "support-advisor@mattermost.com"
	FullNotification             = "full"
	IDLoadedNotification         = "id_loaded"

	DirectMessageAny  = "any"
	DirectMessageTeam = "team"

	ShowUsername         = "username"
	ShowNicknameFullName = "nickname_full_name"
	ShowFullName         = "full_name"

	PermissionsAll          = "all"
	PermissionsChannelAdmin = "channel_admin"
	PermissionsTeamAdmin    = "team_admin"
	PermissionsSystemAdmin  = "system_admin"

	FakeSetting = "********************************"

	RestrictEmojiCreationAll         = "all"
	RestrictEmojiCreationAdmin       = "admin"
	RestrictEmojiCreationSystemAdmin = "system_admin"

	PermissionsDeletePostAll         = "all"
	PermissionsDeletePostTeamAdmin   = "team_admin"
	PermissionsDeletePostSystemAdmin = "system_admin"

	AllowEditPostAlways    = "always"
	AllowEditPostNever     = "never"
	AllowEditPostTimeLimit = "time_limit"

	GroupUnreadChannelsDisabled   = "disabled"
	GroupUnreadChannelsDefaultOn  = "default_on"
	GroupUnreadChannelsDefaultOff = "default_off"

	CollapsedThreadsDisabled   = "disabled"
	CollapsedThreadsDefaultOn  = "default_on"
	CollapsedThreadsDefaultOff = "default_off"

	EmailBatchingBufferSize = 256
	EmailBatchingInterval   = 30

	EmailNotificationContentsFull    = "full"
	EmailNotificationContentsGeneric = "generic"

	SitenameMaxLength = 30

	ServiceSettingsDefaultSiteURL          = "http://localhost:8065"
	ServiceSettingsDefaultTlsCertFile      = ""
	ServiceSettingsDefaultTlsKeyFile       = ""
	ServiceSettingsDefaultReadTimeout      = 300
	ServiceSettingsDefaultWriteTimeout     = 300
	ServiceSettingsDefaultIdleTimeout      = 60
	ServiceSettingsDefaultMaxLoginAttempts = 10
	ServiceSettingsDefaultAllowCorsFrom    = ""
	ServiceSettingsDefaultListenAndAddress = ":8065"
	ServiceSettingsDefaultGfycatAPIKey     = "2_KtH_W5"
	ServiceSettingsDefaultGfycatAPISecret  = "3wLVZPiswc3DnaiaFoLkDvB4X0IV6CpMkj4tf2inJRsBY6-FnkT08zGmppWFgeof"

	TeamSettingsDefaultSiteName              = "Mattermost"
	TeamSettingsDefaultMaxUsersPerTeam       = 50
	TeamSettingsDefaultCustomBrandText       = ""
	TeamSettingsDefaultCustomDescriptionText = ""
	TeamSettingsDefaultUserStatusAwayTimeout = 300

	SQLSettingsDefaultDataSource = "postgres://mmuser:mostest@localhost/mattermost_test?sslmode=disable&connect_timeout=10"

	FileSettingsDefaultDirectory = "./data/"

	ImportSettingsDefaultDirectory     = "./import"
	ImportSettingsDefaultRetentionDays = 30

	ExportSettingsDefaultDirectory     = "./export"
	ExportSettingsDefaultRetentionDays = 30

	EmailSettingsDefaultFeedbackOrganization = ""

	SupportSettingsDefaultTermsOfServiceLink = "https://mattermost.com/terms-of-service/"
	SupportSettingsDefaultPrivacyPolicyLink  = "https://mattermost.com/privacy-policy/"
	SupportSettingsDefaultAboutLink          = "https://about.mattermost.com/default-about/"
	SupportSettingsDefaultHelpLink           = "https://about.mattermost.com/default-help/"
	SupportSettingsDefaultReportAProblemLink = "https://about.mattermost.com/default-report-a-problem/"
	SupportSettingsDefaultSupportEmail       = ""
	SupportSettingsDefaultReAcceptancePeriod = 365

	LdapSettingsDefaultFirstNameAttribute        = ""
	LdapSettingsDefaultLastNameAttribute         = ""
	LdapSettingsDefaultEmailAttribute            = ""
	LdapSettingsDefaultUsernameAttribute         = ""
	LdapSettingsDefaultNicknameAttribute         = ""
	LdapSettingsDefaultIDAttribute               = ""
	LdapSettingsDefaultPositionAttribute         = ""
	LdapSettingsDefaultLoginFieldName            = ""
	LdapSettingsDefaultGroupDisplayNameAttribute = ""
	LdapSettingsDefaultGroupIDAttribute          = ""
	LdapSettingsDefaultPictureAttribute          = ""

	SamlSettingsDefaultIDAttribute        = ""
	SamlSettingsDefaultGuestAttribute     = ""
	SamlSettingsDefaultAdminAttribute     = ""
	SamlSettingsDefaultFirstNameAttribute = ""
	SamlSettingsDefaultLastNameAttribute  = ""
	SamlSettingsDefaultEmailAttribute     = ""
	SamlSettingsDefaultUsernameAttribute  = ""
	SamlSettingsDefaultNicknameAttribute  = ""
	SamlSettingsDefaultLocaleAttribute    = ""
	SamlSettingsDefaultPositionAttribute  = ""

	SamlSettingsSignatureAlgorithmSha1    = "RSAwithSHA1"
	SamlSettingsSignatureAlgorithmSha256  = "RSAwithSHA256"
	SamlSettingsSignatureAlgorithmSha512  = "RSAwithSHA512"
	SamlSettingsDefaultSignatureAlgorithm = SamlSettingsSignatureAlgorithmSha1

	SamlSettingsCanonicalAlgorithmC14n    = "Canonical1.0"
	SamlSettingsCanonicalAlgorithmC14n11  = "Canonical1.1"
	SamlSettingsDefaultCanonicalAlgorithm = SamlSettingsCanonicalAlgorithmC14n

	NativeappSettingsDefaultAppDownloadLink        = "https://mattermost.com/download/#mattermostApps"
	NativeappSettingsDefaultAndroidAppDownloadLink = "https://about.mattermost.com/mattermost-android-app/"
	NativeappSettingsDefaultIosAppDownloadLink     = "https://about.mattermost.com/mattermost-ios-app/"

	ExperimentalSettingsDefaultLinkMetadataTimeoutMilliseconds = 5000

	AnalyticsSettingsDefaultMaxUsersForStatistics = 2500

	AnnouncementSettingsDefaultBannerColor                  = "#f2a93b"
	AnnouncementSettingsDefaultBannerTextColor              = "#333333"
	AnnouncementSettingsDefaultNoticesJSONURL               = "https://notices.mattermost.com/"
	AnnouncementSettingsDefaultNoticesFetchFrequencySeconds = 3600

	TeamSettingsDefaultTeamText = "default"

	ElasticsearchSettingsDefaultConnectionURL                 = "http://localhost:9200"
	ElasticsearchSettingsDefaultUsername                      = "elastic"
	ElasticsearchSettingsDefaultPassword                      = "changeme"
	ElasticsearchSettingsDefaultPostIndexReplicas             = 1
	ElasticsearchSettingsDefaultPostIndexShards               = 1
	ElasticsearchSettingsDefaultChannelIndexReplicas          = 1
	ElasticsearchSettingsDefaultChannelIndexShards            = 1
	ElasticsearchSettingsDefaultUserIndexReplicas             = 1
	ElasticsearchSettingsDefaultUserIndexShards               = 1
	ElasticsearchSettingsDefaultAggregatePostsAfterDays       = 365
	ElasticsearchSettingsDefaultPostsAggregatorJobStartTime   = "03:00"
	ElasticsearchSettingsDefaultIndexPrefix                   = ""
	ElasticsearchSettingsDefaultLiveIndexingBatchSize         = 1
	ElasticsearchSettingsDefaultBulkIndexingTimeWindowSeconds = 3600
	ElasticsearchSettingsDefaultRequestTimeoutSeconds         = 30

	BleveSettingsDefaultIndexDir                      = ""
	BleveSettingsDefaultBulkIndexingTimeWindowSeconds = 3600

	DataRetentionSettingsDefaultMessageRetentionDays = 365
	DataRetentionSettingsDefaultFileRetentionDays    = 365
	DataRetentionSettingsDefaultDeletionJobStartTime = "02:00"
	DataRetentionSettingsDefaultBatchSize            = 3000

	PluginSettingsDefaultDirectory         = "./plugins"
	PluginSettingsDefaultClientDirectory   = "./client/plugins"
	PluginSettingsDefaultEnableMarketplace = true
	PluginSettingsDefaultMarketplaceURL    = "https://api.integrations.mattermost.com"
	PluginSettingsOldMarketplaceURL        = "https://marketplace.integrations.mattermost.com"

	ComplianceExportTypeCsv            = "csv"
	ComplianceExportTypeActiance       = "actiance"
	ComplianceExportTypeGlobalrelay    = "globalrelay"
	ComplianceExportTypeGlobalrelayZip = "globalrelay-zip"
	GlobalrelayCustomerTypeA9          = "A9"
	GlobalrelayCustomerTypeA10         = "A10"

	ClientSideCertCheckPrimaryAuth   = "primary"
	ClientSideCertCheckSecondaryAuth = "secondary"

	ImageProxyTypeLocal     = "local"
	ImageProxyTypeAtmosCamo = "atmos/camo"

	GoogleSettingsDefaultScope           = "profile email"
	GoogleSettingsDefaultAuthEndpoint    = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleSettingsDefaultTokenEndpoint   = "https://www.googleapis.com/oauth2/v4/token"
	GoogleSettingsDefaultUserAPIEndpoint = "https://people.googleapis.com/v1/people/me?personFields=names,emailAddresses,nicknames,metadata"

	Office365SettingsDefaultScope           = "User.Read"
	Office365SettingsDefaultAuthEndpoint    = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	Office365SettingsDefaultTokenEndpoint   = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	Office365SettingsDefaultUserAPIEndpoint = "https://graph.microsoft.com/v1.0/me"

	CloudSettingsDefaultCwsURL    = "https://customers.mattermost.com"
	CloudSettingsDefaultCwsAPIURL = "https://portal.internal.prod.cloud.mattermost.com"
	OpenidSettingsDefaultScope    = "profile openid email"

	LocalModeSocketPath = "/var/tmp/mattermost_local.socket"
)

func GetDefaultAppCustomURLSchemes() []string {
	return []string{"mmauth://", "mmauthbeta://"}
}

var ServerTLSSupportedCiphers = map[string]uint16{
	"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

type ServiceSettings struct {
	SiteURL                                           *string  `access:"environment_web_server,authentication_saml,write_restrictable"`
	WebsocketURL                                      *string  `access:"write_restrictable,cloud_restrictable"`
	LicenseFileLocation                               *string  `access:"write_restrictable,cloud_restrictable"`                        // telemetry: none
	ListenAddress                                     *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"` // telemetry: none
	ConnectionSecurity                                *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	TLSCertFile                                       *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	TLSKeyFile                                        *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	TLSMinVer                                         *string  `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	TLSStrictTransport                                *bool    `access:"write_restrictable,cloud_restrictable"`
	TLSStrictTransportMaxAge                          *int64   `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	TLSOverwriteCiphers                               []string `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	UseLetsEncrypt                                    *bool    `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	LetsEncryptCertificateCacheFile                   *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"` // telemetry: none
	Forward80To443                                    *bool    `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	TrustedProxyIPHeader                              []string `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	ReadTimeout                                       *int     `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	WriteTimeout                                      *int     `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	IdleTimeout                                       *int     `access:"write_restrictable,cloud_restrictable"`
	MaximumLoginAttempts                              *int     `access:"authentication_password,write_restrictable,cloud_restrictable"`
	GoroutineHealthThreshold                          *int     `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	EnableOAuthServiceProvider                        *bool    `access:"integrations_integration_management"`
	EnableIncomingWebhooks                            *bool    `access:"integrations_integration_management"`
	EnableOutgoingWebhooks                            *bool    `access:"integrations_integration_management"`
	EnableCommands                                    *bool    `access:"integrations_integration_management"`
	EnablePostUsernameOverride                        *bool    `access:"integrations_integration_management"`
	EnablePostIconOverride                            *bool    `access:"integrations_integration_management"`
	GoogleDeveloperKey                                *string  `access:"site_posts,write_restrictable,cloud_restrictable"`
	DEPRECATED_DO_NOT_USE_EnableOnlyAdminIntegrations *bool    `json:"EnableOnlyAdminIntegrations" mapstructure:"EnableOnlyAdminIntegrations"` // Deprecated: do not use
	EnableLinkPreviews                                *bool    `access:"site_posts"`
	RestrictLinkPreviews                              *string  `access:"site_posts"`
	EnableTesting                                     *bool    `access:"environment_developer,write_restrictable,cloud_restrictable"`
	EnableDeveloper                                   *bool    `access:"environment_developer,write_restrictable,cloud_restrictable"`
	EnableOpenTracing                                 *bool    `access:"write_restrictable,cloud_restrictable"`
	EnableSecurityFixAlert                            *bool    `access:"environment_smtp,write_restrictable,cloud_restrictable"`
	EnableInsecureOutgoingConnections                 *bool    `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	AllowedUntrustedInternalConnections               *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	EnableMultifactorAuthentication                   *bool    `access:"authentication_mfa"`
	EnforceMultifactorAuthentication                  *bool    `access:"authentication_mfa"`
	EnableUserAccessTokens                            *bool    `access:"integrations_integration_management"`
	AllowCorsFrom                                     *string  `access:"integrations_cors,write_restrictable,cloud_restrictable"`
	CorsExposedHeaders                                *string  `access:"integrations_cors,write_restrictable,cloud_restrictable"`
	CorsAllowCredentials                              *bool    `access:"integrations_cors,write_restrictable,cloud_restrictable"`
	CorsDebug                                         *bool    `access:"integrations_cors,write_restrictable,cloud_restrictable"`
	AllowCookiesForSubdomains                         *bool    `access:"write_restrictable,cloud_restrictable"`
	ExtendSessionLengthWithActivity                   *bool    `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	SessionLengthWebInDays                            *int     `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	SessionLengthMobileInDays                         *int     `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	SessionLengthSSOInDays                            *int     `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	SessionCacheInMinutes                             *int     `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	SessionIdleTimeoutInMinutes                       *int     `access:"environment_session_lengths,write_restrictable,cloud_restrictable"`
	WebsocketSecurePort                               *int     `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	WebsocketPort                                     *int     `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	WebserverMode                                     *string  `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	EnableGifPicker                                   *bool    `access:"integrations_gif"`
	GfycatAPIKey                                      *string  `access:"integrations_gif"`
	GfycatAPISecret                                   *string  `access:"integrations_gif"`
	EnableCustomEmoji                                 *bool    `access:"site_emoji"`
	EnableEmojiPicker                                 *bool    `access:"site_emoji"`
	DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation *string  `json:"RestrictCustomEmojiCreation" mapstructure:"RestrictCustomEmojiCreation"` // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPostDelete          *string  `json:"RestrictPostDelete" mapstructure:"RestrictPostDelete"`                   // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_AllowEditPost               *string  `json:"AllowEditPost" mapstructure:"AllowEditPost"`                             // Deprecated: do not use
	PostEditTimeLimit                                 *int     `access:"user_management_permissions"`
	TimeBetweenUserTypingUpdatesMilliseconds          *int64   `access:"experimental_features,write_restrictable,cloud_restrictable"`
	EnablePostSearch                                  *bool    `access:"write_restrictable,cloud_restrictable"`
	EnableFileSearch                                  *bool    `access:"write_restrictable"`
	MinimumHashtagLength                              *int     `access:"environment_database,write_restrictable,cloud_restrictable"`
	EnableUserTypingMessages                          *bool    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	EnableChannelViewedMessages                       *bool    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	EnableUserStatuses                                *bool    `access:"write_restrictable,cloud_restrictable"`
	ExperimentalEnableAuthenticationTransfer          *bool    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	ClusterLogTimeoutMilliseconds                     *int     `access:"write_restrictable,cloud_restrictable"`
	CloseUnusedDirectMessages                         *bool    `access:"experimental_features"`
	EnablePreviewFeatures                             *bool    `access:"experimental_features"`
	EnableTutorial                                    *bool    `access:"experimental_features"`
	ExperimentalEnableDefaultChannelLeaveJoinMessages *bool    `access:"experimental_features"`
	ExperimentalGroupUnreadChannels                   *string  `access:"experimental_features"`
	ExperimentalChannelOrganization                   *bool    `access:"experimental_features"`
	DEPRECATED_DO_NOT_USE_ImageProxyType              *string  `json:"ImageProxyType" mapstructure:"ImageProxyType"`       // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_ImageProxyURL               *string  `json:"ImageProxyURL" mapstructure:"ImageProxyURL"`         // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_ImageProxyOptions           *string  `json:"ImageProxyOptions" mapstructure:"ImageProxyOptions"` // Deprecated: do not use
	EnableAPITeamDeletion                             *bool
	EnableAPIUserDeletion                             *bool
	ExperimentalEnableHardenedMode                    *bool `access:"experimental_features"`
	DisableLegacyMFA                                  *bool `access:"write_restrictable,cloud_restrictable"`
	ExperimentalStrictCSRFEnforcement                 *bool `access:"experimental_features,write_restrictable,cloud_restrictable"`
	EnableEmailInvitations                            *bool `access:"authentication_signup"`
	DisableBotsWhenOwnerIsDeactivated                 *bool `access:"integrations_bot_accounts,write_restrictable,cloud_restrictable"`
	EnableBotAccountCreation                          *bool `access:"integrations_bot_accounts"`
	EnableSVGs                                        *bool `access:"site_posts"`
	EnableLatex                                       *bool `access:"site_posts"`
	EnableAPIChannelDeletion                          *bool
	EnableLocalMode                                   *bool
	LocalModeSocketLocation                           *string // telemetry: none
	EnableAWSMetering                                 *bool   // telemetry: none
	SplitKey                                          *string `access:"experimental_feature_flags,write_restrictable"` // telemetry: none
	FeatureFlagSyncIntervalSeconds                    *int    `access:"experimental_feature_flags,write_restrictable"` // telemetry: none
	DebugSplit                                        *bool   `access:"experimental_feature_flags,write_restrictable"` // telemetry: none
	ThreadAutoFollow                                  *bool   `access:"experimental_features"`
	CollapsedThreads                                  *string `access:"experimental_features"`
	ManagedResourcePaths                              *string `access:"environment_web_server,write_restrictable,cloud_restrictable"`
	EnableLegacySidebar                               *bool   `access:"experimental_features"`
	EnableReliableWebSockets                          *bool   `access:"experimental_features"` // telemetry: none
}

func (s *ServiceSettings) SetDefaults(isUpdate bool) {
	if s.EnableEmailInvitations == nil {
		// If the site URL is also not present then assume this is a clean install
		if s.SiteURL == nil {
			s.EnableEmailInvitations = NewBool(false)
		} else {
			s.EnableEmailInvitations = NewBool(true)
		}
	}

	if s.SiteURL == nil {
		if s.EnableDeveloper != nil && *s.EnableDeveloper {
			s.SiteURL = NewString(ServiceSettingsDefaultSiteURL)
		} else {
			s.SiteURL = NewString("")
		}
	}

	if s.WebsocketURL == nil {
		s.WebsocketURL = NewString("")
	}

	if s.LicenseFileLocation == nil {
		s.LicenseFileLocation = NewString("")
	}

	if s.ListenAddress == nil {
		s.ListenAddress = NewString(ServiceSettingsDefaultListenAndAddress)
	}

	if s.EnableLinkPreviews == nil {
		s.EnableLinkPreviews = NewBool(true)
	}

	if s.RestrictLinkPreviews == nil {
		s.RestrictLinkPreviews = NewString("")
	}

	if s.EnableTesting == nil {
		s.EnableTesting = NewBool(false)
	}

	if s.EnableDeveloper == nil {
		s.EnableDeveloper = NewBool(false)
	}

	if s.EnableOpenTracing == nil {
		s.EnableOpenTracing = NewBool(false)
	}

	if s.EnableSecurityFixAlert == nil {
		s.EnableSecurityFixAlert = NewBool(true)
	}

	if s.EnableInsecureOutgoingConnections == nil {
		s.EnableInsecureOutgoingConnections = NewBool(false)
	}

	if s.AllowedUntrustedInternalConnections == nil {
		s.AllowedUntrustedInternalConnections = NewString("")
	}

	if s.EnableMultifactorAuthentication == nil {
		s.EnableMultifactorAuthentication = NewBool(false)
	}

	if s.EnforceMultifactorAuthentication == nil {
		s.EnforceMultifactorAuthentication = NewBool(false)
	}

	if s.EnableUserAccessTokens == nil {
		s.EnableUserAccessTokens = NewBool(false)
	}

	if s.GoroutineHealthThreshold == nil {
		s.GoroutineHealthThreshold = NewInt(-1)
	}

	if s.GoogleDeveloperKey == nil {
		s.GoogleDeveloperKey = NewString("")
	}

	if s.EnableOAuthServiceProvider == nil {
		s.EnableOAuthServiceProvider = NewBool(false)
	}

	if s.EnableIncomingWebhooks == nil {
		s.EnableIncomingWebhooks = NewBool(true)
	}

	if s.EnableOutgoingWebhooks == nil {
		s.EnableOutgoingWebhooks = NewBool(true)
	}

	if s.ConnectionSecurity == nil {
		s.ConnectionSecurity = NewString("")
	}

	if s.TLSKeyFile == nil {
		s.TLSKeyFile = NewString(ServiceSettingsDefaultTlsKeyFile)
	}

	if s.TLSCertFile == nil {
		s.TLSCertFile = NewString(ServiceSettingsDefaultTlsCertFile)
	}

	if s.TLSMinVer == nil {
		s.TLSMinVer = NewString("1.2")
	}

	if s.TLSStrictTransport == nil {
		s.TLSStrictTransport = NewBool(false)
	}

	if s.TLSStrictTransportMaxAge == nil {
		s.TLSStrictTransportMaxAge = NewInt64(63072000)
	}

	if s.TLSOverwriteCiphers == nil {
		s.TLSOverwriteCiphers = []string{}
	}

	if s.UseLetsEncrypt == nil {
		s.UseLetsEncrypt = NewBool(false)
	}

	if s.LetsEncryptCertificateCacheFile == nil {
		s.LetsEncryptCertificateCacheFile = NewString("./config/letsencrypt.cache")
	}

	if s.ReadTimeout == nil {
		s.ReadTimeout = NewInt(ServiceSettingsDefaultReadTimeout)
	}

	if s.WriteTimeout == nil {
		s.WriteTimeout = NewInt(ServiceSettingsDefaultWriteTimeout)
	}

	if s.IdleTimeout == nil {
		s.IdleTimeout = NewInt(ServiceSettingsDefaultIdleTimeout)
	}

	if s.MaximumLoginAttempts == nil {
		s.MaximumLoginAttempts = NewInt(ServiceSettingsDefaultMaxLoginAttempts)
	}

	if s.Forward80To443 == nil {
		s.Forward80To443 = NewBool(false)
	}

	if isUpdate {
		// When updating an existing configuration, ensure that defaults are set.
		if s.TrustedProxyIPHeader == nil {
			s.TrustedProxyIPHeader = []string{HeaderForwarded, HeaderRealID}
		}
	} else {
		// When generating a blank configuration, leave the list empty.
		s.TrustedProxyIPHeader = []string{}
	}

	if s.TimeBetweenUserTypingUpdatesMilliseconds == nil {
		s.TimeBetweenUserTypingUpdatesMilliseconds = NewInt64(5000)
	}

	if s.EnablePostSearch == nil {
		s.EnablePostSearch = NewBool(true)
	}

	if s.EnableFileSearch == nil {
		s.EnableFileSearch = NewBool(true)
	}

	if s.MinimumHashtagLength == nil {
		s.MinimumHashtagLength = NewInt(3)
	}

	if s.EnableUserTypingMessages == nil {
		s.EnableUserTypingMessages = NewBool(true)
	}

	if s.EnableChannelViewedMessages == nil {
		s.EnableChannelViewedMessages = NewBool(true)
	}

	if s.EnableUserStatuses == nil {
		s.EnableUserStatuses = NewBool(true)
	}

	if s.ClusterLogTimeoutMilliseconds == nil {
		s.ClusterLogTimeoutMilliseconds = NewInt(2000)
	}

	if s.CloseUnusedDirectMessages == nil {
		s.CloseUnusedDirectMessages = NewBool(false)
	}

	if s.EnableTutorial == nil {
		s.EnableTutorial = NewBool(true)
	}

	// Must be manually enabled for existing installations.
	if s.ExtendSessionLengthWithActivity == nil {
		s.ExtendSessionLengthWithActivity = NewBool(!isUpdate)
	}

	if s.SessionLengthWebInDays == nil {
		if isUpdate {
			s.SessionLengthWebInDays = NewInt(180)
		} else {
			s.SessionLengthWebInDays = NewInt(30)
		}
	}

	if s.SessionLengthMobileInDays == nil {
		if isUpdate {
			s.SessionLengthMobileInDays = NewInt(180)
		} else {
			s.SessionLengthMobileInDays = NewInt(30)
		}
	}

	if s.SessionLengthSSOInDays == nil {
		s.SessionLengthSSOInDays = NewInt(30)
	}

	if s.SessionCacheInMinutes == nil {
		s.SessionCacheInMinutes = NewInt(10)
	}

	if s.SessionIdleTimeoutInMinutes == nil {
		s.SessionIdleTimeoutInMinutes = NewInt(43200)
	}

	if s.EnableCommands == nil {
		s.EnableCommands = NewBool(true)
	}

	if s.DEPRECATED_DO_NOT_USE_EnableOnlyAdminIntegrations == nil {
		s.DEPRECATED_DO_NOT_USE_EnableOnlyAdminIntegrations = NewBool(true)
	}

	if s.EnablePostUsernameOverride == nil {
		s.EnablePostUsernameOverride = NewBool(false)
	}

	if s.EnablePostIconOverride == nil {
		s.EnablePostIconOverride = NewBool(false)
	}

	if s.WebsocketPort == nil {
		s.WebsocketPort = NewInt(80)
	}

	if s.WebsocketSecurePort == nil {
		s.WebsocketSecurePort = NewInt(443)
	}

	if s.AllowCorsFrom == nil {
		s.AllowCorsFrom = NewString(ServiceSettingsDefaultAllowCorsFrom)
	}

	if s.CorsExposedHeaders == nil {
		s.CorsExposedHeaders = NewString("")
	}

	if s.CorsAllowCredentials == nil {
		s.CorsAllowCredentials = NewBool(false)
	}

	if s.CorsDebug == nil {
		s.CorsDebug = NewBool(false)
	}

	if s.AllowCookiesForSubdomains == nil {
		s.AllowCookiesForSubdomains = NewBool(false)
	}

	if s.WebserverMode == nil {
		s.WebserverMode = NewString("gzip")
	} else if *s.WebserverMode == "regular" {
		*s.WebserverMode = "gzip"
	}

	if s.EnableCustomEmoji == nil {
		s.EnableCustomEmoji = NewBool(true)
	}

	if s.EnableEmojiPicker == nil {
		s.EnableEmojiPicker = NewBool(true)
	}

	if s.EnableGifPicker == nil {
		s.EnableGifPicker = NewBool(true)
	}

	if s.GfycatAPIKey == nil || *s.GfycatAPIKey == "" {
		s.GfycatAPIKey = NewString(ServiceSettingsDefaultGfycatAPIKey)
	}

	if s.GfycatAPISecret == nil || *s.GfycatAPISecret == "" {
		s.GfycatAPISecret = NewString(ServiceSettingsDefaultGfycatAPISecret)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation = NewString(RestrictEmojiCreationAll)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPostDelete == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictPostDelete = NewString(PermissionsDeletePostAll)
	}

	if s.DEPRECATED_DO_NOT_USE_AllowEditPost == nil {
		s.DEPRECATED_DO_NOT_USE_AllowEditPost = NewString(AllowEditPostAlways)
	}

	if s.ExperimentalEnableAuthenticationTransfer == nil {
		s.ExperimentalEnableAuthenticationTransfer = NewBool(true)
	}

	if s.PostEditTimeLimit == nil {
		s.PostEditTimeLimit = NewInt(-1)
	}

	if s.EnablePreviewFeatures == nil {
		s.EnablePreviewFeatures = NewBool(true)
	}

	if s.ExperimentalEnableDefaultChannelLeaveJoinMessages == nil {
		s.ExperimentalEnableDefaultChannelLeaveJoinMessages = NewBool(true)
	}

	if s.ExperimentalGroupUnreadChannels == nil {
		s.ExperimentalGroupUnreadChannels = NewString(GroupUnreadChannelsDisabled)
	} else if *s.ExperimentalGroupUnreadChannels == "0" {
		s.ExperimentalGroupUnreadChannels = NewString(GroupUnreadChannelsDisabled)
	} else if *s.ExperimentalGroupUnreadChannels == "1" {
		s.ExperimentalGroupUnreadChannels = NewString(GroupUnreadChannelsDefaultOn)
	}

	if s.ExperimentalChannelOrganization == nil {
		experimentalUnreadEnabled := *s.ExperimentalGroupUnreadChannels != GroupUnreadChannelsDisabled
		s.ExperimentalChannelOrganization = NewBool(experimentalUnreadEnabled)
	}

	if s.DEPRECATED_DO_NOT_USE_ImageProxyType == nil {
		s.DEPRECATED_DO_NOT_USE_ImageProxyType = NewString("")
	}

	if s.DEPRECATED_DO_NOT_USE_ImageProxyURL == nil {
		s.DEPRECATED_DO_NOT_USE_ImageProxyURL = NewString("")
	}

	if s.DEPRECATED_DO_NOT_USE_ImageProxyOptions == nil {
		s.DEPRECATED_DO_NOT_USE_ImageProxyOptions = NewString("")
	}

	if s.EnableAPITeamDeletion == nil {
		s.EnableAPITeamDeletion = NewBool(false)
	}

	if s.EnableAPIUserDeletion == nil {
		s.EnableAPIUserDeletion = NewBool(false)
	}

	if s.EnableAPIChannelDeletion == nil {
		s.EnableAPIChannelDeletion = NewBool(false)
	}

	if s.ExperimentalEnableHardenedMode == nil {
		s.ExperimentalEnableHardenedMode = NewBool(false)
	}

	if s.DisableLegacyMFA == nil {
		s.DisableLegacyMFA = NewBool(!isUpdate)
	}

	if s.ExperimentalStrictCSRFEnforcement == nil {
		s.ExperimentalStrictCSRFEnforcement = NewBool(false)
	}

	if s.DisableBotsWhenOwnerIsDeactivated == nil {
		s.DisableBotsWhenOwnerIsDeactivated = NewBool(true)
	}

	if s.EnableBotAccountCreation == nil {
		s.EnableBotAccountCreation = NewBool(false)
	}

	if s.EnableSVGs == nil {
		if isUpdate {
			s.EnableSVGs = NewBool(true)
		} else {
			s.EnableSVGs = NewBool(false)
		}
	}

	if s.EnableLatex == nil {
		if isUpdate {
			s.EnableLatex = NewBool(true)
		} else {
			s.EnableLatex = NewBool(false)
		}
	}

	if s.EnableLocalMode == nil {
		s.EnableLocalMode = NewBool(false)
	}

	if s.LocalModeSocketLocation == nil {
		s.LocalModeSocketLocation = NewString(LocalModeSocketPath)
	}

	if s.EnableAWSMetering == nil {
		s.EnableAWSMetering = NewBool(false)
	}

	if s.SplitKey == nil {
		s.SplitKey = NewString("")
	}

	if s.FeatureFlagSyncIntervalSeconds == nil {
		s.FeatureFlagSyncIntervalSeconds = NewInt(30)
	}

	if s.DebugSplit == nil {
		s.DebugSplit = NewBool(false)
	}

	if s.ThreadAutoFollow == nil {
		s.ThreadAutoFollow = NewBool(true)
	}

	if s.CollapsedThreads == nil {
		s.CollapsedThreads = NewString(CollapsedThreadsDisabled)
	}

	if s.ManagedResourcePaths == nil {
		s.ManagedResourcePaths = NewString("")
	}

	if s.EnableLegacySidebar == nil {
		s.EnableLegacySidebar = NewBool(false)
	}

	if s.EnableReliableWebSockets == nil {
		s.EnableReliableWebSockets = NewBool(false)
	}
}

type ClusterSettings struct {
	Enable                                      *bool   `access:"environment_high_availability,write_restrictable"`
	ClusterName                                 *string `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	OverrideHostname                            *string `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	NetworkInterface                            *string `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	BindAddress                                 *string `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	AdvertiseAddress                            *string `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	UseIDAddress                                *bool   `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	DEPRECATED_DO_NOT_USE_UseExperimentalGossip *bool   `json:"UseExperimentalGossip" access:"environment_high_availability,write_restrictable,cloud_restrictable"` // Deprecated: do not use
	EnableGossipCompression                     *bool   `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	EnableExperimentalGossipEncryption          *bool   `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	ReadOnlyConfig                              *bool   `access:"environment_high_availability,write_restrictable,cloud_restrictable"`
	GossipPort                                  *int    `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	StreamingPort                               *int    `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	MaxIdleConns                                *int    `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	MaxIdleConnsPerHost                         *int    `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
	IdleConnTimeoutMilliseconds                 *int    `access:"environment_high_availability,write_restrictable,cloud_restrictable"` // telemetry: none
}

func (s *ClusterSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.ClusterName == nil {
		s.ClusterName = NewString("")
	}

	if s.OverrideHostname == nil {
		s.OverrideHostname = NewString("")
	}

	if s.NetworkInterface == nil {
		s.NetworkInterface = NewString("")
	}

	if s.BindAddress == nil {
		s.BindAddress = NewString("")
	}

	if s.AdvertiseAddress == nil {
		s.AdvertiseAddress = NewString("")
	}

	if s.UseIDAddress == nil {
		s.UseIDAddress = NewBool(true)
	}

	if s.DEPRECATED_DO_NOT_USE_UseExperimentalGossip == nil {
		s.DEPRECATED_DO_NOT_USE_UseExperimentalGossip = NewBool(true)
	}

	if s.EnableExperimentalGossipEncryption == nil {
		s.EnableExperimentalGossipEncryption = NewBool(false)
	}

	if s.EnableGossipCompression == nil {
		s.EnableGossipCompression = NewBool(true)
	}

	if s.ReadOnlyConfig == nil {
		s.ReadOnlyConfig = NewBool(true)
	}

	if s.GossipPort == nil {
		s.GossipPort = NewInt(8074)
	}

	if s.StreamingPort == nil {
		s.StreamingPort = NewInt(8075)
	}

	if s.MaxIdleConns == nil {
		s.MaxIdleConns = NewInt(100)
	}

	if s.MaxIdleConnsPerHost == nil {
		s.MaxIdleConnsPerHost = NewInt(128)
	}

	if s.IdleConnTimeoutMilliseconds == nil {
		s.IdleConnTimeoutMilliseconds = NewInt(90000)
	}
}

type MetricsSettings struct {
	Enable           *bool   `access:"environment_performance_monitoring,write_restrictable,cloud_restrictable"`
	BlockProfileRate *int    `access:"environment_performance_monitoring,write_restrictable,cloud_restrictable"`
	ListenAddress    *string `access:"environment_performance_monitoring,write_restrictable,cloud_restrictable"` // telemetry: none
}

func (s *MetricsSettings) SetDefaults() {
	if s.ListenAddress == nil {
		s.ListenAddress = NewString(":8067")
	}

	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.BlockProfileRate == nil {
		s.BlockProfileRate = NewInt(0)
	}
}

type ExperimentalSettings struct {
	ClientSideCertEnable            *bool   `access:"experimental_features,cloud_restrictable"`
	ClientSideCertCheck             *string `access:"experimental_features,cloud_restrictable"`
	EnableClickToReply              *bool   `access:"experimental_features,write_restrictable,cloud_restrictable"`
	LinkMetadataTimeoutMilliseconds *int64  `access:"experimental_features,write_restrictable,cloud_restrictable"`
	RestrictSystemAdmin             *bool   `access:"experimental_features,write_restrictable"`
	UseNewSAMLLibrary               *bool   `access:"experimental_features,cloud_restrictable"`
	CloudUserLimit                  *int64  `access:"experimental_features,write_restrictable"`
	CloudBilling                    *bool   `access:"experimental_features,write_restrictable"`
	EnableSharedChannels            *bool   `access:"experimental_features"`
	EnableRemoteClusterService      *bool   `access:"experimental_features"`
}

func (s *ExperimentalSettings) SetDefaults() {
	if s.ClientSideCertEnable == nil {
		s.ClientSideCertEnable = NewBool(false)
	}

	if s.ClientSideCertCheck == nil {
		s.ClientSideCertCheck = NewString(ClientSideCertCheckSecondaryAuth)
	}

	if s.EnableClickToReply == nil {
		s.EnableClickToReply = NewBool(false)
	}

	if s.LinkMetadataTimeoutMilliseconds == nil {
		s.LinkMetadataTimeoutMilliseconds = NewInt64(ExperimentalSettingsDefaultLinkMetadataTimeoutMilliseconds)
	}

	if s.RestrictSystemAdmin == nil {
		s.RestrictSystemAdmin = NewBool(false)
	}

	if s.CloudUserLimit == nil {
		// User limit 0 is treated as no limit
		s.CloudUserLimit = NewInt64(0)
	}

	if s.CloudBilling == nil {
		s.CloudBilling = NewBool(false)
	}

	if s.UseNewSAMLLibrary == nil {
		s.UseNewSAMLLibrary = NewBool(false)
	}

	if s.EnableSharedChannels == nil {
		s.EnableSharedChannels = NewBool(false)
	}

	if s.EnableRemoteClusterService == nil {
		s.EnableRemoteClusterService = NewBool(false)
	}
}

type AnalyticsSettings struct {
	MaxUsersForStatistics *int `access:"write_restrictable,cloud_restrictable"`
}

func (s *AnalyticsSettings) SetDefaults() {
	if s.MaxUsersForStatistics == nil {
		s.MaxUsersForStatistics = NewInt(AnalyticsSettingsDefaultMaxUsersForStatistics)
	}
}

type SSOSettings struct {
	Enable            *bool   `access:"authentication_openid"`
	Secret            *string `access:"authentication_openid"` // telemetry: none
	ID                *string `access:"authentication_openid"` // telemetry: none
	Scope             *string `access:"authentication_openid"` // telemetry: none
	AuthEndpoint      *string `access:"authentication_openid"` // telemetry: none
	TokenEndpoint     *string `access:"authentication_openid"` // telemetry: none
	UserAPIEndpoint   *string `access:"authentication_openid"` // telemetry: none
	DiscoveryEndpoint *string `access:"authentication_openid"` // telemetry: none
	ButtonText        *string `access:"authentication_openid"` // telemetry: none
	ButtonColor       *string `access:"authentication_openid"` // telemetry: none
}

func (s *SSOSettings) setDefaults(scope, authEndpoint, tokenEndpoint, userAPIEndpoint, buttonColor string) {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.Secret == nil {
		s.Secret = NewString("")
	}

	if s.ID == nil {
		s.ID = NewString("")
	}

	if s.Scope == nil {
		s.Scope = NewString(scope)
	}

	if s.DiscoveryEndpoint == nil {
		s.DiscoveryEndpoint = NewString("")
	}

	if s.AuthEndpoint == nil {
		s.AuthEndpoint = NewString(authEndpoint)
	}

	if s.TokenEndpoint == nil {
		s.TokenEndpoint = NewString(tokenEndpoint)
	}

	if s.UserAPIEndpoint == nil {
		s.UserAPIEndpoint = NewString(userAPIEndpoint)
	}

	if s.ButtonText == nil {
		s.ButtonText = NewString("")
	}

	if s.ButtonColor == nil {
		s.ButtonColor = NewString(buttonColor)
	}
}

type Office365Settings struct {
	Enable            *bool   `access:"authentication_openid"`
	Secret            *string `access:"authentication_openid"` // telemetry: none
	ID                *string `access:"authentication_openid"` // telemetry: none
	Scope             *string `access:"authentication_openid"`
	AuthEndpoint      *string `access:"authentication_openid"` // telemetry: none
	TokenEndpoint     *string `access:"authentication_openid"` // telemetry: none
	UserAPIEndpoint   *string `access:"authentication_openid"` // telemetry: none
	DiscoveryEndpoint *string `access:"authentication_openid"` // telemetry: none
	DirectoryID       *string `access:"authentication_openid"` // telemetry: none
}

func (s *Office365Settings) setDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.ID == nil {
		s.ID = NewString("")
	}

	if s.Secret == nil {
		s.Secret = NewString("")
	}

	if s.Scope == nil {
		s.Scope = NewString(Office365SettingsDefaultScope)
	}

	if s.DiscoveryEndpoint == nil {
		s.DiscoveryEndpoint = NewString("")
	}

	if s.AuthEndpoint == nil {
		s.AuthEndpoint = NewString(Office365SettingsDefaultAuthEndpoint)
	}

	if s.TokenEndpoint == nil {
		s.TokenEndpoint = NewString(Office365SettingsDefaultTokenEndpoint)
	}

	if s.UserAPIEndpoint == nil {
		s.UserAPIEndpoint = NewString(Office365SettingsDefaultUserAPIEndpoint)
	}

	if s.DirectoryID == nil {
		s.DirectoryID = NewString("")
	}
}

func (s *Office365Settings) SSOSettings() *SSOSettings {
	ssoSettings := SSOSettings{}
	ssoSettings.Enable = s.Enable
	ssoSettings.Secret = s.Secret
	ssoSettings.ID = s.ID
	ssoSettings.Scope = s.Scope
	ssoSettings.DiscoveryEndpoint = s.DiscoveryEndpoint
	ssoSettings.AuthEndpoint = s.AuthEndpoint
	ssoSettings.TokenEndpoint = s.TokenEndpoint
	ssoSettings.UserAPIEndpoint = s.UserAPIEndpoint
	return &ssoSettings
}

type ReplicaLagSettings struct {
	DataSource       *string `access:"environment,write_restrictable,cloud_restrictable"` // telemetry: none
	QueryAbsoluteLag *string `access:"environment,write_restrictable,cloud_restrictable"` // telemetry: none
	QueryTimeLag     *string `access:"environment,write_restrictable,cloud_restrictable"` // telemetry: none
}

type SQLSettings struct {
	DriverName                  *string               `access:"environment_database,write_restrictable,cloud_restrictable"`
	DataSource                  *string               `access:"environment_database,write_restrictable,cloud_restrictable"` // telemetry: none
	DataSourceReplicas          []string              `access:"environment_database,write_restrictable,cloud_restrictable"`
	DataSourceSearchReplicas    []string              `access:"environment_database,write_restrictable,cloud_restrictable"`
	MaxIdleConns                *int                  `access:"environment_database,write_restrictable,cloud_restrictable"`
	ConnMaxLifetimeMilliseconds *int                  `access:"environment_database,write_restrictable,cloud_restrictable"`
	ConnMaxIdleTimeMilliseconds *int                  `access:"environment_database,write_restrictable,cloud_restrictable"`
	MaxOpenConns                *int                  `access:"environment_database,write_restrictable,cloud_restrictable"`
	Trace                       *bool                 `access:"environment_database,write_restrictable,cloud_restrictable"`
	AtRestEncryptKey            *string               `access:"environment_database,write_restrictable,cloud_restrictable"` // telemetry: none
	QueryTimeout                *int                  `access:"environment_database,write_restrictable,cloud_restrictable"`
	DisableDatabaseSearch       *bool                 `access:"environment_database,write_restrictable,cloud_restrictable"`
	ReplicaLagSettings          []*ReplicaLagSettings `access:"environment_database,write_restrictable,cloud_restrictable"` // telemetry: none
}

func (s *SQLSettings) SetDefaults(isUpdate bool) {
	if s.DriverName == nil {
		s.DriverName = NewString(DatabaseDriverPostgres)
	}

	if s.DataSource == nil {
		s.DataSource = NewString(SQLSettingsDefaultDataSource)
	}

	if s.DataSourceReplicas == nil {
		s.DataSourceReplicas = []string{}
	}

	if s.DataSourceSearchReplicas == nil {
		s.DataSourceSearchReplicas = []string{}
	}

	if isUpdate {
		// When updating an existing configuration, ensure an encryption key has been specified.
		if s.AtRestEncryptKey == nil || *s.AtRestEncryptKey == "" {
			s.AtRestEncryptKey = NewString(NewRandomString(32))
		}
	} else {
		// When generating a blank configuration, leave this key empty to be generated on server start.
		s.AtRestEncryptKey = NewString("")
	}

	if s.MaxIdleConns == nil {
		s.MaxIdleConns = NewInt(20)
	}

	if s.MaxOpenConns == nil {
		s.MaxOpenConns = NewInt(300)
	}

	if s.ConnMaxLifetimeMilliseconds == nil {
		s.ConnMaxLifetimeMilliseconds = NewInt(3600000)
	}

	if s.ConnMaxIdleTimeMilliseconds == nil {
		s.ConnMaxIdleTimeMilliseconds = NewInt(300000)
	}

	if s.Trace == nil {
		s.Trace = NewBool(false)
	}

	if s.QueryTimeout == nil {
		s.QueryTimeout = NewInt(30)
	}

	if s.DisableDatabaseSearch == nil {
		s.DisableDatabaseSearch = NewBool(false)
	}

	if s.ReplicaLagSettings == nil {
		s.ReplicaLagSettings = []*ReplicaLagSettings{}
	}
}

type LogSettings struct {
	EnableConsole          *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"`
	ConsoleLevel           *string `access:"environment_logging,write_restrictable,cloud_restrictable"`
	ConsoleJSON            *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"`
	EnableColor            *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"` // telemetry: none
	EnableFile             *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"`
	FileLevel              *string `access:"environment_logging,write_restrictable,cloud_restrictable"`
	FileJSON               *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"`
	FileLocation           *string `access:"environment_logging,write_restrictable,cloud_restrictable"`
	EnableWebhookDebugging *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"`
	EnableDiagnostics      *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"` // telemetry: none
	EnableSentry           *bool   `access:"environment_logging,write_restrictable,cloud_restrictable"` // telemetry: none
	AdvancedLoggingConfig  *string `access:"environment_logging,write_restrictable,cloud_restrictable"`
}

func (s *LogSettings) SetDefaults() {
	if s.EnableConsole == nil {
		s.EnableConsole = NewBool(true)
	}

	if s.ConsoleLevel == nil {
		s.ConsoleLevel = NewString("DEBUG")
	}

	if s.EnableColor == nil {
		s.EnableColor = NewBool(false)
	}

	if s.EnableFile == nil {
		s.EnableFile = NewBool(true)
	}

	if s.FileLevel == nil {
		s.FileLevel = NewString("INFO")
	}

	if s.FileLocation == nil {
		s.FileLocation = NewString("")
	}

	if s.EnableWebhookDebugging == nil {
		s.EnableWebhookDebugging = NewBool(true)
	}

	if s.EnableDiagnostics == nil {
		s.EnableDiagnostics = NewBool(true)
	}

	if s.EnableSentry == nil {
		s.EnableSentry = NewBool(*s.EnableDiagnostics)
	}

	if s.ConsoleJSON == nil {
		s.ConsoleJSON = NewBool(true)
	}

	if s.FileJSON == nil {
		s.FileJSON = NewBool(true)
	}

	if s.AdvancedLoggingConfig == nil {
		s.AdvancedLoggingConfig = NewString("")
	}
}

type ExperimentalAuditSettings struct {
	FileEnabled           *bool   `access:"experimental_features,write_restrictable,cloud_restrictable"`
	FileName              *string `access:"experimental_features,write_restrictable,cloud_restrictable"` // telemetry: none
	FileMaxSizeMB         *int    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	FileMaxAgeDays        *int    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	FileMaxBackups        *int    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	FileCompress          *bool   `access:"experimental_features,write_restrictable,cloud_restrictable"`
	FileMaxQueueSize      *int    `access:"experimental_features,write_restrictable,cloud_restrictable"`
	AdvancedLoggingConfig *string `access:"experimental_features,write_restrictable,cloud_restrictable"`
}

func (s *ExperimentalAuditSettings) SetDefaults() {
	if s.FileEnabled == nil {
		s.FileEnabled = NewBool(false)
	}

	if s.FileName == nil {
		s.FileName = NewString("")
	}

	if s.FileMaxSizeMB == nil {
		s.FileMaxSizeMB = NewInt(100)
	}

	if s.FileMaxAgeDays == nil {
		s.FileMaxAgeDays = NewInt(0) // no limit on age
	}

	if s.FileMaxBackups == nil { // no limit on number of backups
		s.FileMaxBackups = NewInt(0)
	}

	if s.FileCompress == nil {
		s.FileCompress = NewBool(false)
	}

	if s.FileMaxQueueSize == nil {
		s.FileMaxQueueSize = NewInt(1000)
	}

	if s.AdvancedLoggingConfig == nil {
		s.AdvancedLoggingConfig = NewString("")
	}
}

type NotificationLogSettings struct {
	EnableConsole         *bool   `access:"write_restrictable,cloud_restrictable"`
	ConsoleLevel          *string `access:"write_restrictable,cloud_restrictable"`
	ConsoleJSON           *bool   `access:"write_restrictable,cloud_restrictable"`
	EnableColor           *bool   `access:"write_restrictable,cloud_restrictable"` // telemetry: none
	EnableFile            *bool   `access:"write_restrictable,cloud_restrictable"`
	FileLevel             *string `access:"write_restrictable,cloud_restrictable"`
	FileJSON              *bool   `access:"write_restrictable,cloud_restrictable"`
	FileLocation          *string `access:"write_restrictable,cloud_restrictable"`
	AdvancedLoggingConfig *string `access:"write_restrictable,cloud_restrictable"`
}

func (s *NotificationLogSettings) SetDefaults() {
	if s.EnableConsole == nil {
		s.EnableConsole = NewBool(true)
	}

	if s.ConsoleLevel == nil {
		s.ConsoleLevel = NewString("DEBUG")
	}

	if s.EnableFile == nil {
		s.EnableFile = NewBool(true)
	}

	if s.FileLevel == nil {
		s.FileLevel = NewString("INFO")
	}

	if s.FileLocation == nil {
		s.FileLocation = NewString("")
	}

	if s.ConsoleJSON == nil {
		s.ConsoleJSON = NewBool(true)
	}

	if s.EnableColor == nil {
		s.EnableColor = NewBool(false)
	}

	if s.FileJSON == nil {
		s.FileJSON = NewBool(true)
	}

	if s.AdvancedLoggingConfig == nil {
		s.AdvancedLoggingConfig = NewString("")
	}
}

type PasswordSettings struct {
	MinimumLength *int  `access:"authentication_password"`
	Lowercase     *bool `access:"authentication_password"`
	Number        *bool `access:"authentication_password"`
	Uppercase     *bool `access:"authentication_password"`
	Symbol        *bool `access:"authentication_password"`
}

func (s *PasswordSettings) SetDefaults() {
	if s.MinimumLength == nil {
		s.MinimumLength = NewInt(10)
	}

	if s.Lowercase == nil {
		s.Lowercase = NewBool(true)
	}

	if s.Number == nil {
		s.Number = NewBool(true)
	}

	if s.Uppercase == nil {
		s.Uppercase = NewBool(true)
	}

	if s.Symbol == nil {
		s.Symbol = NewBool(true)
	}
}

type FileSettings struct {
	EnableFileAttachments   *bool   `access:"site_file_sharing_and_downloads,cloud_restrictable"`
	EnableMobileUpload      *bool   `access:"site_file_sharing_and_downloads,cloud_restrictable"`
	EnableMobileDownload    *bool   `access:"site_file_sharing_and_downloads,cloud_restrictable"`
	MaxFileSize             *int64  `access:"environment_file_storage,cloud_restrictable"`
	DriverName              *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
	Directory               *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
	EnablePublicLink        *bool   `access:"site_public_links,cloud_restrictable"`
	ExtractContent          *bool   `access:"environment_file_storage,write_restrictable"`
	ArchiveRecursion        *bool   `access:"environment_file_storage,write_restrictable"`
	PublicLinkSalt          *string `access:"site_public_links,cloud_restrictable"`                           // telemetry: none
	InitialFont             *string `access:"environment_file_storage,cloud_restrictable"`                    // telemetry: none
	AmazonS3AccessKeyID     *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3SecretAccessKey *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3Bucket          *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3PathPrefix      *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3Region          *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3Endpoint        *string `access:"environment_file_storage,write_restrictable,cloud_restrictable"` // telemetry: none
	AmazonS3SSL             *bool   `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
	AmazonS3SignV2          *bool   `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
	AmazonS3SSE             *bool   `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
	AmazonS3Trace           *bool   `access:"environment_file_storage,write_restrictable,cloud_restrictable"`
}

func (s *FileSettings) SetDefaults(isUpdate bool) {
	if s.EnableFileAttachments == nil {
		s.EnableFileAttachments = NewBool(true)
	}

	if s.EnableMobileUpload == nil {
		s.EnableMobileUpload = NewBool(true)
	}

	if s.EnableMobileDownload == nil {
		s.EnableMobileDownload = NewBool(true)
	}

	if s.MaxFileSize == nil {
		s.MaxFileSize = NewInt64(MB * 100)
	}

	if s.DriverName == nil {
		s.DriverName = NewString(ImageDriverLocal)
	}

	if s.Directory == nil || *s.Directory == "" {
		s.Directory = NewString(FileSettingsDefaultDirectory)
	}

	if s.EnablePublicLink == nil {
		s.EnablePublicLink = NewBool(false)
	}

	if s.ExtractContent == nil {
		s.ExtractContent = NewBool(true)
	}

	if s.ArchiveRecursion == nil {
		s.ArchiveRecursion = NewBool(false)
	}

	if isUpdate {
		// When updating an existing configuration, ensure link salt has been specified.
		if s.PublicLinkSalt == nil || *s.PublicLinkSalt == "" {
			s.PublicLinkSalt = NewString(NewRandomString(32))
		}
	} else {
		// When generating a blank configuration, leave link salt empty to be generated on server start.
		s.PublicLinkSalt = NewString("")
	}

	if s.InitialFont == nil {
		// Defaults to "nunito-bold.ttf"
		s.InitialFont = NewString("nunito-bold.ttf")
	}

	if s.AmazonS3AccessKeyID == nil {
		s.AmazonS3AccessKeyID = NewString("")
	}

	if s.AmazonS3SecretAccessKey == nil {
		s.AmazonS3SecretAccessKey = NewString("")
	}

	if s.AmazonS3Bucket == nil {
		s.AmazonS3Bucket = NewString("")
	}

	if s.AmazonS3PathPrefix == nil {
		s.AmazonS3PathPrefix = NewString("")
	}

	if s.AmazonS3Region == nil {
		s.AmazonS3Region = NewString("")
	}

	if s.AmazonS3Endpoint == nil || *s.AmazonS3Endpoint == "" {
		// Defaults to "s3.amazonaws.com"
		s.AmazonS3Endpoint = NewString("s3.amazonaws.com")
	}

	if s.AmazonS3SSL == nil {
		s.AmazonS3SSL = NewBool(true) // Secure by default.
	}

	if s.AmazonS3SignV2 == nil {
		s.AmazonS3SignV2 = new(bool)
		// Signature v2 is not enabled by default.
	}

	if s.AmazonS3SSE == nil {
		s.AmazonS3SSE = NewBool(false) // Not Encrypted by default.
	}

	if s.AmazonS3Trace == nil {
		s.AmazonS3Trace = NewBool(false)
	}
}

func (s *FileSettings) ToFileBackendSettings(enableComplianceFeature bool) filestore.FileBackendSettings {
	if *s.DriverName == ImageDriverLocal {
		return filestore.FileBackendSettings{
			DriverName: *s.DriverName,
			Directory:  *s.Directory,
		}
	}
	return filestore.FileBackendSettings{
		DriverName:              *s.DriverName,
		AmazonS3AccessKeyID:     *s.AmazonS3AccessKeyID,
		AmazonS3SecretAccessKey: *s.AmazonS3SecretAccessKey,
		AmazonS3Bucket:          *s.AmazonS3Bucket,
		AmazonS3PathPrefix:      *s.AmazonS3PathPrefix,
		AmazonS3Region:          *s.AmazonS3Region,
		AmazonS3Endpoint:        *s.AmazonS3Endpoint,
		AmazonS3SSL:             s.AmazonS3SSL == nil || *s.AmazonS3SSL,
		AmazonS3SignV2:          s.AmazonS3SignV2 != nil && *s.AmazonS3SignV2,
		AmazonS3SSE:             s.AmazonS3SSE != nil && *s.AmazonS3SSE && enableComplianceFeature,
		AmazonS3Trace:           s.AmazonS3Trace != nil && *s.AmazonS3Trace,
	}
}

type EmailSettings struct {
	EnableSignUpWithEmail             *bool   `access:"authentication_email"`
	EnableSignInWithEmail             *bool   `access:"authentication_email"`
	EnableSignInWithUsername          *bool   `access:"authentication_email"`
	SendEmailNotifications            *bool   `access:"site_notifications"`
	UseChannelInEmailNotifications    *bool   `access:"experimental_features"`
	RequireEmailVerification          *bool   `access:"authentication_email"`
	FeedbackName                      *string `access:"site_notifications"`
	FeedbackEmail                     *string `access:"site_notifications,cloud_restrictable"`
	ReplyToAddress                    *string `access:"site_notifications,cloud_restrictable"`
	FeedbackOrganization              *string `access:"site_notifications"`
	EnableSMTPAuth                    *bool   `access:"environment_smtp,write_restrictable,cloud_restrictable"`
	SMTPUsername                      *string `access:"environment_smtp,write_restrictable,cloud_restrictable"` // telemetry: none
	SMTPPassword                      *string `access:"environment_smtp,write_restrictable,cloud_restrictable"` // telemetry: none
	SMTPServer                        *string `access:"environment_smtp,write_restrictable,cloud_restrictable"` // telemetry: none
	SMTPPort                          *string `access:"environment_smtp,write_restrictable,cloud_restrictable"` // telemetry: none
	SMTPServerTimeout                 *int    `access:"cloud_restrictable"`
	ConnectionSecurity                *string `access:"environment_smtp,write_restrictable,cloud_restrictable"`
	SendPushNotifications             *bool   `access:"environment_push_notification_server"`
	PushNotificationServer            *string `access:"environment_push_notification_server"` // telemetry: none
	PushNotificationContents          *string `access:"site_notifications"`
	PushNotificationBuffer            *int    // telemetry: none
	EnableEmailBatching               *bool   `access:"site_notifications"`
	EmailBatchingBufferSize           *int    `access:"experimental_features"`
	EmailBatchingInterval             *int    `access:"experimental_features"`
	EnablePreviewModeBanner           *bool   `access:"site_notifications"`
	SkipServerCertificateVerification *bool   `access:"environment_smtp,write_restrictable,cloud_restrictable"`
	EmailNotificationContentsType     *string `access:"site_notifications"`
	LoginButtonColor                  *string `access:"experimental_features"`
	LoginButtonBorderColor            *string `access:"experimental_features"`
	LoginButtonTextColor              *string `access:"experimental_features"`
}

func (s *EmailSettings) SetDefaults(isUpdate bool) {
	if s.EnableSignUpWithEmail == nil {
		s.EnableSignUpWithEmail = NewBool(true)
	}

	if s.EnableSignInWithEmail == nil {
		s.EnableSignInWithEmail = NewBool(*s.EnableSignUpWithEmail)
	}

	if s.EnableSignInWithUsername == nil {
		s.EnableSignInWithUsername = NewBool(true)
	}

	if s.SendEmailNotifications == nil {
		s.SendEmailNotifications = NewBool(true)
	}

	if s.UseChannelInEmailNotifications == nil {
		s.UseChannelInEmailNotifications = NewBool(false)
	}

	if s.RequireEmailVerification == nil {
		s.RequireEmailVerification = NewBool(false)
	}

	if s.FeedbackName == nil {
		s.FeedbackName = NewString("")
	}

	if s.FeedbackEmail == nil {
		s.FeedbackEmail = NewString("test@example.com")
	}

	if s.ReplyToAddress == nil {
		s.ReplyToAddress = NewString("test@example.com")
	}

	if s.FeedbackOrganization == nil {
		s.FeedbackOrganization = NewString(EmailSettingsDefaultFeedbackOrganization)
	}

	if s.EnableSMTPAuth == nil {
		if s.ConnectionSecurity == nil || *s.ConnectionSecurity == ConnSecurityNone {
			s.EnableSMTPAuth = NewBool(false)
		} else {
			s.EnableSMTPAuth = NewBool(true)
		}
	}

	if s.SMTPUsername == nil {
		s.SMTPUsername = NewString("")
	}

	if s.SMTPPassword == nil {
		s.SMTPPassword = NewString("")
	}

	if s.SMTPServer == nil || *s.SMTPServer == "" {
		s.SMTPServer = NewString("localhost")
	}

	if s.SMTPPort == nil || *s.SMTPPort == "" {
		s.SMTPPort = NewString("10025")
	}

	if s.SMTPServerTimeout == nil || *s.SMTPServerTimeout == 0 {
		s.SMTPServerTimeout = NewInt(10)
	}

	if s.ConnectionSecurity == nil || *s.ConnectionSecurity == ConnSecurityPlain {
		s.ConnectionSecurity = NewString(ConnSecurityNone)
	}

	if s.SendPushNotifications == nil {
		s.SendPushNotifications = NewBool(!isUpdate)
	}

	if s.PushNotificationServer == nil {
		if isUpdate {
			s.PushNotificationServer = NewString("")
		} else {
			s.PushNotificationServer = NewString(GenericNotificationServer)
		}
	}

	if s.PushNotificationContents == nil {
		s.PushNotificationContents = NewString(FullNotification)
	}

	if s.PushNotificationBuffer == nil {
		s.PushNotificationBuffer = NewInt(1000)
	}

	if s.EnableEmailBatching == nil {
		s.EnableEmailBatching = NewBool(false)
	}

	if s.EmailBatchingBufferSize == nil {
		s.EmailBatchingBufferSize = NewInt(EmailBatchingBufferSize)
	}

	if s.EmailBatchingInterval == nil {
		s.EmailBatchingInterval = NewInt(EmailBatchingInterval)
	}

	if s.EnablePreviewModeBanner == nil {
		s.EnablePreviewModeBanner = NewBool(true)
	}

	if s.EnableSMTPAuth == nil {
		if *s.ConnectionSecurity == ConnSecurityNone {
			s.EnableSMTPAuth = NewBool(false)
		} else {
			s.EnableSMTPAuth = NewBool(true)
		}
	}

	if *s.ConnectionSecurity == ConnSecurityPlain {
		*s.ConnectionSecurity = ConnSecurityNone
	}

	if s.SkipServerCertificateVerification == nil {
		s.SkipServerCertificateVerification = NewBool(false)
	}

	if s.EmailNotificationContentsType == nil {
		s.EmailNotificationContentsType = NewString(EmailNotificationContentsFull)
	}

	if s.LoginButtonColor == nil {
		s.LoginButtonColor = NewString("#0000")
	}

	if s.LoginButtonBorderColor == nil {
		s.LoginButtonBorderColor = NewString("#2389D7")
	}

	if s.LoginButtonTextColor == nil {
		s.LoginButtonTextColor = NewString("#2389D7")
	}
}

type RateLimitSettings struct {
	Enable           *bool  `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	PerSec           *int   `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	MaxBurst         *int   `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	MemoryStoreSize  *int   `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	VaryByRemoteAddr *bool  `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	VaryByUser       *bool  `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
	VaryByHeader     string `access:"environment_rate_limiting,write_restrictable,cloud_restrictable"`
}

func (s *RateLimitSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.PerSec == nil {
		s.PerSec = NewInt(10)
	}

	if s.MaxBurst == nil {
		s.MaxBurst = NewInt(100)
	}

	if s.MemoryStoreSize == nil {
		s.MemoryStoreSize = NewInt(10000)
	}

	if s.VaryByRemoteAddr == nil {
		s.VaryByRemoteAddr = NewBool(true)
	}

	if s.VaryByUser == nil {
		s.VaryByUser = NewBool(false)
	}
}

type PrivacySettings struct {
	ShowEmailAddress *bool `access:"site_users_and_teams"`
	ShowFullName     *bool `access:"site_users_and_teams"`
}

func (s *PrivacySettings) setDefaults() {
	if s.ShowEmailAddress == nil {
		s.ShowEmailAddress = NewBool(true)
	}

	if s.ShowFullName == nil {
		s.ShowFullName = NewBool(true)
	}
}

type SupportSettings struct {
	TermsOfServiceLink                     *string `access:"site_customization,write_restrictable,cloud_restrictable"`
	PrivacyPolicyLink                      *string `access:"site_customization,write_restrictable,cloud_restrictable"`
	AboutLink                              *string `access:"site_customization,write_restrictable,cloud_restrictable"`
	HelpLink                               *string `access:"site_customization,write_restrictable,cloud_restrictable"`
	ReportAProblemLink                     *string `access:"site_customization,write_restrictable,cloud_restrictable"`
	SupportEmail                           *string `access:"site_customization"`
	CustomTermsOfServiceEnabled            *bool   `access:"compliance_custom_terms_of_service"`
	CustomTermsOfServiceReAcceptancePeriod *int    `access:"compliance_custom_terms_of_service"`
	EnableAskCommunityLink                 *bool   `access:"site_customization"`
}

func (s *SupportSettings) SetDefaults() {
	if !IsSafeLink(s.TermsOfServiceLink) {
		*s.TermsOfServiceLink = SupportSettingsDefaultTermsOfServiceLink
	}

	if s.TermsOfServiceLink == nil {
		s.TermsOfServiceLink = NewString(SupportSettingsDefaultTermsOfServiceLink)
	}

	if !IsSafeLink(s.PrivacyPolicyLink) {
		*s.PrivacyPolicyLink = ""
	}

	if s.PrivacyPolicyLink == nil {
		s.PrivacyPolicyLink = NewString(SupportSettingsDefaultPrivacyPolicyLink)
	}

	if !IsSafeLink(s.AboutLink) {
		*s.AboutLink = ""
	}

	if s.AboutLink == nil {
		s.AboutLink = NewString(SupportSettingsDefaultAboutLink)
	}

	if !IsSafeLink(s.HelpLink) {
		*s.HelpLink = ""
	}

	if s.HelpLink == nil {
		s.HelpLink = NewString(SupportSettingsDefaultHelpLink)
	}

	if !IsSafeLink(s.ReportAProblemLink) {
		*s.ReportAProblemLink = ""
	}

	if s.ReportAProblemLink == nil {
		s.ReportAProblemLink = NewString(SupportSettingsDefaultReportAProblemLink)
	}

	if s.SupportEmail == nil {
		s.SupportEmail = NewString(SupportSettingsDefaultSupportEmail)
	}

	if s.CustomTermsOfServiceEnabled == nil {
		s.CustomTermsOfServiceEnabled = NewBool(false)
	}

	if s.CustomTermsOfServiceReAcceptancePeriod == nil {
		s.CustomTermsOfServiceReAcceptancePeriod = NewInt(SupportSettingsDefaultReAcceptancePeriod)
	}

	if s.EnableAskCommunityLink == nil {
		s.EnableAskCommunityLink = NewBool(true)
	}
}

type AnnouncementSettings struct {
	EnableBanner          *bool   `access:"site_announcement_banner"`
	BannerText            *string `access:"site_announcement_banner"` // telemetry: none
	BannerColor           *string `access:"site_announcement_banner"`
	BannerTextColor       *string `access:"site_announcement_banner"`
	AllowBannerDismissal  *bool   `access:"site_announcement_banner"`
	AdminNoticesEnabled   *bool   `access:"site_notices"`
	UserNoticesEnabled    *bool   `access:"site_notices"`
	NoticesURL            *string `access:"site_notices,write_restrictable"` // telemetry: none
	NoticesFetchFrequency *int    `access:"site_notices,write_restrictable"` // telemetry: none
	NoticesSkipCache      *bool   `access:"site_notices,write_restrictable"` // telemetry: none
}

func (s *AnnouncementSettings) SetDefaults() {
	if s.EnableBanner == nil {
		s.EnableBanner = NewBool(false)
	}

	if s.BannerText == nil {
		s.BannerText = NewString("")
	}

	if s.BannerColor == nil {
		s.BannerColor = NewString(AnnouncementSettingsDefaultBannerColor)
	}

	if s.BannerTextColor == nil {
		s.BannerTextColor = NewString(AnnouncementSettingsDefaultBannerTextColor)
	}

	if s.AllowBannerDismissal == nil {
		s.AllowBannerDismissal = NewBool(true)
	}

	if s.AdminNoticesEnabled == nil {
		s.AdminNoticesEnabled = NewBool(true)
	}

	if s.UserNoticesEnabled == nil {
		s.UserNoticesEnabled = NewBool(true)
	}
	if s.NoticesURL == nil {
		s.NoticesURL = NewString(AnnouncementSettingsDefaultNoticesJSONURL)
	}
	if s.NoticesSkipCache == nil {
		s.NoticesSkipCache = NewBool(false)
	}
	if s.NoticesFetchFrequency == nil {
		s.NoticesFetchFrequency = NewInt(AnnouncementSettingsDefaultNoticesFetchFrequencySeconds)
	}

}

type ThemeSettings struct {
	EnableThemeSelection *bool   `access:"experimental_features"`
	DefaultTheme         *string `access:"experimental_features"`
	AllowCustomThemes    *bool   `access:"experimental_features"`
	AllowedThemes        []string
}

func (s *ThemeSettings) SetDefaults() {
	if s.EnableThemeSelection == nil {
		s.EnableThemeSelection = NewBool(true)
	}

	if s.DefaultTheme == nil {
		s.DefaultTheme = NewString(TeamSettingsDefaultTeamText)
	}

	if s.AllowCustomThemes == nil {
		s.AllowCustomThemes = NewBool(true)
	}

	if s.AllowedThemes == nil {
		s.AllowedThemes = []string{}
	}
}

type TeamSettings struct {
	SiteName                                                  *string  `access:"site_customization"`
	MaxUsersPerTeam                                           *int     `access:"site_users_and_teams"`
	DEPRECATED_DO_NOT_USE_EnableTeamCreation                  *bool    `json:"EnableTeamCreation" mapstructure:"EnableTeamCreation"` // Deprecated: do not use
	EnableUserCreation                                        *bool    `access:"authentication_signup"`
	EnableOpenServer                                          *bool    `access:"authentication_signup"`
	EnableUserDeactivation                                    *bool    `access:"experimental_features"`
	RestrictCreationToDomains                                 *string  `access:"authentication_signup"` // telemetry: none
	EnableCustomUserStatuses                                  *bool    `access:"site_users_and_teams"`
	EnableCustomBrand                                         *bool    `access:"site_customization"`
	CustomBrandText                                           *string  `access:"site_customization"`
	CustomDescriptionText                                     *string  `access:"site_customization"`
	RestrictDirectMessage                                     *string  `access:"site_users_and_teams"`
	DEPRECATED_DO_NOT_USE_RestrictTeamInvite                  *string  `json:"RestrictTeamInvite" mapstructure:"RestrictTeamInvite"`                                   // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement     *string  `json:"RestrictPublicChannelManagement" mapstructure:"RestrictPublicChannelManagement"`         // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement    *string  `json:"RestrictPrivateChannelManagement" mapstructure:"RestrictPrivateChannelManagement"`       // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation       *string  `json:"RestrictPublicChannelCreation" mapstructure:"RestrictPublicChannelCreation"`             // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPrivateChannelCreation      *string  `json:"RestrictPrivateChannelCreation" mapstructure:"RestrictPrivateChannelCreation"`           // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPublicChannelDeletion       *string  `json:"RestrictPublicChannelDeletion" mapstructure:"RestrictPublicChannelDeletion"`             // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPrivateChannelDeletion      *string  `json:"RestrictPrivateChannelDeletion" mapstructure:"RestrictPrivateChannelDeletion"`           // Deprecated: do not use
	DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManageMembers *string  `json:"RestrictPrivateChannelManageMembers" mapstructure:"RestrictPrivateChannelManageMembers"` // Deprecated: do not use
	EnableXToLeaveChannelsFromLHS                             *bool    `access:"experimental_features"`
	UserStatusAwayTimeout                                     *int64   `access:"experimental_features"`
	MaxChannelsPerTeam                                        *int64   `access:"site_users_and_teams"`
	MaxNotificationsPerChannel                                *int64   `access:"environment_push_notification_server"`
	EnableConfirmNotificationsToChannel                       *bool    `access:"site_notifications"`
	TeammateNameDisplay                                       *string  `access:"site_users_and_teams"`
	ExperimentalViewArchivedChannels                          *bool    `access:"experimental_features,site_users_and_teams"`
	ExperimentalEnableAutomaticReplies                        *bool    `access:"experimental_features"`
	ExperimentalHideTownSquareinLHS                           *bool    `access:"experimental_features"`
	ExperimentalTownSquareIsReadOnly                          *bool    `access:"experimental_features"`
	LockTeammateNameDisplay                                   *bool    `access:"site_users_and_teams"`
	ExperimentalPrimaryTeam                                   *string  `access:"experimental_features"`
	ExperimentalDefaultChannels                               []string `access:"experimental_features"`
}

func (s *TeamSettings) SetDefaults() {

	if s.SiteName == nil || *s.SiteName == "" {
		s.SiteName = NewString(TeamSettingsDefaultSiteName)
	}

	if s.MaxUsersPerTeam == nil {
		s.MaxUsersPerTeam = NewInt(TeamSettingsDefaultMaxUsersPerTeam)
	}

	if s.DEPRECATED_DO_NOT_USE_EnableTeamCreation == nil {
		s.DEPRECATED_DO_NOT_USE_EnableTeamCreation = NewBool(true)
	}

	if s.EnableUserCreation == nil {
		s.EnableUserCreation = NewBool(true)
	}

	if s.EnableOpenServer == nil {
		s.EnableOpenServer = NewBool(false)
	}

	if s.RestrictCreationToDomains == nil {
		s.RestrictCreationToDomains = NewString("")
	}

	if s.EnableCustomUserStatuses == nil {
		s.EnableCustomUserStatuses = NewBool(true)
	}

	if s.EnableCustomBrand == nil {
		s.EnableCustomBrand = NewBool(false)
	}

	if s.EnableUserDeactivation == nil {
		s.EnableUserDeactivation = NewBool(false)
	}

	if s.CustomBrandText == nil {
		s.CustomBrandText = NewString(TeamSettingsDefaultCustomBrandText)
	}

	if s.CustomDescriptionText == nil {
		s.CustomDescriptionText = NewString(TeamSettingsDefaultCustomDescriptionText)
	}

	if s.RestrictDirectMessage == nil {
		s.RestrictDirectMessage = NewString(DirectMessageAny)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictTeamInvite == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictTeamInvite = NewString(PermissionsAll)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement = NewString(PermissionsAll)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement = NewString(PermissionsAll)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation = new(string)
		// If this setting does not exist, assume migration from <3.6, so use management setting as default.
		if *s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement == PermissionsChannelAdmin {
			*s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation = PermissionsTeamAdmin
		} else {
			*s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation = *s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement
		}
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelCreation == nil {
		// If this setting does not exist, assume migration from <3.6, so use management setting as default.
		if *s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement == PermissionsChannelAdmin {
			s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelCreation = NewString(PermissionsTeamAdmin)
		} else {
			s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelCreation = NewString(*s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement)
		}
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelDeletion == nil {
		// If this setting does not exist, assume migration from <3.6, so use management setting as default.
		s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelDeletion = NewString(*s.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelDeletion == nil {
		// If this setting does not exist, assume migration from <3.6, so use management setting as default.
		s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelDeletion = NewString(*s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement)
	}

	if s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManageMembers == nil {
		s.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManageMembers = NewString(PermissionsAll)
	}

	if s.EnableXToLeaveChannelsFromLHS == nil {
		s.EnableXToLeaveChannelsFromLHS = NewBool(false)
	}

	if s.UserStatusAwayTimeout == nil {
		s.UserStatusAwayTimeout = NewInt64(TeamSettingsDefaultUserStatusAwayTimeout)
	}

	if s.MaxChannelsPerTeam == nil {
		s.MaxChannelsPerTeam = NewInt64(2000)
	}

	if s.MaxNotificationsPerChannel == nil {
		s.MaxNotificationsPerChannel = NewInt64(1000)
	}

	if s.EnableConfirmNotificationsToChannel == nil {
		s.EnableConfirmNotificationsToChannel = NewBool(true)
	}

	if s.ExperimentalEnableAutomaticReplies == nil {
		s.ExperimentalEnableAutomaticReplies = NewBool(false)
	}

	if s.ExperimentalHideTownSquareinLHS == nil {
		s.ExperimentalHideTownSquareinLHS = NewBool(false)
	}

	if s.ExperimentalTownSquareIsReadOnly == nil {
		s.ExperimentalTownSquareIsReadOnly = NewBool(false)
	}

	if s.ExperimentalPrimaryTeam == nil {
		s.ExperimentalPrimaryTeam = NewString("")
	}

	if s.ExperimentalDefaultChannels == nil {
		s.ExperimentalDefaultChannels = []string{}
	}

	if s.DEPRECATED_DO_NOT_USE_EnableTeamCreation == nil {
		s.DEPRECATED_DO_NOT_USE_EnableTeamCreation = NewBool(true)
	}

	if s.EnableUserCreation == nil {
		s.EnableUserCreation = NewBool(true)
	}

	if s.ExperimentalViewArchivedChannels == nil {
		s.ExperimentalViewArchivedChannels = NewBool(true)
	}

	if s.LockTeammateNameDisplay == nil {
		s.LockTeammateNameDisplay = NewBool(false)
	}
}

type ClientRequirements struct {
	AndroidLatestVersion string `access:"write_restrictable,cloud_restrictable"`
	AndroidMinVersion    string `access:"write_restrictable,cloud_restrictable"`
	DesktopLatestVersion string `access:"write_restrictable,cloud_restrictable"`
	DesktopMinVersion    string `access:"write_restrictable,cloud_restrictable"`
	IosLatestVersion     string `access:"write_restrictable,cloud_restrictable"`
	IosMinVersion        string `access:"write_restrictable,cloud_restrictable"`
}

type LdapSettings struct {
	// Basic
	Enable             *bool   `access:"authentication_ldap"`
	EnableSync         *bool   `access:"authentication_ldap"`
	LdapServer         *string `access:"authentication_ldap"` // telemetry: none
	LdapPort           *int    `access:"authentication_ldap"` // telemetry: none
	ConnectionSecurity *string `access:"authentication_ldap"`
	BaseDN             *string `access:"authentication_ldap"` // telemetry: none
	BindUsername       *string `access:"authentication_ldap"` // telemetry: none
	BindPassword       *string `access:"authentication_ldap"` // telemetry: none

	// Filtering
	UserFilter        *string `access:"authentication_ldap"` // telemetry: none
	GroupFilter       *string `access:"authentication_ldap"`
	GuestFilter       *string `access:"authentication_ldap"`
	EnableAdminFilter *bool
	AdminFilter       *string

	// Group Mapping
	GroupDisplayNameAttribute *string `access:"authentication_ldap"`
	GroupIDAttribute          *string `access:"authentication_ldap"`

	// User Mapping
	FirstNameAttribute *string `access:"authentication_ldap"`
	LastNameAttribute  *string `access:"authentication_ldap"`
	EmailAttribute     *string `access:"authentication_ldap"`
	UsernameAttribute  *string `access:"authentication_ldap"`
	NicknameAttribute  *string `access:"authentication_ldap"`
	IDAttribute        *string `access:"authentication_ldap"`
	PositionAttribute  *string `access:"authentication_ldap"`
	LoginIDAttribute   *string `access:"authentication_ldap"`
	PictureAttribute   *string `access:"authentication_ldap"`

	// Synchronization
	SyncIntervalMinutes *int `access:"authentication_ldap"`

	// Advanced
	SkipCertificateVerification *bool   `access:"authentication_ldap"`
	PublicCertificateFile       *string `access:"authentication_ldap"`
	PrivateKeyFile              *string `access:"authentication_ldap"`
	QueryTimeout                *int    `access:"authentication_ldap"`
	MaxPageSize                 *int    `access:"authentication_ldap"`

	// Customization
	LoginFieldName *string `access:"authentication_ldap"`

	LoginButtonColor       *string `access:"experimental_features"`
	LoginButtonBorderColor *string `access:"experimental_features"`
	LoginButtonTextColor   *string `access:"experimental_features"`

	Trace *bool `access:"authentication_ldap"` // telemetry: none
}

func (s *LdapSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	// When unset should default to LDAP Enabled
	if s.EnableSync == nil {
		s.EnableSync = NewBool(*s.Enable)
	}

	if s.EnableAdminFilter == nil {
		s.EnableAdminFilter = NewBool(false)
	}

	if s.LdapServer == nil {
		s.LdapServer = NewString("")
	}

	if s.LdapPort == nil {
		s.LdapPort = NewInt(389)
	}

	if s.ConnectionSecurity == nil {
		s.ConnectionSecurity = NewString("")
	}

	if s.PublicCertificateFile == nil {
		s.PublicCertificateFile = NewString("")
	}

	if s.PrivateKeyFile == nil {
		s.PrivateKeyFile = NewString("")
	}

	if s.BaseDN == nil {
		s.BaseDN = NewString("")
	}

	if s.BindUsername == nil {
		s.BindUsername = NewString("")
	}

	if s.BindPassword == nil {
		s.BindPassword = NewString("")
	}

	if s.UserFilter == nil {
		s.UserFilter = NewString("")
	}

	if s.GuestFilter == nil {
		s.GuestFilter = NewString("")
	}

	if s.AdminFilter == nil {
		s.AdminFilter = NewString("")
	}

	if s.GroupFilter == nil {
		s.GroupFilter = NewString("")
	}

	if s.GroupDisplayNameAttribute == nil {
		s.GroupDisplayNameAttribute = NewString(LdapSettingsDefaultGroupDisplayNameAttribute)
	}

	if s.GroupIDAttribute == nil {
		s.GroupIDAttribute = NewString(LdapSettingsDefaultGroupIDAttribute)
	}

	if s.FirstNameAttribute == nil {
		s.FirstNameAttribute = NewString(LdapSettingsDefaultFirstNameAttribute)
	}

	if s.LastNameAttribute == nil {
		s.LastNameAttribute = NewString(LdapSettingsDefaultLastNameAttribute)
	}

	if s.EmailAttribute == nil {
		s.EmailAttribute = NewString(LdapSettingsDefaultEmailAttribute)
	}

	if s.UsernameAttribute == nil {
		s.UsernameAttribute = NewString(LdapSettingsDefaultUsernameAttribute)
	}

	if s.NicknameAttribute == nil {
		s.NicknameAttribute = NewString(LdapSettingsDefaultNicknameAttribute)
	}

	if s.IDAttribute == nil {
		s.IDAttribute = NewString(LdapSettingsDefaultIDAttribute)
	}

	if s.PositionAttribute == nil {
		s.PositionAttribute = NewString(LdapSettingsDefaultPositionAttribute)
	}

	if s.PictureAttribute == nil {
		s.PictureAttribute = NewString(LdapSettingsDefaultPictureAttribute)
	}

	// For those upgrading to the version when LoginIdAttribute was added
	// they need IdAttribute == LoginIdAttribute not to break
	if s.LoginIDAttribute == nil {
		s.LoginIDAttribute = s.IDAttribute
	}

	if s.SyncIntervalMinutes == nil {
		s.SyncIntervalMinutes = NewInt(60)
	}

	if s.SkipCertificateVerification == nil {
		s.SkipCertificateVerification = NewBool(false)
	}

	if s.QueryTimeout == nil {
		s.QueryTimeout = NewInt(60)
	}

	if s.MaxPageSize == nil {
		s.MaxPageSize = NewInt(0)
	}

	if s.LoginFieldName == nil {
		s.LoginFieldName = NewString(LdapSettingsDefaultLoginFieldName)
	}

	if s.LoginButtonColor == nil {
		s.LoginButtonColor = NewString("#0000")
	}

	if s.LoginButtonBorderColor == nil {
		s.LoginButtonBorderColor = NewString("#2389D7")
	}

	if s.LoginButtonTextColor == nil {
		s.LoginButtonTextColor = NewString("#2389D7")
	}

	if s.Trace == nil {
		s.Trace = NewBool(false)
	}
}

type ComplianceSettings struct {
	Enable      *bool   `access:"compliance_compliance_monitoring"`
	Directory   *string `access:"compliance_compliance_monitoring"` // telemetry: none
	EnableDaily *bool   `access:"compliance_compliance_monitoring"`
	BatchSize   *int    `access:"compliance_compliance_monitoring"` // telemetry: none
}

func (s *ComplianceSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.Directory == nil {
		s.Directory = NewString("./data/")
	}

	if s.EnableDaily == nil {
		s.EnableDaily = NewBool(false)
	}

	if s.BatchSize == nil {
		s.BatchSize = NewInt(30000)
	}
}

type LocalizationSettings struct {
	DefaultServerLocale *string `access:"site_localization"`
	DefaultClientLocale *string `access:"site_localization"`
	AvailableLocales    *string `access:"site_localization"`
}

func (s *LocalizationSettings) SetDefaults() {
	if s.DefaultServerLocale == nil {
		s.DefaultServerLocale = NewString(DefaultLocale)
	}

	if s.DefaultClientLocale == nil {
		s.DefaultClientLocale = NewString(DefaultLocale)
	}

	if s.AvailableLocales == nil {
		s.AvailableLocales = NewString("")
	}
}

type SamlSettings struct {
	// Basic
	Enable                        *bool `access:"authentication_saml"`
	EnableSyncWithLdap            *bool `access:"authentication_saml"`
	EnableSyncWithLdapIncludeAuth *bool `access:"authentication_saml"`
	IgnoreGuestsLdapSync          *bool `access:"authentication_saml"`

	Verify      *bool `access:"authentication_saml"`
	Encrypt     *bool `access:"authentication_saml"`
	SignRequest *bool `access:"authentication_saml"`

	IDpURL                      *string `access:"authentication_saml"` // telemetry: none
	IDpDescriptorURL            *string `access:"authentication_saml"` // telemetry: none
	IDpMetadataURL              *string `access:"authentication_saml"` // telemetry: none
	ServiceProviderIdentifier   *string `access:"authentication_saml"` // telemetry: none
	AssertionConsumerServiceURL *string `access:"authentication_saml"` // telemetry: none

	SignatureAlgorithm *string `access:"authentication_saml"`
	CanonicalAlgorithm *string `access:"authentication_saml"`

	ScopingIDPProviderID *string `access:"authentication_saml"`
	ScopingIDPName       *string `access:"authentication_saml"`

	IDpCertificateFile    *string `access:"authentication_saml"` // telemetry: none
	PublicCertificateFile *string `access:"authentication_saml"` // telemetry: none
	PrivateKeyFile        *string `access:"authentication_saml"` // telemetry: none

	// User Mapping
	IDAttribute          *string `access:"authentication_saml"`
	GuestAttribute       *string `access:"authentication_saml"`
	EnableAdminAttribute *bool
	AdminAttribute       *string
	FirstNameAttribute   *string `access:"authentication_saml"`
	LastNameAttribute    *string `access:"authentication_saml"`
	EmailAttribute       *string `access:"authentication_saml"`
	UsernameAttribute    *string `access:"authentication_saml"`
	NicknameAttribute    *string `access:"authentication_saml"`
	LocaleAttribute      *string `access:"authentication_saml"`
	PositionAttribute    *string `access:"authentication_saml"`

	LoginButtonText *string `access:"authentication_saml"`

	LoginButtonColor       *string `access:"experimental_features"`
	LoginButtonBorderColor *string `access:"experimental_features"`
	LoginButtonTextColor   *string `access:"experimental_features"`
}

func (s *SamlSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.EnableSyncWithLdap == nil {
		s.EnableSyncWithLdap = NewBool(false)
	}

	if s.EnableSyncWithLdapIncludeAuth == nil {
		s.EnableSyncWithLdapIncludeAuth = NewBool(false)
	}

	if s.IgnoreGuestsLdapSync == nil {
		s.IgnoreGuestsLdapSync = NewBool(false)
	}

	if s.EnableAdminAttribute == nil {
		s.EnableAdminAttribute = NewBool(false)
	}

	if s.Verify == nil {
		s.Verify = NewBool(true)
	}

	if s.Encrypt == nil {
		s.Encrypt = NewBool(true)
	}

	if s.SignRequest == nil {
		s.SignRequest = NewBool(false)
	}

	if s.SignatureAlgorithm == nil {
		s.SignatureAlgorithm = NewString(SamlSettingsDefaultSignatureAlgorithm)
	}

	if s.CanonicalAlgorithm == nil {
		s.CanonicalAlgorithm = NewString(SamlSettingsDefaultCanonicalAlgorithm)
	}

	if s.IDpURL == nil {
		s.IDpURL = NewString("")
	}

	if s.IDpDescriptorURL == nil {
		s.IDpDescriptorURL = NewString("")
	}

	if s.ServiceProviderIdentifier == nil {
		if s.IDpDescriptorURL != nil {
			s.ServiceProviderIdentifier = NewString(*s.IDpDescriptorURL)
		} else {
			s.ServiceProviderIdentifier = NewString("")
		}
	}

	if s.IDpMetadataURL == nil {
		s.IDpMetadataURL = NewString("")
	}

	if s.IDpCertificateFile == nil {
		s.IDpCertificateFile = NewString("")
	}

	if s.PublicCertificateFile == nil {
		s.PublicCertificateFile = NewString("")
	}

	if s.PrivateKeyFile == nil {
		s.PrivateKeyFile = NewString("")
	}

	if s.AssertionConsumerServiceURL == nil {
		s.AssertionConsumerServiceURL = NewString("")
	}

	if s.ScopingIDPProviderID == nil {
		s.ScopingIDPProviderID = NewString("")
	}

	if s.ScopingIDPName == nil {
		s.ScopingIDPName = NewString("")
	}

	if s.LoginButtonText == nil || *s.LoginButtonText == "" {
		s.LoginButtonText = NewString(UserAuthServiceSamlText)
	}

	if s.IDAttribute == nil {
		s.IDAttribute = NewString(SamlSettingsDefaultIDAttribute)
	}

	if s.GuestAttribute == nil {
		s.GuestAttribute = NewString(SamlSettingsDefaultGuestAttribute)
	}
	if s.AdminAttribute == nil {
		s.AdminAttribute = NewString(SamlSettingsDefaultAdminAttribute)
	}
	if s.FirstNameAttribute == nil {
		s.FirstNameAttribute = NewString(SamlSettingsDefaultFirstNameAttribute)
	}

	if s.LastNameAttribute == nil {
		s.LastNameAttribute = NewString(SamlSettingsDefaultLastNameAttribute)
	}

	if s.EmailAttribute == nil {
		s.EmailAttribute = NewString(SamlSettingsDefaultEmailAttribute)
	}

	if s.UsernameAttribute == nil {
		s.UsernameAttribute = NewString(SamlSettingsDefaultUsernameAttribute)
	}

	if s.NicknameAttribute == nil {
		s.NicknameAttribute = NewString(SamlSettingsDefaultNicknameAttribute)
	}

	if s.PositionAttribute == nil {
		s.PositionAttribute = NewString(SamlSettingsDefaultPositionAttribute)
	}

	if s.LocaleAttribute == nil {
		s.LocaleAttribute = NewString(SamlSettingsDefaultLocaleAttribute)
	}

	if s.LoginButtonColor == nil {
		s.LoginButtonColor = NewString("#34a28b")
	}

	if s.LoginButtonBorderColor == nil {
		s.LoginButtonBorderColor = NewString("#2389D7")
	}

	if s.LoginButtonTextColor == nil {
		s.LoginButtonTextColor = NewString("#ffffff")
	}
}

type NativeAppSettings struct {
	AppCustomURLSchemes    []string `access:"site_customization,write_restrictable,cloud_restrictable"` // telemetry: none
	AppDownloadLink        *string  `access:"site_customization,write_restrictable,cloud_restrictable"`
	AndroidAppDownloadLink *string  `access:"site_customization,write_restrictable,cloud_restrictable"`
	IosAppDownloadLink     *string  `access:"site_customization,write_restrictable,cloud_restrictable"`
}

func (s *NativeAppSettings) SetDefaults() {
	if s.AppDownloadLink == nil {
		s.AppDownloadLink = NewString(NativeappSettingsDefaultAppDownloadLink)
	}

	if s.AndroidAppDownloadLink == nil {
		s.AndroidAppDownloadLink = NewString(NativeappSettingsDefaultAndroidAppDownloadLink)
	}

	if s.IosAppDownloadLink == nil {
		s.IosAppDownloadLink = NewString(NativeappSettingsDefaultIosAppDownloadLink)
	}

	if s.AppCustomURLSchemes == nil {
		s.AppCustomURLSchemes = GetDefaultAppCustomURLSchemes()
	}
}

type ElasticsearchSettings struct {
	ConnectionURL                 *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	Username                      *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	Password                      *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	EnableIndexing                *bool   `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	EnableSearching               *bool   `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	EnableAutocomplete            *bool   `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	Sniff                         *bool   `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	PostIndexReplicas             *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	PostIndexShards               *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	ChannelIndexReplicas          *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	ChannelIndexShards            *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	UserIndexReplicas             *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	UserIndexShards               *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	AggregatePostsAfterDays       *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"` // telemetry: none
	PostsAggregatorJobStartTime   *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"` // telemetry: none
	IndexPrefix                   *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	LiveIndexingBatchSize         *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	BulkIndexingTimeWindowSeconds *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	RequestTimeoutSeconds         *int    `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	SkipTLSVerification           *bool   `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
	Trace                         *string `access:"environment_elasticsearch,write_restrictable,cloud_restrictable"`
}

func (s *ElasticsearchSettings) SetDefaults() {
	if s.ConnectionURL == nil {
		s.ConnectionURL = NewString(ElasticsearchSettingsDefaultConnectionURL)
	}

	if s.Username == nil {
		s.Username = NewString(ElasticsearchSettingsDefaultUsername)
	}

	if s.Password == nil {
		s.Password = NewString(ElasticsearchSettingsDefaultPassword)
	}

	if s.EnableIndexing == nil {
		s.EnableIndexing = NewBool(false)
	}

	if s.EnableSearching == nil {
		s.EnableSearching = NewBool(false)
	}

	if s.EnableAutocomplete == nil {
		s.EnableAutocomplete = NewBool(false)
	}

	if s.Sniff == nil {
		s.Sniff = NewBool(true)
	}

	if s.PostIndexReplicas == nil {
		s.PostIndexReplicas = NewInt(ElasticsearchSettingsDefaultPostIndexReplicas)
	}

	if s.PostIndexShards == nil {
		s.PostIndexShards = NewInt(ElasticsearchSettingsDefaultPostIndexShards)
	}

	if s.ChannelIndexReplicas == nil {
		s.ChannelIndexReplicas = NewInt(ElasticsearchSettingsDefaultChannelIndexReplicas)
	}

	if s.ChannelIndexShards == nil {
		s.ChannelIndexShards = NewInt(ElasticsearchSettingsDefaultChannelIndexShards)
	}

	if s.UserIndexReplicas == nil {
		s.UserIndexReplicas = NewInt(ElasticsearchSettingsDefaultUserIndexReplicas)
	}

	if s.UserIndexShards == nil {
		s.UserIndexShards = NewInt(ElasticsearchSettingsDefaultUserIndexShards)
	}

	if s.AggregatePostsAfterDays == nil {
		s.AggregatePostsAfterDays = NewInt(ElasticsearchSettingsDefaultAggregatePostsAfterDays)
	}

	if s.PostsAggregatorJobStartTime == nil {
		s.PostsAggregatorJobStartTime = NewString(ElasticsearchSettingsDefaultPostsAggregatorJobStartTime)
	}

	if s.IndexPrefix == nil {
		s.IndexPrefix = NewString(ElasticsearchSettingsDefaultIndexPrefix)
	}

	if s.LiveIndexingBatchSize == nil {
		s.LiveIndexingBatchSize = NewInt(ElasticsearchSettingsDefaultLiveIndexingBatchSize)
	}

	if s.BulkIndexingTimeWindowSeconds == nil {
		s.BulkIndexingTimeWindowSeconds = NewInt(ElasticsearchSettingsDefaultBulkIndexingTimeWindowSeconds)
	}

	if s.RequestTimeoutSeconds == nil {
		s.RequestTimeoutSeconds = NewInt(ElasticsearchSettingsDefaultRequestTimeoutSeconds)
	}

	if s.SkipTLSVerification == nil {
		s.SkipTLSVerification = NewBool(false)
	}

	if s.Trace == nil {
		s.Trace = NewString("")
	}
}

type BleveSettings struct {
	IndexDir                      *string `access:"experimental_bleve"` // telemetry: none
	EnableIndexing                *bool   `access:"experimental_bleve"`
	EnableSearching               *bool   `access:"experimental_bleve"`
	EnableAutocomplete            *bool   `access:"experimental_bleve"`
	BulkIndexingTimeWindowSeconds *int    `access:"experimental_bleve"`
}

func (bs *BleveSettings) SetDefaults() {
	if bs.IndexDir == nil {
		bs.IndexDir = NewString(BleveSettingsDefaultIndexDir)
	}

	if bs.EnableIndexing == nil {
		bs.EnableIndexing = NewBool(false)
	}

	if bs.EnableSearching == nil {
		bs.EnableSearching = NewBool(false)
	}

	if bs.EnableAutocomplete == nil {
		bs.EnableAutocomplete = NewBool(false)
	}

	if bs.BulkIndexingTimeWindowSeconds == nil {
		bs.BulkIndexingTimeWindowSeconds = NewInt(BleveSettingsDefaultBulkIndexingTimeWindowSeconds)
	}
}

type DataRetentionSettings struct {
	EnableMessageDeletion *bool   `access:"compliance_data_retention_policy"`
	EnableFileDeletion    *bool   `access:"compliance_data_retention_policy"`
	MessageRetentionDays  *int    `access:"compliance_data_retention_policy"`
	FileRetentionDays     *int    `access:"compliance_data_retention_policy"`
	DeletionJobStartTime  *string `access:"compliance_data_retention_policy"`
	BatchSize             *int    `access:"compliance_data_retention_policy"`
}

func (s *DataRetentionSettings) SetDefaults() {
	if s.EnableMessageDeletion == nil {
		s.EnableMessageDeletion = NewBool(false)
	}

	if s.EnableFileDeletion == nil {
		s.EnableFileDeletion = NewBool(false)
	}

	if s.MessageRetentionDays == nil {
		s.MessageRetentionDays = NewInt(DataRetentionSettingsDefaultMessageRetentionDays)
	}

	if s.FileRetentionDays == nil {
		s.FileRetentionDays = NewInt(DataRetentionSettingsDefaultFileRetentionDays)
	}

	if s.DeletionJobStartTime == nil {
		s.DeletionJobStartTime = NewString(DataRetentionSettingsDefaultDeletionJobStartTime)
	}

	if s.BatchSize == nil {
		s.BatchSize = NewInt(DataRetentionSettingsDefaultBatchSize)
	}
}

type JobSettings struct {
	RunJobs      *bool `access:"write_restrictable,cloud_restrictable"`
	RunScheduler *bool `access:"write_restrictable,cloud_restrictable"`
}

func (s *JobSettings) SetDefaults() {
	if s.RunJobs == nil {
		s.RunJobs = NewBool(true)
	}

	if s.RunScheduler == nil {
		s.RunScheduler = NewBool(true)
	}
}

type CloudSettings struct {
	CWSURL    *string `access:"write_restrictable"`
	CWSAPIURL *string `access:"write_restrictable"`
}

func (s *CloudSettings) SetDefaults() {
	if s.CWSURL == nil {
		s.CWSURL = NewString(CloudSettingsDefaultCwsURL)
	}
	if s.CWSAPIURL == nil {
		s.CWSAPIURL = NewString(CloudSettingsDefaultCwsAPIURL)
	}
}

type PluginState struct {
	Enable bool
}

type PluginSettings struct {
	Enable                      *bool                             `access:"plugins,write_restrictable"`
	EnableUploads               *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	AllowInsecureDownloadURL    *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	EnableHealthCheck           *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	Directory                   *string                           `access:"plugins,write_restrictable,cloud_restrictable"` // telemetry: none
	ClientDirectory             *string                           `access:"plugins,write_restrictable,cloud_restrictable"` // telemetry: none
	Plugins                     map[string]map[string]interface{} `access:"plugins"`                                       // telemetry: none
	PluginStates                map[string]*PluginState           `access:"plugins"`                                       // telemetry: none
	EnableMarketplace           *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	EnableRemoteMarketplace     *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	AutomaticPrepackagedPlugins *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	RequirePluginSignature      *bool                             `access:"plugins,write_restrictable,cloud_restrictable"`
	MarketplaceURL              *string                           `access:"plugins,write_restrictable,cloud_restrictable"`
	SignaturePublicKeyFiles     []string                          `access:"plugins,write_restrictable,cloud_restrictable"`
}

func (s *PluginSettings) SetDefaults(ls LogSettings) {
	if s.Enable == nil {
		s.Enable = NewBool(true)
	}

	if s.EnableUploads == nil {
		s.EnableUploads = NewBool(false)
	}

	if s.AllowInsecureDownloadURL == nil {
		s.AllowInsecureDownloadURL = NewBool(false)
	}

	if s.EnableHealthCheck == nil {
		s.EnableHealthCheck = NewBool(true)
	}

	if s.Directory == nil || *s.Directory == "" {
		s.Directory = NewString(PluginSettingsDefaultDirectory)
	}

	if s.ClientDirectory == nil || *s.ClientDirectory == "" {
		s.ClientDirectory = NewString(PluginSettingsDefaultClientDirectory)
	}

	if s.Plugins == nil {
		s.Plugins = make(map[string]map[string]interface{})
	}

	if s.PluginStates == nil {
		s.PluginStates = make(map[string]*PluginState)
	}

	if s.PluginStates["com.mattermost.nps"] == nil {
		// Enable the NPS plugin by default if diagnostics are enabled
		s.PluginStates["com.mattermost.nps"] = &PluginState{Enable: ls.EnableDiagnostics == nil || *ls.EnableDiagnostics}
	}

	if s.PluginStates["com.mattermost.plugin-incident-management"] == nil && BuildEnterpriseReady == "true" {
		// Enable the incident management plugin by default
		s.PluginStates["com.mattermost.plugin-incident-management"] = &PluginState{Enable: true}
	}

	if s.PluginStates["com.mattermost.plugin-channel-export"] == nil && BuildEnterpriseReady == "true" {
		// Enable the channel export plugin by default
		s.PluginStates["com.mattermost.plugin-channel-export"] = &PluginState{Enable: true}
	}

	if s.EnableMarketplace == nil {
		s.EnableMarketplace = NewBool(PluginSettingsDefaultEnableMarketplace)
	}

	if s.EnableRemoteMarketplace == nil {
		s.EnableRemoteMarketplace = NewBool(true)
	}

	if s.AutomaticPrepackagedPlugins == nil {
		s.AutomaticPrepackagedPlugins = NewBool(true)
	}

	if s.MarketplaceURL == nil || *s.MarketplaceURL == "" || *s.MarketplaceURL == PluginSettingsOldMarketplaceURL {
		s.MarketplaceURL = NewString(PluginSettingsDefaultMarketplaceURL)
	}

	if s.RequirePluginSignature == nil {
		s.RequirePluginSignature = NewBool(false)
	}

	if s.SignaturePublicKeyFiles == nil {
		s.SignaturePublicKeyFiles = []string{}
	}
}

type GlobalRelayMessageExportSettings struct {
	CustomerType      *string `access:"compliance_compliance_export"` // must be either A9 or A10, dictates SMTP server url
	SmtpUsername      *string `access:"compliance_compliance_export"`
	SmtpPassword      *string `access:"compliance_compliance_export"`
	EmailAddress      *string `access:"compliance_compliance_export"` // the address to send messages to
	SMTPServerTimeout *int    `access:"compliance_compliance_export"`
}

func (s *GlobalRelayMessageExportSettings) SetDefaults() {
	if s.CustomerType == nil {
		s.CustomerType = NewString(GlobalrelayCustomerTypeA9)
	}
	if s.SmtpUsername == nil {
		s.SmtpUsername = NewString("")
	}
	if s.SmtpPassword == nil {
		s.SmtpPassword = NewString("")
	}
	if s.EmailAddress == nil {
		s.EmailAddress = NewString("")
	}
	if s.SMTPServerTimeout == nil || *s.SMTPServerTimeout == 0 {
		s.SMTPServerTimeout = NewInt(1800)
	}
}

type MessageExportSettings struct {
	EnableExport          *bool   `access:"compliance_compliance_export"`
	ExportFormat          *string `access:"compliance_compliance_export"`
	DailyRunTime          *string `access:"compliance_compliance_export"`
	ExportFromTimestamp   *int64  `access:"compliance_compliance_export"`
	BatchSize             *int    `access:"compliance_compliance_export"`
	DownloadExportResults *bool   `access:"compliance_compliance_export"`

	// formatter-specific settings - these are only expected to be non-nil if ExportFormat is set to the associated format
	GlobalRelaySettings *GlobalRelayMessageExportSettings `access:"compliance_compliance_export"`
}

func (s *MessageExportSettings) SetDefaults() {
	if s.EnableExport == nil {
		s.EnableExport = NewBool(false)
	}

	if s.DownloadExportResults == nil {
		s.DownloadExportResults = NewBool(false)
	}

	if s.ExportFormat == nil {
		s.ExportFormat = NewString(ComplianceExportTypeActiance)
	}

	if s.DailyRunTime == nil {
		s.DailyRunTime = NewString("01:00")
	}

	if s.ExportFromTimestamp == nil {
		s.ExportFromTimestamp = NewInt64(0)
	}

	if s.BatchSize == nil {
		s.BatchSize = NewInt(10000)
	}

	if s.GlobalRelaySettings == nil {
		s.GlobalRelaySettings = &GlobalRelayMessageExportSettings{}
	}
	s.GlobalRelaySettings.SetDefaults()
}

type DisplaySettings struct {
	CustomURLSchemes     []string `access:"site_customization"`
	ExperimentalTimezone *bool    `access:"experimental_features"`
}

func (s *DisplaySettings) SetDefaults() {
	if s.CustomURLSchemes == nil {
		customURLSchemes := []string{}
		s.CustomURLSchemes = customURLSchemes
	}

	if s.ExperimentalTimezone == nil {
		s.ExperimentalTimezone = NewBool(true)
	}
}

type GuestAccountsSettings struct {
	Enable                           *bool   `access:"authentication_guest_access"`
	AllowEmailAccounts               *bool   `access:"authentication_guest_access"`
	EnforceMultifactorAuthentication *bool   `access:"authentication_guest_access"`
	RestrictCreationToDomains        *string `access:"authentication_guest_access"`
}

func (s *GuestAccountsSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewBool(false)
	}

	if s.AllowEmailAccounts == nil {
		s.AllowEmailAccounts = NewBool(true)
	}

	if s.EnforceMultifactorAuthentication == nil {
		s.EnforceMultifactorAuthentication = NewBool(false)
	}

	if s.RestrictCreationToDomains == nil {
		s.RestrictCreationToDomains = NewString("")
	}
}

type ImageProxySettings struct {
	Enable                  *bool   `access:"environment_image_proxy"`
	ImageProxyType          *string `access:"environment_image_proxy"`
	RemoteImageProxyURL     *string `access:"environment_image_proxy"`
	RemoteImageProxyOptions *string `access:"environment_image_proxy"`
}

func (s *ImageProxySettings) SetDefaults(ss ServiceSettings) {
	if s.Enable == nil {
		if ss.DEPRECATED_DO_NOT_USE_ImageProxyType == nil || *ss.DEPRECATED_DO_NOT_USE_ImageProxyType == "" {
			s.Enable = NewBool(false)
		} else {
			s.Enable = NewBool(true)
		}
	}

	if s.ImageProxyType == nil {
		if ss.DEPRECATED_DO_NOT_USE_ImageProxyType == nil || *ss.DEPRECATED_DO_NOT_USE_ImageProxyType == "" {
			s.ImageProxyType = NewString(ImageProxyTypeLocal)
		} else {
			s.ImageProxyType = ss.DEPRECATED_DO_NOT_USE_ImageProxyType
		}
	}

	if s.RemoteImageProxyURL == nil {
		if ss.DEPRECATED_DO_NOT_USE_ImageProxyURL == nil {
			s.RemoteImageProxyURL = NewString("")
		} else {
			s.RemoteImageProxyURL = ss.DEPRECATED_DO_NOT_USE_ImageProxyURL
		}
	}

	if s.RemoteImageProxyOptions == nil {
		if ss.DEPRECATED_DO_NOT_USE_ImageProxyOptions == nil {
			s.RemoteImageProxyOptions = NewString("")
		} else {
			s.RemoteImageProxyOptions = ss.DEPRECATED_DO_NOT_USE_ImageProxyOptions
		}
	}
}

// ImportSettings defines configuration settings for file imports.
type ImportSettings struct {
	// The directory where to store the imported files.
	Directory *string
	// The number of days to retain the imported files before deleting them.
	RetentionDays *int
}

func (s *ImportSettings) isValid() *AppError {
	if *s.Directory == "" {
		return NewAppError("Config.IsValid", "model.config.is_valid.import.directory.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.RetentionDays <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.import.retention_days_too_low.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

// SetDefaults applies the default settings to the struct.
func (s *ImportSettings) SetDefaults() {
	if s.Directory == nil || *s.Directory == "" {
		s.Directory = NewString(ImportSettingsDefaultDirectory)
	}

	if s.RetentionDays == nil {
		s.RetentionDays = NewInt(ImportSettingsDefaultRetentionDays)
	}
}

// ExportSettings defines configuration settings for file exports.
type ExportSettings struct {
	// The directory where to store the exported files.
	Directory *string // telemetry: none
	// The number of days to retain the exported files before deleting them.
	RetentionDays *int
}

func (s *ExportSettings) isValid() *AppError {
	if *s.Directory == "" {
		return NewAppError("Config.IsValid", "model.config.is_valid.export.directory.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.RetentionDays <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.export.retention_days_too_low.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

// SetDefaults applies the default settings to the struct.
func (s *ExportSettings) SetDefaults() {
	if s.Directory == nil || *s.Directory == "" {
		s.Directory = NewString(ExportSettingsDefaultDirectory)
	}

	if s.RetentionDays == nil {
		s.RetentionDays = NewInt(ExportSettingsDefaultRetentionDays)
	}
}

type ConfigFunc func() *Config

const ConfigAccessTagType = "access"
const ConfigAccessTagWriteRestrictable = "write_restrictable"
const ConfigAccessTagCloudRestrictable = "cloud_restrictable"

// Allows read access if any PermissionSysconsoleRead* is allowed
const ConfigAccessTagAnySysConsoleRead = "*_read"

// Config fields support the 'access' tag with the following values corresponding to the suffix of the associated
// PermissionSysconsole* permission Id: 'about', 'reporting', 'user_management_users',
// 'user_management_groups', 'user_management_teams', 'user_management_channels',
// 'user_management_permissions', 'environment_web_server', 'environment_database', 'environment_elasticsearch',
// 'environment_file_storage', 'environment_image_proxy', 'environment_smtp', 'environment_push_notification_server',
// 'environment_high_availability', 'environment_rate_limiting', 'environment_logging', 'environment_session_lengths',
// 'environment_performance_monitoring', 'environment_developer', 'site', 'authentication', 'plugins',
// 'integrations', 'compliance', 'plugins', and 'experimental'. They grant read and/or write access to the config field
// to roles without PermissionManageSystem.
//
// The 'access' tag '*_read' checks for any Sysconsole read permission and grants access if any read permission is allowed.
//
// By default config values can be written with PermissionManageSystem, but if ExperimentalSettings.RestrictSystemAdmin is true
// and the access tag contains the value 'write_restrictable', then even PermissionManageSystem, does not grant write access.
//
// PermissionManageSystem always grants read access.
//
// Config values with the access tag 'cloud_restrictable' mean that are marked to be filtered when it's used in a cloud licensed
// environment with ExperimentalSettings.RestrictedSystemAdmin set to true.
//
// Example:
//  type HairSettings struct {
//      // Colour is writeable with either PermissionSysconsoleWriteReporting or PermissionSysconsoleWriteUserManagementGroups.
//      // It is readable by PermissionSysconsoleReadReporting and PermissionSysconsoleReadUserManagementGroups permissions.
//      // PermissionManageSystem grants read and write access.
//      Colour string `access:"reporting,user_management_groups"`
//
//      // Length is only readable and writable via PermissionManageSystem.
//      Length string
//
//      // Product is only writeable by PermissionManageSystem if ExperimentalSettings.RestrictSystemAdmin is false.
//      // PermissionManageSystem can always read the value.
//      Product bool `access:write_restrictable`
//  }
type Config struct {
	ServiceSettings           ServiceSettings
	TeamSettings              TeamSettings
	ClientRequirements        ClientRequirements
	SQLSettings               SQLSettings
	LogSettings               LogSettings
	ExperimentalAuditSettings ExperimentalAuditSettings
	NotificationLogSettings   NotificationLogSettings
	PasswordSettings          PasswordSettings
	FileSettings              FileSettings
	EmailSettings             EmailSettings
	RateLimitSettings         RateLimitSettings
	PrivacySettings           PrivacySettings
	SupportSettings           SupportSettings
	AnnouncementSettings      AnnouncementSettings
	ThemeSettings             ThemeSettings
	GitLabSettings            SSOSettings
	GoogleSettings            SSOSettings
	Office365Settings         Office365Settings
	OpenIDSettings            SSOSettings
	LdapSettings              LdapSettings
	ComplianceSettings        ComplianceSettings
	LocalizationSettings      LocalizationSettings
	SamlSettings              SamlSettings
	NativeAppSettings         NativeAppSettings
	ClusterSettings           ClusterSettings
	MetricsSettings           MetricsSettings
	ExperimentalSettings      ExperimentalSettings
	AnalyticsSettings         AnalyticsSettings
	ElasticsearchSettings     ElasticsearchSettings
	BleveSettings             BleveSettings
	DataRetentionSettings     DataRetentionSettings
	MessageExportSettings     MessageExportSettings
	JobSettings               JobSettings // telemetry: none
	PluginSettings            PluginSettings
	DisplaySettings           DisplaySettings
	GuestAccountsSettings     GuestAccountsSettings
	ImageProxySettings        ImageProxySettings
	CloudSettings             CloudSettings  // telemetry: none
	FeatureFlags              *FeatureFlags  `access:"*_read" json:",omitempty"`
	ImportSettings            ImportSettings // telemetry: none
	ExportSettings            ExportSettings
}

func (o *Config) Clone() *Config {
	var ret Config
	if err := json.Unmarshal([]byte(o.ToJSON()), &ret); err != nil {
		panic(err)
	}
	return &ret
}

func (o *Config) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *Config) ToJSONFiltered(tagType, tagValue string) string {
	filteredConfigMap := structToMapFilteredByTag(*o, tagType, tagValue)
	for key, value := range filteredConfigMap {
		v, ok := value.(map[string]interface{})
		if ok && len(v) == 0 {
			delete(filteredConfigMap, key)
		}
	}
	b, _ := json.Marshal(filteredConfigMap)
	return string(b)
}

func (o *Config) GetSSOService(service string) *SSOSettings {
	switch service {
	case ServiceGitlab:
		return &o.GitLabSettings
	case ServiceGoogle:
		return &o.GoogleSettings
	case ServiceOffice365:
		return o.Office365Settings.SSOSettings()
	case ServiceOpenid:
		return &o.OpenIDSettings
	}

	return nil
}

func ConfigFromJSON(data io.Reader) *Config {
	var o *Config
	json.NewDecoder(data).Decode(&o)
	return o
}

// isUpdate detects a pre-existing config based on whether SiteURL has been changed
func (o *Config) isUpdate() bool {
	return o.ServiceSettings.SiteURL != nil
}

func (o *Config) SetDefaults() {
	isUpdate := o.isUpdate()

	o.LdapSettings.SetDefaults()
	o.SamlSettings.SetDefaults()

	if o.TeamSettings.TeammateNameDisplay == nil {
		o.TeamSettings.TeammateNameDisplay = NewString(ShowUsername)

		if *o.SamlSettings.Enable || *o.LdapSettings.Enable {
			*o.TeamSettings.TeammateNameDisplay = ShowFullName
		}
	}

	o.SQLSettings.SetDefaults(isUpdate)
	o.FileSettings.SetDefaults(isUpdate)
	o.EmailSettings.SetDefaults(isUpdate)
	o.PrivacySettings.setDefaults()
	o.Office365Settings.setDefaults()
	o.Office365Settings.setDefaults()
	o.GitLabSettings.setDefaults("", "", "", "", "")
	o.GoogleSettings.setDefaults(GoogleSettingsDefaultScope, GoogleSettingsDefaultAuthEndpoint, GoogleSettingsDefaultTokenEndpoint, GoogleSettingsDefaultUserAPIEndpoint, "")
	o.OpenIDSettings.setDefaults(OpenidSettingsDefaultScope, "", "", "", "#145DBF")
	o.ServiceSettings.SetDefaults(isUpdate)
	o.PasswordSettings.SetDefaults()
	o.TeamSettings.SetDefaults()
	o.MetricsSettings.SetDefaults()
	o.ExperimentalSettings.SetDefaults()
	o.SupportSettings.SetDefaults()
	o.AnnouncementSettings.SetDefaults()
	o.ThemeSettings.SetDefaults()
	o.ClusterSettings.SetDefaults()
	o.PluginSettings.SetDefaults(o.LogSettings)
	o.AnalyticsSettings.SetDefaults()
	o.ComplianceSettings.SetDefaults()
	o.LocalizationSettings.SetDefaults()
	o.ElasticsearchSettings.SetDefaults()
	o.BleveSettings.SetDefaults()
	o.NativeAppSettings.SetDefaults()
	o.DataRetentionSettings.SetDefaults()
	o.RateLimitSettings.SetDefaults()
	o.LogSettings.SetDefaults()
	o.ExperimentalAuditSettings.SetDefaults()
	o.NotificationLogSettings.SetDefaults()
	o.JobSettings.SetDefaults()
	o.MessageExportSettings.SetDefaults()
	o.DisplaySettings.SetDefaults()
	o.GuestAccountsSettings.SetDefaults()
	o.ImageProxySettings.SetDefaults(o.ServiceSettings)
	o.CloudSettings.SetDefaults()
	if o.FeatureFlags == nil {
		o.FeatureFlags = &FeatureFlags{}
		o.FeatureFlags.SetDefaults()
	}
	o.ImportSettings.SetDefaults()
	o.ExportSettings.SetDefaults()
}

func (o *Config) IsValid() *AppError {
	if *o.ServiceSettings.SiteURL == "" && *o.EmailSettings.EnableEmailBatching {
		return NewAppError("Config.IsValid", "model.config.is_valid.site_url_email_batching.app_error", nil, "", http.StatusBadRequest)
	}

	if *o.ClusterSettings.Enable && *o.EmailSettings.EnableEmailBatching {
		return NewAppError("Config.IsValid", "model.config.is_valid.cluster_email_batching.app_error", nil, "", http.StatusBadRequest)
	}

	if *o.ServiceSettings.SiteURL == "" && *o.ServiceSettings.AllowCookiesForSubdomains {
		return NewAppError("Config.IsValid", "model.config.is_valid.allow_cookies_for_subdomains.app_error", nil, "", http.StatusBadRequest)
	}

	if err := o.TeamSettings.isValid(); err != nil {
		return err
	}

	if err := o.SQLSettings.isValid(); err != nil {
		return err
	}

	if err := o.FileSettings.isValid(); err != nil {
		return err
	}

	if err := o.EmailSettings.isValid(); err != nil {
		return err
	}

	if err := o.LdapSettings.isValid(); err != nil {
		return err
	}

	if err := o.SamlSettings.isValid(); err != nil {
		return err
	}

	if *o.PasswordSettings.MinimumLength < PasswordMinimumLength || *o.PasswordSettings.MinimumLength > PasswordMaximumLength {
		return NewAppError("Config.IsValid", "model.config.is_valid.password_length.app_error", map[string]interface{}{"MinLength": PasswordMinimumLength, "MaxLength": PasswordMaximumLength}, "", http.StatusBadRequest)
	}

	if err := o.RateLimitSettings.isValid(); err != nil {
		return err
	}

	if err := o.ServiceSettings.isValid(); err != nil {
		return err
	}

	if err := o.ElasticsearchSettings.isValid(); err != nil {
		return err
	}

	if err := o.BleveSettings.isValid(); err != nil {
		return err
	}

	if err := o.DataRetentionSettings.isValid(); err != nil {
		return err
	}

	if err := o.LocalizationSettings.isValid(); err != nil {
		return err
	}

	if err := o.MessageExportSettings.isValid(); err != nil {
		return err
	}

	if err := o.DisplaySettings.isValid(); err != nil {
		return err
	}

	if err := o.ImageProxySettings.isValid(); err != nil {
		return err
	}

	if err := o.ImportSettings.isValid(); err != nil {
		return err
	}
	return nil
}

func (s *TeamSettings) isValid() *AppError {
	if *s.MaxUsersPerTeam <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.max_users.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxChannelsPerTeam <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.max_channels.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxNotificationsPerChannel <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.max_notify_per_channel.app_error", nil, "", http.StatusBadRequest)
	}

	if !(*s.RestrictDirectMessage == DirectMessageAny || *s.RestrictDirectMessage == DirectMessageTeam) {
		return NewAppError("Config.IsValid", "model.config.is_valid.restrict_direct_message.app_error", nil, "", http.StatusBadRequest)
	}

	if !(*s.TeammateNameDisplay == ShowFullName || *s.TeammateNameDisplay == ShowNicknameFullName || *s.TeammateNameDisplay == ShowUsername) {
		return NewAppError("Config.IsValid", "model.config.is_valid.teammate_name_display.app_error", nil, "", http.StatusBadRequest)
	}

	if len(*s.SiteName) > SitenameMaxLength {
		return NewAppError("Config.IsValid", "model.config.is_valid.sitename_length.app_error", map[string]interface{}{"MaxLength": SitenameMaxLength}, "", http.StatusBadRequest)
	}

	return nil
}

func (s *SQLSettings) isValid() *AppError {
	if *s.AtRestEncryptKey != "" && len(*s.AtRestEncryptKey) < 32 {
		return NewAppError("Config.IsValid", "model.config.is_valid.encrypt_sql.app_error", nil, "", http.StatusBadRequest)
	}

	if !(*s.DriverName == DatabaseDriverMysql || *s.DriverName == DatabaseDriverPostgres) {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_driver.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxIdleConns <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_idle.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.ConnMaxLifetimeMilliseconds < 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_conn_max_lifetime_milliseconds.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.ConnMaxIdleTimeMilliseconds < 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_conn_max_idle_time_milliseconds.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.QueryTimeout <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_query_timeout.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.DataSource == "" {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_data_src.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxOpenConns <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.sql_max_conn.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *FileSettings) isValid() *AppError {
	if *s.MaxFileSize <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.max_file_size.app_error", nil, "", http.StatusBadRequest)
	}

	if !(*s.DriverName == ImageDriverLocal || *s.DriverName == ImageDriverS3) {
		return NewAppError("Config.IsValid", "model.config.is_valid.file_driver.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.PublicLinkSalt != "" && len(*s.PublicLinkSalt) < 32 {
		return NewAppError("Config.IsValid", "model.config.is_valid.file_salt.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.Directory == "" {
		return NewAppError("Config.IsValid", "model.config.is_valid.directory.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *EmailSettings) isValid() *AppError {
	if !(*s.ConnectionSecurity == ConnSecurityNone || *s.ConnectionSecurity == ConnSecurityTls || *s.ConnectionSecurity == ConnSecurityStarttls || *s.ConnectionSecurity == ConnSecurityPlain) {
		return NewAppError("Config.IsValid", "model.config.is_valid.email_security.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.EmailBatchingBufferSize <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.email_batching_buffer_size.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.EmailBatchingInterval < 30 {
		return NewAppError("Config.IsValid", "model.config.is_valid.email_batching_interval.app_error", nil, "", http.StatusBadRequest)
	}

	if !(*s.EmailNotificationContentsType == EmailNotificationContentsFull || *s.EmailNotificationContentsType == EmailNotificationContentsGeneric) {
		return NewAppError("Config.IsValid", "model.config.is_valid.email_notification_contents_type.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *RateLimitSettings) isValid() *AppError {
	if *s.MemoryStoreSize <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.rate_mem.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.PerSec <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.rate_sec.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxBurst <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.max_burst.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *LdapSettings) isValid() *AppError {
	if !(*s.ConnectionSecurity == ConnSecurityNone || *s.ConnectionSecurity == ConnSecurityTls || *s.ConnectionSecurity == ConnSecurityStarttls) {
		return NewAppError("Config.IsValid", "model.config.is_valid.ldap_security.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.SyncIntervalMinutes <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.ldap_sync_interval.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaxPageSize < 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.ldap_max_page_size.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.Enable {
		if *s.LdapServer == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_server", nil, "", http.StatusBadRequest)
		}

		if *s.BaseDN == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_basedn", nil, "", http.StatusBadRequest)
		}

		if *s.EmailAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_email", nil, "", http.StatusBadRequest)
		}

		if *s.UsernameAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_username", nil, "", http.StatusBadRequest)
		}

		if *s.IDAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_id", nil, "", http.StatusBadRequest)
		}

		if *s.LoginIDAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.ldap_login_id", nil, "", http.StatusBadRequest)
		}

		if *s.UserFilter != "" {
			if _, err := ldap.CompileFilter(*s.UserFilter); err != nil {
				return NewAppError("ValidateFilter", "ent.ldap.validate_filter.app_error", nil, err.Error(), http.StatusBadRequest)
			}
		}

		if *s.GuestFilter != "" {
			if _, err := ldap.CompileFilter(*s.GuestFilter); err != nil {
				return NewAppError("LdapSettings.isValid", "ent.ldap.validate_guest_filter.app_error", nil, err.Error(), http.StatusBadRequest)
			}
		}

		if *s.AdminFilter != "" {
			if _, err := ldap.CompileFilter(*s.AdminFilter); err != nil {
				return NewAppError("LdapSettings.isValid", "ent.ldap.validate_admin_filter.app_error", nil, err.Error(), http.StatusBadRequest)
			}
		}
	}

	return nil
}

func (s *SamlSettings) isValid() *AppError {
	if *s.Enable {
		if *s.IDpURL == "" || !IsValidHTTPURL(*s.IDpURL) {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_idp_url.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.IDpDescriptorURL == "" || !IsValidHTTPURL(*s.IDpDescriptorURL) {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_idp_descriptor_url.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.IDpCertificateFile == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_idp_cert.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.EmailAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_email_attribute.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.UsernameAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_username_attribute.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.ServiceProviderIdentifier == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_spidentifier_attribute.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.Verify {
			if *s.AssertionConsumerServiceURL == "" || !IsValidHTTPURL(*s.AssertionConsumerServiceURL) {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_assertion_consumer_service_url.app_error", nil, "", http.StatusBadRequest)
			}
		}

		if *s.Encrypt {
			if *s.PrivateKeyFile == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_private_key.app_error", nil, "", http.StatusBadRequest)
			}

			if *s.PublicCertificateFile == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_public_cert.app_error", nil, "", http.StatusBadRequest)
			}
		}

		if *s.EmailAttribute == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_email_attribute.app_error", nil, "", http.StatusBadRequest)
		}

		if !(*s.SignatureAlgorithm == SamlSettingsSignatureAlgorithmSha1 || *s.SignatureAlgorithm == SamlSettingsSignatureAlgorithmSha256 || *s.SignatureAlgorithm == SamlSettingsSignatureAlgorithmSha512) {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_signature_algorithm.app_error", nil, "", http.StatusBadRequest)
		}
		if !(*s.CanonicalAlgorithm == SamlSettingsCanonicalAlgorithmC14n || *s.CanonicalAlgorithm == SamlSettingsCanonicalAlgorithmC14n11) {
			return NewAppError("Config.IsValid", "model.config.is_valid.saml_canonical_algorithm.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.GuestAttribute != "" {
			if !(strings.Contains(*s.GuestAttribute, "=")) {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_guest_attribute.app_error", nil, "", http.StatusBadRequest)
			}
			if len(strings.Split(*s.GuestAttribute, "=")) != 2 {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_guest_attribute.app_error", nil, "", http.StatusBadRequest)
			}
		}

		if *s.AdminAttribute != "" {
			if !(strings.Contains(*s.AdminAttribute, "=")) {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_admin_attribute.app_error", nil, "", http.StatusBadRequest)
			}
			if len(strings.Split(*s.AdminAttribute, "=")) != 2 {
				return NewAppError("Config.IsValid", "model.config.is_valid.saml_admin_attribute.app_error", nil, "", http.StatusBadRequest)
			}
		}
	}

	return nil
}

func (s *ServiceSettings) isValid() *AppError {
	if !(*s.ConnectionSecurity == ConnSecurityNone || *s.ConnectionSecurity == ConnSecurityTls) {
		return NewAppError("Config.IsValid", "model.config.is_valid.webserver_security.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.ConnectionSecurity == ConnSecurityTls && !*s.UseLetsEncrypt {
		appErr := NewAppError("Config.IsValid", "model.config.is_valid.tls_cert_file_missing.app_error", nil, "", http.StatusBadRequest)

		if *s.TLSCertFile == "" {
			return appErr
		} else if _, err := os.Stat(*s.TLSCertFile); os.IsNotExist(err) {
			return appErr
		}

		appErr = NewAppError("Config.IsValid", "model.config.is_valid.tls_key_file_missing.app_error", nil, "", http.StatusBadRequest)

		if *s.TLSKeyFile == "" {
			return appErr
		} else if _, err := os.Stat(*s.TLSKeyFile); os.IsNotExist(err) {
			return appErr
		}
	}

	if len(s.TLSOverwriteCiphers) > 0 {
		for _, cipher := range s.TLSOverwriteCiphers {
			if _, ok := ServerTLSSupportedCiphers[cipher]; !ok {
				return NewAppError("Config.IsValid", "model.config.is_valid.tls_overwrite_cipher.app_error", map[string]interface{}{"name": cipher}, "", http.StatusBadRequest)
			}
		}
	}

	if *s.ReadTimeout <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.read_timeout.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.WriteTimeout <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.write_timeout.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.TimeBetweenUserTypingUpdatesMilliseconds < 1000 {
		return NewAppError("Config.IsValid", "model.config.is_valid.time_between_user_typing.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.MaximumLoginAttempts <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.login_attempts.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.SiteURL != "" {
		if _, err := url.ParseRequestURI(*s.SiteURL); err != nil {
			return NewAppError("Config.IsValid", "model.config.is_valid.site_url.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if *s.WebsocketURL != "" {
		if _, err := url.ParseRequestURI(*s.WebsocketURL); err != nil {
			return NewAppError("Config.IsValid", "model.config.is_valid.websocket_url.app_error", nil, "", http.StatusBadRequest)
		}
	}

	host, port, _ := net.SplitHostPort(*s.ListenAddress)
	var isValidHost bool
	if host == "" {
		isValidHost = true
	} else {
		isValidHost = (net.ParseIP(host) != nil) || IsDomainName(host)
	}
	portInt, err := strconv.Atoi(port)
	if err != nil || !isValidHost || portInt < 0 || portInt > math.MaxUint16 {
		return NewAppError("Config.IsValid", "model.config.is_valid.listen_address.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.ExperimentalGroupUnreadChannels != GroupUnreadChannelsDisabled &&
		*s.ExperimentalGroupUnreadChannels != GroupUnreadChannelsDefaultOn &&
		*s.ExperimentalGroupUnreadChannels != GroupUnreadChannelsDefaultOff {
		return NewAppError("Config.IsValid", "model.config.is_valid.group_unread_channels.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.CollapsedThreads != CollapsedThreadsDisabled &&
		*s.CollapsedThreads != CollapsedThreadsDefaultOn &&
		*s.CollapsedThreads != CollapsedThreadsDefaultOff {
		return NewAppError("Config.IsValid", "model.config.is_valid.collapsed_threads.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *ElasticsearchSettings) isValid() *AppError {
	if *s.EnableIndexing {
		if *s.ConnectionURL == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.connection_url.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if *s.EnableSearching && !*s.EnableIndexing {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.enable_searching.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.EnableAutocomplete && !*s.EnableIndexing {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.enable_autocomplete.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.AggregatePostsAfterDays < 1 {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.aggregate_posts_after_days.app_error", nil, "", http.StatusBadRequest)
	}

	if _, err := time.Parse("15:04", *s.PostsAggregatorJobStartTime); err != nil {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.posts_aggregator_job_start_time.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	if *s.LiveIndexingBatchSize < 1 {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.live_indexing_batch_size.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.BulkIndexingTimeWindowSeconds < 1 {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.bulk_indexing_time_window_seconds.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.RequestTimeoutSeconds < 1 {
		return NewAppError("Config.IsValid", "model.config.is_valid.elastic_search.request_timeout_seconds.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (bs *BleveSettings) isValid() *AppError {
	if *bs.EnableIndexing {
		if *bs.IndexDir == "" {
			return NewAppError("Config.IsValid", "model.config.is_valid.bleve_search.filename.app_error", nil, "", http.StatusBadRequest)
		}
	} else {
		if *bs.EnableSearching {
			return NewAppError("Config.IsValid", "model.config.is_valid.bleve_search.enable_searching.app_error", nil, "", http.StatusBadRequest)
		}
		if *bs.EnableAutocomplete {
			return NewAppError("Config.IsValid", "model.config.is_valid.bleve_search.enable_autocomplete.app_error", nil, "", http.StatusBadRequest)
		}
	}
	if *bs.BulkIndexingTimeWindowSeconds < 1 {
		return NewAppError("Config.IsValid", "model.config.is_valid.bleve_search.bulk_indexing_time_window_seconds.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (s *DataRetentionSettings) isValid() *AppError {
	if *s.MessageRetentionDays <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.data_retention.message_retention_days_too_low.app_error", nil, "", http.StatusBadRequest)
	}

	if *s.FileRetentionDays <= 0 {
		return NewAppError("Config.IsValid", "model.config.is_valid.data_retention.file_retention_days_too_low.app_error", nil, "", http.StatusBadRequest)
	}

	if _, err := time.Parse("15:04", *s.DeletionJobStartTime); err != nil {
		return NewAppError("Config.IsValid", "model.config.is_valid.data_retention.deletion_job_start_time.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	return nil
}

func (s *LocalizationSettings) isValid() *AppError {
	if *s.AvailableLocales != "" {
		if !strings.Contains(*s.AvailableLocales, *s.DefaultClientLocale) {
			return NewAppError("Config.IsValid", "model.config.is_valid.localization.available_locales.app_error", nil, "", http.StatusBadRequest)
		}
	}

	return nil
}

func (s *MessageExportSettings) isValid() *AppError {
	if s.EnableExport == nil {
		return NewAppError("Config.IsValid", "model.config.is_valid.message_export.enable.app_error", nil, "", http.StatusBadRequest)
	}
	if *s.EnableExport {
		if s.ExportFromTimestamp == nil || *s.ExportFromTimestamp < 0 || *s.ExportFromTimestamp > GetMillis() {
			return NewAppError("Config.IsValid", "model.config.is_valid.message_export.export_from.app_error", nil, "", http.StatusBadRequest)
		} else if s.DailyRunTime == nil {
			return NewAppError("Config.IsValid", "model.config.is_valid.message_export.daily_runtime.app_error", nil, "", http.StatusBadRequest)
		} else if _, err := time.Parse("15:04", *s.DailyRunTime); err != nil {
			return NewAppError("Config.IsValid", "model.config.is_valid.message_export.daily_runtime.app_error", nil, err.Error(), http.StatusBadRequest)
		} else if s.BatchSize == nil || *s.BatchSize < 0 {
			return NewAppError("Config.IsValid", "model.config.is_valid.message_export.batch_size.app_error", nil, "", http.StatusBadRequest)
		} else if s.ExportFormat == nil || (*s.ExportFormat != ComplianceExportTypeActiance && *s.ExportFormat != ComplianceExportTypeGlobalrelay && *s.ExportFormat != ComplianceExportTypeCsv) {
			return NewAppError("Config.IsValid", "model.config.is_valid.message_export.export_type.app_error", nil, "", http.StatusBadRequest)
		}

		if *s.ExportFormat == ComplianceExportTypeGlobalrelay {
			if s.GlobalRelaySettings == nil {
				return NewAppError("Config.IsValid", "model.config.is_valid.message_export.global_relay.config_missing.app_error", nil, "", http.StatusBadRequest)
			} else if s.GlobalRelaySettings.CustomerType == nil || (*s.GlobalRelaySettings.CustomerType != GlobalrelayCustomerTypeA9 && *s.GlobalRelaySettings.CustomerType != GlobalrelayCustomerTypeA10) {
				return NewAppError("Config.IsValid", "model.config.is_valid.message_export.global_relay.customer_type.app_error", nil, "", http.StatusBadRequest)
			} else if s.GlobalRelaySettings.EmailAddress == nil || !strings.Contains(*s.GlobalRelaySettings.EmailAddress, "@") {
				// validating email addresses is hard - just make sure it contains an '@' sign
				// see https://stackoverflow.com/questions/201323/using-a-regular-expression-to-validate-an-email-address
				return NewAppError("Config.IsValid", "model.config.is_valid.message_export.global_relay.email_address.app_error", nil, "", http.StatusBadRequest)
			} else if s.GlobalRelaySettings.SmtpUsername == nil || *s.GlobalRelaySettings.SmtpUsername == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.message_export.global_relay.smtp_username.app_error", nil, "", http.StatusBadRequest)
			} else if s.GlobalRelaySettings.SmtpPassword == nil || *s.GlobalRelaySettings.SmtpPassword == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.message_export.global_relay.smtp_password.app_error", nil, "", http.StatusBadRequest)
			}
		}
	}
	return nil
}

func (s *DisplaySettings) isValid() *AppError {
	if len(s.CustomURLSchemes) != 0 {
		validProtocolPattern := regexp.MustCompile(`(?i)^\s*[A-Za-z][A-Za-z0-9.+-]*\s*$`)

		for _, scheme := range s.CustomURLSchemes {
			if !validProtocolPattern.MatchString(scheme) {
				return NewAppError(
					"Config.IsValid",
					"model.config.is_valid.display.custom_url_schemes.app_error",
					map[string]interface{}{"Scheme": scheme},
					"",
					http.StatusBadRequest,
				)
			}
		}
	}

	return nil
}

func (s *ImageProxySettings) isValid() *AppError {
	if *s.Enable {
		switch *s.ImageProxyType {
		case ImageProxyTypeLocal:
			// No other settings to validate
		case ImageProxyTypeAtmosCamo:
			if *s.RemoteImageProxyURL == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.atmos_camo_image_proxy_url.app_error", nil, "", http.StatusBadRequest)
			}

			if *s.RemoteImageProxyOptions == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.atmos_camo_image_proxy_options.app_error", nil, "", http.StatusBadRequest)
			}
		default:
			return NewAppError("Config.IsValid", "model.config.is_valid.image_proxy_type.app_error", nil, "", http.StatusBadRequest)
		}
	}

	return nil
}

func (o *Config) GetSanitizeOptions() map[string]bool {
	options := map[string]bool{}
	options["fullname"] = *o.PrivacySettings.ShowFullName
	options["email"] = *o.PrivacySettings.ShowEmailAddress

	return options
}

func (o *Config) Sanitize() {
	if o.LdapSettings.BindPassword != nil && *o.LdapSettings.BindPassword != "" {
		*o.LdapSettings.BindPassword = FakeSetting
	}

	*o.FileSettings.PublicLinkSalt = FakeSetting

	if *o.FileSettings.AmazonS3SecretAccessKey != "" {
		*o.FileSettings.AmazonS3SecretAccessKey = FakeSetting
	}

	if o.EmailSettings.SMTPPassword != nil && *o.EmailSettings.SMTPPassword != "" {
		*o.EmailSettings.SMTPPassword = FakeSetting
	}

	if *o.GitLabSettings.Secret != "" {
		*o.GitLabSettings.Secret = FakeSetting
	}

	if o.GoogleSettings.Secret != nil && *o.GoogleSettings.Secret != "" {
		*o.GoogleSettings.Secret = FakeSetting
	}

	if o.Office365Settings.Secret != nil && *o.Office365Settings.Secret != "" {
		*o.Office365Settings.Secret = FakeSetting
	}

	if o.OpenIDSettings.Secret != nil && *o.OpenIDSettings.Secret != "" {
		*o.OpenIDSettings.Secret = FakeSetting
	}

	*o.SQLSettings.DataSource = FakeSetting
	*o.SQLSettings.AtRestEncryptKey = FakeSetting

	*o.ElasticsearchSettings.Password = FakeSetting

	for i := range o.SQLSettings.DataSourceReplicas {
		o.SQLSettings.DataSourceReplicas[i] = FakeSetting
	}

	for i := range o.SQLSettings.DataSourceSearchReplicas {
		o.SQLSettings.DataSourceSearchReplicas[i] = FakeSetting
	}

	if o.MessageExportSettings.GlobalRelaySettings.SmtpPassword != nil && *o.MessageExportSettings.GlobalRelaySettings.SmtpPassword != "" {
		*o.MessageExportSettings.GlobalRelaySettings.SmtpPassword = FakeSetting
	}

	if o.ServiceSettings.GfycatAPISecret != nil && *o.ServiceSettings.GfycatAPISecret != "" {
		*o.ServiceSettings.GfycatAPISecret = FakeSetting
	}

	*o.ServiceSettings.SplitKey = FakeSetting
}

// structToMapFilteredByTag converts a struct into a map removing those fields that has the tag passed
// as argument
func structToMapFilteredByTag(t interface{}, typeOfTag, filterTag string) map[string]interface{} {
	defer func() {
		if r := recover(); r != nil {
			mlog.Warn("Panicked in structToMapFilteredByTag. This should never happen.", mlog.Any("recover", r))
		}
	}()

	val := reflect.ValueOf(t)
	elemField := reflect.TypeOf(t)

	if val.Kind() != reflect.Struct {
		return nil
	}

	out := map[string]interface{}{}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		structField := elemField.Field(i)
		tagPermissions := strings.Split(structField.Tag.Get(typeOfTag), ",")
		if isTagPresent(filterTag, tagPermissions) {
			continue
		}

		var value interface{}

		switch field.Kind() {
		case reflect.Struct:
			value = structToMapFilteredByTag(field.Interface(), typeOfTag, filterTag)
		case reflect.Ptr:
			indirectType := field.Elem()
			if indirectType.Kind() == reflect.Struct {
				value = structToMapFilteredByTag(indirectType.Interface(), typeOfTag, filterTag)
			} else if indirectType.Kind() != reflect.Invalid {
				value = indirectType.Interface()
			}
		default:
			value = field.Interface()
		}

		out[val.Type().Field(i).Name] = value
	}

	return out
}

func isTagPresent(tag string, tags []string) bool {
	for _, val := range tags {
		tagValue := strings.TrimSpace(val)
		if tagValue != "" && tagValue == tag {
			return true
		}
	}

	return false
}
