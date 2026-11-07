// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable max-lines */

import type {AccessControlPolicy, CELExpressionError, AccessControlTestResult, AccessControlPoliciesResult, AccessControlPolicyChannelsResult, AccessControlVisualAST, AccessControlAttributes, AccessControlPolicyActiveUpdate, PolicySimulationResponse, PolicySimulationByUsersParams} from '@mattermost/types/access_control';
import type {ClusterInfo, AnalyticsRow, SchemaMigration, LogFilterQuery, LogsByNode} from '@mattermost/types/admin';
import type {Agent, LLMService} from '@mattermost/types/agents';
import type {AppBinding, AppCallRequest, AppCallResponse} from '@mattermost/types/apps';
import type {Audit} from '@mattermost/types/audits';
import type {UserAutocomplete, AutocompleteSuggestion} from '@mattermost/types/autocomplete';
import type {Bot, BotPatch} from '@mattermost/types/bots';
import type {ChannelBookmark, ChannelBookmarkCreate, ChannelBookmarkPatch, UpdateChannelBookmarkResponse} from '@mattermost/types/channel_bookmarks';
import type {ChannelCategory, OrderedChannelCategories} from '@mattermost/types/channel_categories';
import type {
    Channel,
    ChannelJoinRequest,
    ChannelJoinRequestApprovalResponse,
    ChannelJoinRequestList,
    ChannelJoinRequestPatch,
    ChannelMemberCountsByGroup,
    ChannelMembership,
    ChannelModeration,
    ChannelModerationPatch,
    ChannelStats,
    ChannelsWithTotalCount,
    ChannelUnread,
    ChannelViewResponse,
    ChannelWithTeamData,
    ChannelSearchOpts,
    GetChannelJoinRequestsOptions,
    ServerChannel,
} from '@mattermost/types/channels';
import type {Options, StatusOK, ClientResponse, FetchPaginatedThreadOptions, OptsSignalExt} from '@mattermost/types/client4';
import {LogLevel} from '@mattermost/types/client4';
import type {
    Product,
    CloudCustomer,
    Limits,
    NotifyAdminRequest,
    Subscription,
    ValidBusinessEmail,
    Installation,
    PreviewModalContentData,
} from '@mattermost/types/cloud';
import type {Compliance} from '@mattermost/types/compliance';
import type {
    ClientConfig,
    ClientLicense,
    DataRetentionPolicy,
    License,
    AdminConfig,
    EnvironmentConfig,
    RequestLicenseBody,
    AllowedIPRanges,
    AllowedIPRange,
    FetchIPResponse,
    LdapSettings, ContentFlaggingSettings,
} from '@mattermost/types/config';
import type {ContentFlaggingConfig} from '@mattermost/types/content_flagging';
import type {
    DataRetentionCustomPolicies,
    CreateDataRetentionCustomPolicy,
    PatchDataRetentionCustomPolicy,
    GetDataRetentionCustomPoliciesRequest,
} from '@mattermost/types/data_retention';
import type {DeliveryTrackingConfig} from '@mattermost/types/delivery_tracking';
import type {Draft} from '@mattermost/types/drafts';
import type {CustomEmoji} from '@mattermost/types/emojis';
import type {ServerError} from '@mattermost/types/errors';
import type {FileInfo, FileUploadResponse, FileSearchResults} from '@mattermost/types/files';
import type {SystemSetting} from '@mattermost/types/general';
import type {GlobalAttributeListing, GlobalAttributeListingSearch} from '@mattermost/types/global_attributes';
import type {
    Group,
    GroupPatch,
    GroupSyncable,
    MixedUnlinkedGroup,
    SyncablePatch,
    UsersWithGroupsAndCount,
    GroupsWithCount,
    GroupCreateWithUserIds,
    GroupSearchParams,
    CustomGroupPatch,
    GetGroupsParams,
    GetGroupsForUserParams,
    GroupStats,
    GroupMember,
} from '@mattermost/types/groups';
import type {HealthFindingFilter, HealthFindingList} from '@mattermost/types/health';
import type {PostActionResponse} from '@mattermost/types/integration_actions';
import type {
    Command,
    CommandArgs,
    CommandResponse,
    DialogSubmission,
    IncomingWebhook,
    IncomingWebhooksWithCount,
    OAuthApp,
    OutgoingOAuthConnection,
    OutgoingWebhook,
    SubmitDialogResponse,
} from '@mattermost/types/integrations';
import type {Job, JobType, JobTypeBase} from '@mattermost/types/jobs';
import type {ServerLimits} from '@mattermost/types/limits';
import type {
    MarketplaceApp,
    MarketplacePlugin,
} from '@mattermost/types/marketplace';
import type {MfaSecret} from '@mattermost/types/mfa';
import type {
    ClientPluginManifest,
    PluginManifest,
    PluginsResponse,
    PluginStatus,
} from '@mattermost/types/plugins';
import type {Post, PostList, PostSearchResults, PostsUsageResponse, TeamsUsageResponse, PaginatedPostList, FilesUsageResponse, PostAcknowledgement, PostAnalytics, PostInfo} from '@mattermost/types/posts';
import type {PreferenceType} from '@mattermost/types/preferences';
import type {ProductNotices} from '@mattermost/types/product_notices';
import type {ChannelAttributeComplianceSummary, ChannelAttributeNotifyResult, ChannelsMissingAttributeValueList, NameMappedPropertyFields, PropertyField, PropertyFieldOptionPage, PropertyFieldsQuery, PropertyValue} from '@mattermost/types/properties';
import type {UserPropertyField, UserPropertyFieldPatch} from '@mattermost/types/properties_user';
import type {Reaction} from '@mattermost/types/reactions';
import type {Recap, CreateRecapRequest, ScheduledRecap, ScheduledRecapInput, RecapLimitStatus} from '@mattermost/types/recaps';
import type {RemoteCluster, RemoteClusterAcceptInvite, RemoteClusterPatch, RemoteClusterWithPassword} from '@mattermost/types/remote_clusters';
import type {ActionSearchRequest, ActionSearchResponse} from '@mattermost/types/render_permissions';
import type {UserReport, UserReportFilter, UserReportOptions} from '@mattermost/types/reports';
import type {Role} from '@mattermost/types/roles';
import type {SamlCertificateStatus, SamlMetadataResponse} from '@mattermost/types/saml';
import type {ScheduledPost} from '@mattermost/types/schedule_post';
import type {Scheme} from '@mattermost/types/schemes';
import type {Session} from '@mattermost/types/sessions';
import type {CompleteOnboardingRequest} from '@mattermost/types/setup';
import type {RemoteClusterInfo, SharedChannelRemote} from '@mattermost/types/shared_channels';
import type {
    GetTeamMembersOpts,
    MemberInviteProfile,
    Team,
    TeamInviteWithError,
    TeamMembership,
    TeamMemberWithError,
    TeamStats,
    TeamsWithCount,
    TeamUnread,
    TeamSearchOpts,
    PagedTeamSearchOpts,
    NotPagedTeamSearchOpts,
} from '@mattermost/types/teams';
import type {TermsOfService} from '@mattermost/types/terms_of_service';
import type {UserThreadList, UserThread, UserThreadWithPost} from '@mattermost/types/threads';
import type {
    AuthChangeResponse,
    UserAccessToken,
    UserAuthUpdate,
    UserProfile,
    UsersStats,
    UserStatus,
    GetFilteredUsersStatsOpts,
    UserCustomStatus,
} from '@mattermost/types/users';
import type {DeepPartial, PartialExcept, RelationOneToOne} from '@mattermost/types/utilities';

import {cleanUrlForLogging} from './errors';
import {buildQueryString, extractFilenameFromContentDisposition} from './helpers';

export enum LdapDiagnosticTestType {
    FILTERS = 'filters',
    ATTRIBUTES = 'attributes',
    GROUP_ATTRIBUTES = 'group_attributes',
}

const HEADER_AUTH = 'Authorization';
const HEADER_BEARER = 'BEARER';
const HEADER_CONTENT_TYPE = 'Content-Type';
const HEADER_REQUESTED_WITH = 'X-Requested-With';
const HEADER_USER_AGENT = 'User-Agent';
export const HEADER_X_CLUSTER_ID = 'X-Cluster-Id';
const HEADER_X_CSRF_TOKEN = 'X-CSRF-Token';
export const HEADER_X_VERSION_ID = 'X-Version-Id';
const LOGS_PER_PAGE_DEFAULT = 10000;
const AUTOCOMPLETE_LIMIT_DEFAULT = 25;
const PER_PAGE_DEFAULT = 60;

// The largest page the options endpoint serves (server
// `model.PropertyFieldOptionsMaxPerRequest`). Sent on every options request:
// the endpoint's own default is PER_PAGE_DEFAULT, which would page a hierarchy
// 60 options at a time.
const PROPERTY_FIELD_OPTIONS_PER_PAGE_DEFAULT = 200;

export const DEFAULT_LIMIT_BEFORE = 30;
export const DEFAULT_LIMIT_AFTER = 30;

export default class Client4 {
    logToConsole = false;
    serverVersion = '';
    clusterId = '';
    token = '';
    csrf = '';
    url = '';
    urlVersion = '/api/v4';
    userAgent: string | null = null;
    enableLogging = false;
    defaultHeaders: {[x: string]: string} = {};
    userId = '';
    diagnosticId = '';
    includeCookies = true;
    setAuthHeader = true;
    translations = {
        connectionError: 'There appears to be a problem with your internet connection.',
        unknownError: 'We received an unexpected status code from the server.',
    };
    userRoles = '';

    getUrl() {
        return this.url;
    }

    getAbsoluteUrl(baseUrl: string) {
        if (typeof baseUrl !== 'string' || !baseUrl.startsWith('/')) {
            return baseUrl;
        }
        return this.getUrl() + baseUrl;
    }

    setUrl(url: string) {
        this.url = url;
    }

    setUserAgent(userAgent: string) {
        this.userAgent = userAgent;
    }

    getToken() {
        return this.token;
    }

    setToken(token: string) {
        this.token = token;
    }

    setCSRF(csrfToken: string) {
        this.csrf = csrfToken;
    }

    setAcceptLanguage(locale: string) {
        this.defaultHeaders['Accept-Language'] = locale;
    }

    setHeader(header: string, value: string) {
        this.defaultHeaders[header] = value;
    }

    removeHeader(header: string) {
        delete this.defaultHeaders[header];
    }

    setEnableLogging(enable: boolean) {
        this.enableLogging = enable;
    }

    setIncludeCookies(include: boolean) {
        this.includeCookies = include;
    }

    setUserId(userId: string) {
        this.userId = userId;
    }

    setUserRoles(roles: string) {
        this.userRoles = roles;
    }

    setDiagnosticId(diagnosticId: string) {
        this.diagnosticId = diagnosticId;
    }

    getServerVersion() {
        return this.serverVersion;
    }

    getUrlVersion() {
        return this.urlVersion;
    }

    getBaseRoute() {
        return `${this.url}${this.urlVersion}`;
    }

    getAppsProxyRoute() {
        return `${this.url}/plugins/com.mattermost.apps`;
    }

    getUsersRoute() {
        return `${this.getBaseRoute()}/users`;
    }

    getUserRoute(userId: string) {
        return `${this.getUsersRoute()}/${userId}`;
    }

    getTeamsRoute() {
        return `${this.getBaseRoute()}/teams`;
    }

    getTeamRoute(teamId: string) {
        return `${this.getTeamsRoute()}/${teamId}`;
    }

    getTeamSchemeRoute(teamId: string) {
        return `${this.getTeamRoute(teamId)}/scheme`;
    }

    getTeamNameRoute(teamName: string) {
        return `${this.getTeamsRoute()}/name/${teamName}`;
    }

    getTeamMembersRoute(teamId: string) {
        return `${this.getTeamRoute(teamId)}/members`;
    }

    getTeamMemberRoute(teamId: string, userId: string) {
        return `${this.getTeamMembersRoute(teamId)}/${userId}`;
    }

    getChannelsRoute() {
        return `${this.getBaseRoute()}/channels`;
    }

    getChannelRoute(channelId: string) {
        return `${this.getChannelsRoute()}/${channelId}`;
    }

    getChannelMembersRoute(channelId: string) {
        return `${this.getChannelRoute(channelId)}/members`;
    }

    getChannelMemberRoute(channelId: string, userId: string) {
        return `${this.getChannelMembersRoute(channelId)}/${userId}`;
    }

    getChannelSchemeRoute(channelId: string) {
        return `${this.getChannelRoute(channelId)}/scheme`;
    }
    getChannelBookmarksRoute(channelId: string) {
        return `${this.getChannelRoute(channelId)}/bookmarks`;
    }
    getChannelBookmarkRoute(channelId: string, bookmarkId: string) {
        return `${this.getChannelRoute(channelId)}/bookmarks/${bookmarkId}`;
    }
    getChannelJoinRequestRoute(channelId: string) {
        return `${this.getChannelRoute(channelId)}/join_request`;
    }
    getChannelJoinRequestsRoute(channelId: string) {
        return `${this.getChannelRoute(channelId)}/join_requests`;
    }

    getChannelCategoriesRoute(userId: string, teamId: string) {
        return `${this.getBaseRoute()}/users/${userId}/teams/${teamId}/channels/categories`;
    }

    getRemoteClustersRoute() {
        return `${this.getBaseRoute()}/remotecluster`;
    }

    getRemoteClusterRoute(remoteId: string) {
        return `${this.getRemoteClustersRoute()}/${remoteId}`;
    }

    getPropertyFieldsRoute(groupName: string, objectType: string) {
        return `${this.getBaseRoute()}/properties/groups/${groupName}/${objectType}/fields`;
    }

    getPropertyFieldRoute(groupName: string, objectType: string, fieldId: string) {
        return `${this.getPropertyFieldsRoute(groupName, objectType)}/${fieldId}`;
    }

    getPropertyFieldOptionsRoute(groupName: string, objectType: string, fieldId: string) {
        return `${this.getPropertyFieldRoute(groupName, objectType, fieldId)}/options`;
    }

    // channelAttributeMissingValuesRoute builds the route for the required-
    // channel-attribute compliance endpoints. An empty fieldId targets the
    // field-less "create mode" form, used before the attribute has been saved
    // and thus has no ID yet.
    channelAttributeMissingValuesRoute(groupName: string, fieldId?: string) {
        if (!fieldId) {
            return `${this.getPropertyFieldsRoute(groupName, 'channel')}/missing_values`;
        }
        return `${this.getPropertyFieldRoute(groupName, 'channel', fieldId)}/missing_values`;
    }

    getCustomProfileAttributeFieldsRoute() {
        return `${this.getBaseRoute()}/custom_profile_attributes/fields`;
    }

    getCustomProfileAttributeFieldRoute(propertyFieldId: string) {
        return `${this.getCustomProfileAttributeFieldsRoute()}/${propertyFieldId}`;
    }

    getCustomProfileAttributeValuesRoute() {
        return `${this.getBaseRoute()}/custom_profile_attributes/values`;
    }

    getPostsRoute() {
        return `${this.getBaseRoute()}/posts`;
    }

    getPostRoute(postId: string) {
        return `${this.getPostsRoute()}/${postId}`;
    }

    getReactionsRoute() {
        return `${this.getBaseRoute()}/reactions`;
    }

    getCommandsRoute() {
        return `${this.getBaseRoute()}/commands`;
    }

    getFilesRoute() {
        return `${this.getBaseRoute()}/files`;
    }

    getFileRoute(fileId: string) {
        return `${this.getFilesRoute()}/${fileId}`;
    }

    getPreferencesRoute(userId: string) {
        return `${this.getUserRoute(userId)}/preferences`;
    }

    getIncomingHooksRoute() {
        return `${this.getBaseRoute()}/hooks/incoming`;
    }

    getIncomingHookRoute(hookId: string) {
        return `${this.getBaseRoute()}/hooks/incoming/${hookId}`;
    }

    getOutgoingHooksRoute() {
        return `${this.getBaseRoute()}/hooks/outgoing`;
    }

    getOutgoingHookRoute(hookId: string) {
        return `${this.getBaseRoute()}/hooks/outgoing/${hookId}`;
    }

    getSharedChannelsRoute() {
        return `${this.getBaseRoute()}/sharedchannels`;
    }

    getOAuthRoute() {
        return `${this.url}/oauth`;
    }

    getOAuthAppsRoute() {
        return `${this.getBaseRoute()}/oauth/apps`;
    }

    getOAuthAppRoute(appId: string) {
        return `${this.getOAuthAppsRoute()}/${appId}`;
    }

    getOutgoingOAuthConnectionsRoute() {
        return `${this.getBaseRoute()}/oauth/outgoing_connections`;
    }

    getOutgoingOAuthConnectionRoute(connectionId: string) {
        return `${this.getBaseRoute()}/oauth/outgoing_connections/${connectionId}`;
    }

    getEmojisRoute() {
        return `${this.getBaseRoute()}/emoji`;
    }

    getEmojiRoute(emojiId: string) {
        return `${this.getEmojisRoute()}/${emojiId}`;
    }

    getBrandRoute() {
        return `${this.getBaseRoute()}/brand`;
    }

    getBrandImageUrl(timestamp: string) {
        return `${this.getBrandRoute()}/image?t=${timestamp}`;
    }

    getDataRetentionRoute() {
        return `${this.getBaseRoute()}/data_retention`;
    }

    getJobsRoute() {
        return `${this.getBaseRoute()}/jobs`;
    }

    getRecapsRoute() {
        return `${this.getBaseRoute()}/recaps`;
    }

    getScheduledRecapsRoute() {
        return `${this.getBaseRoute()}/scheduled_recaps`;
    }

    getPluginsRoute() {
        return `${this.getBaseRoute()}/plugins`;
    }

    getAgentsRoute() {
        return `${this.getBaseRoute()}/agents`;
    }

    getLLMServicesRoute() {
        return `${this.getBaseRoute()}/llmservices`;
    }

    getPluginRoute(pluginId: string) {
        return `${this.getPluginsRoute()}/${pluginId}`;
    }

    getPluginsMarketplaceRoute() {
        return `${this.getPluginsRoute()}/marketplace`;
    }

    getRolesRoute() {
        return `${this.getBaseRoute()}/roles`;
    }

    getSchemesRoute() {
        return `${this.getBaseRoute()}/schemes`;
    }

    getBotsRoute() {
        return `${this.getBaseRoute()}/bots`;
    }

    getBotRoute(botUserId: string) {
        return `${this.getBotsRoute()}/${botUserId}`;
    }

    getGroupsRoute() {
        return `${this.getBaseRoute()}/groups`;
    }

    getGroupRoute(groupID: string) {
        return `${this.getGroupsRoute()}/${groupID}`;
    }

    getNoticesRoute() {
        return `${this.getBaseRoute()}/system/notices`;
    }

    getCloudRoute() {
        return `${this.getBaseRoute()}/cloud`;
    }

    getUsageRoute() {
        return `${this.getBaseRoute()}/usage`;
    }

    getPermissionsRoute() {
        return `${this.getBaseRoute()}/permissions`;
    }

    getUserThreadsRoute(userID: string, teamID: string): string {
        return `${this.getUserRoute(userID)}/teams/${teamID}/threads`;
    }

    getUserThreadRoute(userId: string, teamId: string, threadId: string): string {
        return `${this.getUserThreadsRoute(userId, teamId)}/${threadId}`;
    }

    getSystemRoute(): string {
        return `${this.getBaseRoute()}/system`;
    }

    getDraftsRoute() {
        return `${this.getBaseRoute()}/drafts`;
    }

    getReportsRoute(): string {
        return `${this.getBaseRoute()}/reports`;
    }

    getLimitsRoute(): string {
        return `${this.getBaseRoute()}/limits`;
    }

    getServerLimitsRoute() {
        return `${this.getLimitsRoute()}/server`;
    }

    getClientMetricsRoute() {
        return `${this.getBaseRoute()}/client_perf`;
    }

    getContentFlaggingRoute() {
        return `${this.getBaseRoute()}/content_flagging`;
    }

    getDeliveryTrackingRoute() {
        return `${this.getBaseRoute()}/delivery_tracking`;
    }

    getHealthFindingsRoute() {
        return `${this.getBaseRoute()}/health/findings`;
    }

    getCSRFFromCookie() {
        if (typeof document !== 'undefined' && typeof document.cookie !== 'undefined') {
            const cookies = document.cookie.split(';');
            for (let i = 0; i < cookies.length; i++) {
                const cookie = cookies[i].trim();
                if (cookie.startsWith('MMCSRF=')) {
                    return cookie.replace('MMCSRF=', '');
                }
            }
        }
        return '';
    }

    getOptions(options: Options) {
        const newOptions: Options = {...options};

        const headers: {[x: string]: string} = {
            [HEADER_REQUESTED_WITH]: 'XMLHttpRequest',
            ...this.defaultHeaders,
        };

        if (this.setAuthHeader && this.token) {
            headers[HEADER_AUTH] = `${HEADER_BEARER} ${this.token}`;
        }

        const csrfToken = this.csrf || this.getCSRFFromCookie();
        if (options.method && options.method.toLowerCase() !== 'get' && csrfToken) {
            headers[HEADER_X_CSRF_TOKEN] = csrfToken;
        }

        if (this.includeCookies) {
            newOptions.credentials = 'include';
        }

        if (this.userAgent) {
            headers[HEADER_USER_AGENT] = this.userAgent;
        }

        if (!headers[HEADER_CONTENT_TYPE] && options.body) {
            // when the body is an instance of FormData we let browser set the Content-Type header generated by FormData interface with correct boundary
            if (!(options.body instanceof FormData)) {
                headers[HEADER_CONTENT_TYPE] = 'application/json';
            }
        }

        if (newOptions.headers) {
            Object.assign(headers, newOptions.headers);
        }

        return {
            ...newOptions,
            headers,
        };
    }

    // User Routes

    createUser = (user: UserProfile, token: string, inviteId: string, redirect?: string) => {
        const queryParams: any = {};

        if (token) {
            queryParams.t = token;
        }

        if (inviteId) {
            queryParams.iid = inviteId;
        }

        if (redirect) {
            queryParams.r = redirect;
        }

        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}${buildQueryString(queryParams)}`,
            {method: 'post', body: JSON.stringify(user)},
        );
    };

    patchMe = (userPatch: Partial<UserProfile>) => {
        return this.doFetch<UserProfile>(
            `${this.getUserRoute('me')}/patch`,
            {method: 'put', body: JSON.stringify(userPatch)},
        );
    };

    patchUser = (userPatch: PartialExcept<UserProfile, 'id'>) => {
        return this.doFetch<UserProfile>(
            `${this.getUserRoute(userPatch.id)}/patch`,
            {method: 'put', body: JSON.stringify(userPatch)},
        );
    };

    updateUser = (user: UserProfile) => {
        return this.doFetch<UserProfile>(
            `${this.getUserRoute(user.id)}`,
            {method: 'put', body: JSON.stringify(user)},
        );
    };

    updateUserAuth = (userId: string, userAuth: UserAuthUpdate) => {
        return this.doFetch<UserAuthUpdate>(
            `${this.getUserRoute(userId)}/auth`,
            {method: 'put', body: JSON.stringify(userAuth)},
        );
    };

    promoteGuestToUser = (userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/promote`,
            {method: 'post'},
        );
    };

    demoteUserToGuest = (userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/demote`,
            {method: 'post'},
        );
    };

    updateUserRoles = (userId: string, roles: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/roles`,
            {method: 'put', body: JSON.stringify({roles})},
        );
    };

    updateUserMfa = (userId: string, activate: boolean, code: string) => {
        const body: any = {
            activate,
        };

        if (activate) {
            body.code = code;
        }

        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/mfa`,
            {method: 'put', body: JSON.stringify(body)},
        );
    };

    updateUserPassword = (userId: string, currentPassword: string, newPassword: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/password`,
            {method: 'put', body: JSON.stringify({current_password: currentPassword, new_password: newPassword})},
        );
    };

    resetUserPassword = (token: string, newPassword: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/password/reset`,
            {method: 'post', body: JSON.stringify({token, new_password: newPassword})},
        );
    };

    resetFailedAttempts = (userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/reset_failed_attempts`,
            {method: 'post'},
        );
    };

    getKnownUsers = () => {
        return this.doFetch<Array<UserProfile['id']>>(
            `${this.getUsersRoute()}/known`,
            {method: 'get'},
        );
    };

    sendPasswordResetEmail = (email: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/password/reset/send`,
            {method: 'post', body: JSON.stringify({email})},
        );
    };

    updateUserActive = (userId: string, active: boolean) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/active`,
            {method: 'put', body: JSON.stringify({active})},
        );
    };

    uploadProfileImage = (userId: string, imageData: File) => {
        const formData = new FormData();
        formData.append('image', imageData);
        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/image`,
            request,
        );
    };

    setDefaultProfileImage = (userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/image`,
            {method: 'delete'},
        );
    };

    verifyUserEmail = (token: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/email/verify`,
            {method: 'post', body: JSON.stringify({token})},
        );
    };

    updateMyTermsOfServiceStatus = (termsOfServiceId: string, accepted: boolean) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute('me')}/terms_of_service`,
            {method: 'post', body: JSON.stringify({termsOfServiceId, accepted})},
        );
    };

    getTermsOfService = () => {
        return this.doFetch<TermsOfService>(
            `${this.getBaseRoute()}/terms_of_service`,
            {method: 'get'},
        );
    };

    createTermsOfService = (text: string) => {
        return this.doFetch<TermsOfService>(
            `${this.getBaseRoute()}/terms_of_service`,
            {method: 'post', body: JSON.stringify({text})},
        );
    };

    sendVerificationEmail = (email: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/email/verify/send`,
            {method: 'post', body: JSON.stringify({email})},
        );
    };

    login = async (loginId: string, password: string, token = '', ldapOnly = false) => {
        const body: any = {
            login_id: loginId,
            password,
            token,
            deviceId: '',
        };

        if (ldapOnly) {
            body.ldap_only = 'true';
        }

        const {
            data: profile,
            headers,
        } = await this.doFetchWithResponse<UserProfile>(
            `${this.getUsersRoute()}/login`,
            {method: 'post', body: JSON.stringify(body)},
        );

        if (headers.has('Token')) {
            this.setToken(headers.get('Token')!);
        }

        return profile;
    };

    loginWithDesktopToken = async (token: string) => {
        const body: any = {
            token,
            deviceId: '',
        };

        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}/login/desktop_token`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    loginById = (id: string, password: string, token = '') => {
        const body: any = {
            id,
            password,
            token,
            device_id: '',
        };

        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}/login`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    loginWithMagicLink = (token: string) => {
        const body = {
            token,
        };

        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}/login/one_time_link`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    getUserLoginType = (loginId: string) => {
        const body = {
            login_id: loginId,
        };

        return this.doFetch<{auth_service: 'magic_link' | ''; is_deactivated: boolean}>(
            `${this.getUsersRoute()}/login/type`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    logout = async () => {
        const {response} = await this.doFetchWithResponse(
            `${this.getUsersRoute()}/logout`,
            {method: 'post'},
        );

        if (response.ok) {
            this.token = '';
        }

        this.serverVersion = '';

        return response;
    };

    getProfiles = (page = 0, perPage = PER_PAGE_DEFAULT, options: Record<string, any> = {}) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({page, per_page: perPage, ...options})}`,
            {method: 'get'},
        );
    };

    getProfilesByIds = (userIds: string[], options = {}) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}/ids${buildQueryString(options)}`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    getProfilesByUsernames = (usernames: string[]) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}/usernames`,
            {method: 'post', body: JSON.stringify(usernames)},
        );
    };

    getProfilesInTeam = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT, sort = '', options = {}) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({...options, in_team: teamId, page, per_page: perPage, sort})}`,
            {method: 'get'},
        );
    };

    getProfilesNotInTeam = (teamId: string, groupConstrained: boolean, page = 0, perPage = PER_PAGE_DEFAULT) => {
        const queryStringObj: any = {not_in_team: teamId, page, per_page: perPage};
        if (groupConstrained) {
            queryStringObj.group_constrained = true;
        }

        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString(queryStringObj)}`,
            {method: 'get'},
        );
    };

    getProfilesWithoutTeam = (page = 0, perPage = PER_PAGE_DEFAULT, options = {}) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({...options, without_team: 1, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getProfilesInChannel = (channelId: string, page = 0, perPage = PER_PAGE_DEFAULT, sort = '', options: {active?: boolean} = {}) => {
        const queryStringObj = {in_channel: channelId, page, per_page: perPage, sort};

        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({...queryStringObj, ...options})}`,
            {method: 'get'},
        );
    };

    getProfilesInGroupChannels = (channelsIds: string[]) => {
        return this.doFetch<Record<string, UserProfile[]>>(
            `${this.getUsersRoute()}/group_channels`,
            {method: 'post', body: JSON.stringify(channelsIds)},
        );
    };

    getProfilesNotInChannel = (teamId: string, channelId: string, groupConstrained: boolean, page = 0, perPage = PER_PAGE_DEFAULT, cursorId = '') => {
        const queryStringObj: any = {in_team: teamId, not_in_channel: channelId, per_page: perPage};

        // If cursorId is provided, use cursor-based pagination
        if (cursorId) {
            queryStringObj.cursor_id = cursorId;
        } else {
            // Otherwise use traditional page-based pagination
            queryStringObj.page = page;
        }
        if (groupConstrained) {
            queryStringObj.group_constrained = true;
        }

        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString(queryStringObj)}`,
            {method: 'get'},
        );
    };

    // getProfilesMatchingChannelPolicy returns only users who satisfy the ABAC
    // policy on the channel and are not yet members. Useful for public
    // policy-enforced channels to annotate recommended members without
    // filtering the primary invite list.
    getProfilesMatchingChannelPolicy = (teamId: string, channelId: string, groupConstrained: boolean, perPage = PER_PAGE_DEFAULT, cursorId = '') => {
        const queryStringObj: any = {
            in_team: teamId,
            not_in_channel: channelId,
            per_page: perPage,
            abac_match_only: true,
        };
        if (cursorId) {
            queryStringObj.cursor_id = cursorId;
        }
        if (groupConstrained) {
            queryStringObj.group_constrained = true;
        }

        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString(queryStringObj)}`,
            {method: 'get'},
        );
    };

    getProfilesInGroup = (groupId: string, page = 0, perPage = PER_PAGE_DEFAULT, sort = '') => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({in_group: groupId, page, per_page: perPage, sort})}`,
            {method: 'get'},
        );
    };

    getProfilesNotInGroup = (groupId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString({not_in_group: groupId, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getMe = () => {
        return this.doFetch<UserProfile>(
            `${this.getUserRoute('me')}`,
            {method: 'get'},
        );
    };

    getUser = (userId: string) => {
        return this.doFetch<UserProfile>(
            `${this.getUserRoute(userId)}`,
            {method: 'get'},
        );
    };

    getUserByUsername = (username: string) => {
        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}/username/${username}`,
            {method: 'get'},
        );
    };

    getUserByEmail = (email: string) => {
        return this.doFetch<UserProfile>(
            `${this.getUsersRoute()}/email/${email}`,
            {method: 'get'},
        );
    };

    canUserDirectMessage = (userId: string, otherUserId: string) => {
        return this.doFetch<{can_dm: boolean}>(
            `${this.getSharedChannelsRoute()}/users/${userId}/can_dm/${otherUserId}`,
            {method: 'get'},
        );
    };

    getProfilePictureUrl = (userId: string, lastPictureUpdate: number) => {
        const params: any = {};

        if (lastPictureUpdate) {
            params._ = lastPictureUpdate;
        }

        return `${this.getUserRoute(userId)}/image${buildQueryString(params)}`;
    };

    getDefaultProfilePictureUrl = (userId: string) => {
        return `${this.getUserRoute(userId)}/image/default`;
    };

    autocompleteUsers = (name: string, teamId: string, channelId: string, options = {
        limit: AUTOCOMPLETE_LIMIT_DEFAULT,
    }) => {
        return this.doFetch<UserAutocomplete>(`${this.getUsersRoute()}/autocomplete${buildQueryString({
            in_team: teamId,
            in_channel: channelId,
            name,
            limit: options.limit,
        })}`, {
            method: 'get',
        });
    };

    getSessions = (userId: string) => {
        return this.doFetch<Session[]>(
            `${this.getUserRoute(userId)}/sessions`,
            {method: 'get'},
        );
    };

    revokeSession = (userId: string, sessionId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/sessions/revoke`,
            {method: 'post', body: JSON.stringify({session_id: sessionId})},
        );
    };

    revokeAllSessionsForUser = (userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/sessions/revoke/all`,
            {method: 'post'},
        );
    };

    revokeSessionsForAllUsers = () => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/sessions/revoke/all`,
            {method: 'post'},
        );
    };

    getUserAudits = (userId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Audit[]>(
            `${this.getUserRoute(userId)}/audits${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getUsersForReporting = (filter: UserReportOptions) => {
        const queryString = buildQueryString(filter);
        return this.doFetch<UserReport[]>(
            `${this.getReportsRoute()}/users${queryString}`,
            {method: 'get'},
        );
    };

    getUserCountForReporting = (filter: UserReportFilter) => {
        const queryString = buildQueryString(filter);
        return this.doFetch<number>(
            `${this.getReportsRoute()}/users/count${queryString}`,
            {method: 'get'},
        );
    };

    startUsersBatchExport = (filter: UserReportFilter) => {
        const queryString = buildQueryString(filter);
        return this.doFetch<StatusOK>(
            `${this.getReportsRoute()}/users/export${queryString}`,
            {method: 'post'},
        );
    };

    /**
     * @deprecated
     */
    checkUserMfa = (loginId: string) => {
        return this.doFetch<{mfa_required: boolean}>(
            `${this.getUsersRoute()}/mfa`,
            {method: 'post', body: JSON.stringify({login_id: loginId})},
        );
    };

    generateMfaSecret = (userId: string) => {
        return this.doFetch<MfaSecret>(
            `${this.getUserRoute(userId)}/mfa/generate`,
            {method: 'post'},
        );
    };

    searchUsers = (term: string, options: any) => {
        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}/search`,
            {method: 'post', body: JSON.stringify({term, ...options})},
        );
    };

    getStatusesByIds = (userIds: string[]) => {
        return this.doFetch<UserStatus[]>(
            `${this.getUsersRoute()}/status/ids`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    getStatus = (userId: string) => {
        return this.doFetch<UserStatus>(
            `${this.getUserRoute(userId)}/status`,
            {method: 'get'},
        );
    };

    updateStatus = (status: UserStatus) => {
        return this.doFetch<UserStatus>(
            `${this.getUserRoute(status.user_id)}/status`,
            {method: 'put', body: JSON.stringify(status)},
        );
    };

    updateCustomStatus = (customStatus: UserCustomStatus) => {
        return this.doFetch(
            `${this.getUserRoute('me')}/status/custom`,
            {method: 'put', body: JSON.stringify(customStatus)},
        );
    };

    unsetCustomStatus = () => {
        return this.doFetch(
            `${this.getUserRoute('me')}/status/custom`,
            {method: 'delete'},
        );
    };

    removeRecentCustomStatus = (customStatus: UserCustomStatus) => {
        return this.doFetch(
            `${this.getUserRoute('me')}/status/custom/recent/delete`,
            {method: 'post', body: JSON.stringify(customStatus)},
        );
    };

    moveThread = (postId: string, channelId: string) => {
        const url = this.getPostRoute(postId) + '/move';
        return this.doFetch<StatusOK>(
            url,
            {method: 'post', body: JSON.stringify({channel_id: channelId})},
        );
    };

    switchEmailToOAuth = (service: string, email: string, password: string, mfaCode = '') => {
        return this.doFetch<AuthChangeResponse>(
            `${this.getUsersRoute()}/login/switch`,
            {method: 'post', body: JSON.stringify({current_service: 'email', new_service: service, email, password, mfa_code: mfaCode})},
        );
    };

    switchOAuthToEmail = (currentService: string, email: string, password: string) => {
        return this.doFetch<AuthChangeResponse>(
            `${this.getUsersRoute()}/login/switch`,
            {method: 'post', body: JSON.stringify({current_service: currentService, new_service: 'email', email, new_password: password})},
        );
    };

    switchEmailToLdap = (email: string, emailPassword: string, ldapId: string, ldapPassword: string, mfaCode = '') => {
        return this.doFetch<AuthChangeResponse>(
            `${this.getUsersRoute()}/login/switch`,
            {method: 'post', body: JSON.stringify({current_service: 'email', new_service: 'ldap', email, password: emailPassword, ldap_id: ldapId, new_password: ldapPassword, mfa_code: mfaCode})},
        );
    };

    switchLdapToEmail = (ldapPassword: string, email: string, emailPassword: string, mfaCode = '') => {
        return this.doFetch<AuthChangeResponse>(
            `${this.getUsersRoute()}/login/switch`,
            {method: 'post', body: JSON.stringify({current_service: 'ldap', new_service: 'email', email, password: ldapPassword, new_password: emailPassword, mfa_code: mfaCode})},
        );
    };

    getAuthorizedOAuthApps = (userId: string) => {
        return this.doFetch<OAuthApp[]>(
            `${this.getUserRoute(userId)}/oauth/apps/authorized`,
            {method: 'get'},
        );
    };

    authorizeOAuthApp = (
        responseType: string | null,
        clientId: string | null,
        redirectUri: string | null,
        state: string | null,
        scope: string | null,
        resource?: string | null,
        codeChallenge?: string | null,
        codeChallengeMethod?: string | null,
    ) => {
        const body: any = {client_id: clientId, response_type: responseType, redirect_uri: redirectUri, state, scope};

        // Include resource parameter if provided
        if (resource) {
            body.resource = resource;
        }

        // Include PKCE parameters if provided
        if (codeChallenge) {
            body.code_challenge = codeChallenge;
        }
        if (codeChallengeMethod) {
            body.code_challenge_method = codeChallengeMethod;
        }

        return this.doFetch<{redirect: string}>(
            `${this.url}/oauth/authorize`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    deauthorizeOAuthApp = (clientId: string) => {
        return this.doFetch<StatusOK>(
            `${this.url}/oauth/deauthorize`,
            {method: 'post', body: JSON.stringify({client_id: clientId})},
        );
    };

    createUserAccessToken = (userId: string, description: string, expiresAt?: number) => {
        const body: {description: string; expires_at?: number} = {description};
        if (expiresAt !== undefined) {
            body.expires_at = expiresAt;
        }
        return this.doFetch<UserAccessToken>(
            `${this.getUserRoute(userId)}/tokens`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    getUserAccessToken = (tokenId: string) => {
        return this.doFetch<UserAccessToken>(
            `${this.getUsersRoute()}/tokens/${tokenId}`,
            {method: 'get'},
        );
    };

    getUserAccessTokensForUser = (userId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<UserAccessToken[]>(
            `${this.getUserRoute(userId)}/tokens${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getUserAccessTokens = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<UserAccessToken[]>(
            `${this.getUsersRoute()}/tokens${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    revokeUserAccessToken = (tokenId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/tokens/revoke`,
            {method: 'post', body: JSON.stringify({token_id: tokenId})},
        );
    };

    getNonCompliantUserAccessTokenCount = () => {
        return this.doFetch<{count: number}>(
            `${this.getUsersRoute()}/tokens/non_compliant/count`,
            {method: 'get'},
        );
    };

    revokeNonCompliantUserAccessTokens = () => {
        return this.doFetch<{count: number}>(
            `${this.getUsersRoute()}/tokens/non_compliant/revoke`,
            {method: 'post'},
        );
    };

    disableUserAccessToken = (tokenId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/tokens/disable`,
            {method: 'post', body: JSON.stringify({token_id: tokenId})},
        );
    };

    enableUserAccessToken = (tokenId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUsersRoute()}/tokens/enable`,
            {method: 'post', body: JSON.stringify({token_id: tokenId})},
        );
    };

    rotateUserAccessToken = (tokenId: string, expiresAt?: number) => {
        const body: {token_id: string; expires_at?: number} = {token_id: tokenId};
        if (expiresAt !== undefined) {
            body.expires_at = expiresAt;
        }
        return this.doFetch<UserAccessToken>(
            `${this.getUsersRoute()}/tokens/rotate`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    // Limits Routes

    getServerLimits = () => {
        return this.doFetchWithResponse<ServerLimits>(
            `${this.getServerLimitsRoute()}`,
            {
                method: 'get',
            },
        );
    };

    // Team Routes

    createTeam = (team: Team) => {
        return this.doFetch<Team>(
            `${this.getTeamsRoute()}`,
            {method: 'post', body: JSON.stringify(team)},
        );
    };

    deleteTeam = (teamId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}`,
            {method: 'delete'},
        );
    };

    unarchiveTeam = (teamId: string) => {
        return this.doFetch<Team>(
            `${this.getTeamRoute(teamId)}/restore`,
            {method: 'post'},
        );
    };

    updateTeam = (team: Team) => {
        return this.doFetch<Team>(
            `${this.getTeamRoute(team.id)}`,
            {method: 'put', body: JSON.stringify(team)},
        );
    };

    patchTeam = (team: PartialExcept<Team, 'id'>) => {
        return this.doFetch<Team>(
            `${this.getTeamRoute(team.id)}/patch`,
            {method: 'put', body: JSON.stringify(team)},
        );
    };

    updateTeamPrivacy = (teamId: string, privacy: string) => {
        return this.doFetch<Team>(
            `${this.getTeamRoute(teamId)}/privacy`,
            {method: 'put', body: JSON.stringify({privacy})},
        );
    };

    regenerateTeamInviteId = (teamId: string) => {
        return this.doFetch<Team>(
            `${this.getTeamRoute(teamId)}/regenerate_invite_id`,
            {method: 'post'},
        );
    };

    updateTeamScheme = (teamId: string, schemeId: string) => {
        const patch = {scheme_id: schemeId};

        return this.doFetch<StatusOK>(
            `${this.getTeamSchemeRoute(teamId)}`,
            {method: 'put', body: JSON.stringify(patch)},
        );
    };

    checkIfTeamExists = (teamName: string) => {
        return this.doFetch<{exists: boolean}>(
            `${this.getTeamNameRoute(teamName)}/exists`,
            {method: 'get'},
        );
    };

    getTeams = (page = 0, perPage = PER_PAGE_DEFAULT, includeTotalCount = false, excludePolicyConstrained = false, forDirectory = false) => {
        return this.doFetch<Team[] | TeamsWithCount>(
            `${this.getTeamsRoute()}${buildQueryString({page, per_page: perPage, include_total_count: includeTotalCount, exclude_policy_constrained: excludePolicyConstrained, for_directory: forDirectory})}`,
            {method: 'get'},
        );
    };

    searchTeams(term: string, opts: PagedTeamSearchOpts): Promise<Team[]>;
    searchTeams(term: string, opts: NotPagedTeamSearchOpts): Promise<TeamsWithCount>;
    searchTeams(term: string, opts: TeamSearchOpts): Promise<Team[] | TeamsWithCount> {
        return this.doFetch<Team[] | TeamsWithCount>(
            `${this.getTeamsRoute()}/search`,
            {method: 'post', body: JSON.stringify({term, ...opts})},
        );
    }

    getTeam = (teamId: string, asContentReviewer = false, flaggedPostId?: string) => {
        const queryParamsArgs: Record<string, any> = {};
        if (asContentReviewer) {
            queryParamsArgs.as_content_reviewer = true;
        }

        if (flaggedPostId) {
            queryParamsArgs.flagged_post_id = flaggedPostId;
        }

        const queryParams = buildQueryString(queryParamsArgs);
        return this.doFetch<Team>(
            `${this.getTeamRoute(teamId)}${queryParams}`,
            {method: 'get'},
        );
    };

    getTeamByName = (teamName: string) => {
        return this.doFetch<Team>(
            this.getTeamNameRoute(teamName),
            {method: 'get'},
        );
    };

    getMyTeams = () => {
        return this.doFetch<Team[]>(
            `${this.getUserRoute('me')}/teams`,
            {method: 'get'},
        );
    };

    getTeamsForUser = (userId: string) => {
        return this.doFetch<Team[]>(
            `${this.getUserRoute(userId)}/teams`,
            {method: 'get'},
        );
    };

    getMyTeamMembers = () => {
        return this.doFetch<TeamMembership[]>(
            `${this.getUserRoute('me')}/teams/members`,
            {method: 'get'},
        );
    };

    getMyTeamUnreads = (includeCollapsedThreads = false) => {
        return this.doFetch<TeamUnread[]>(
            `${this.getUserRoute('me')}/teams/unread${buildQueryString({include_collapsed_threads: includeCollapsedThreads})}`,
            {method: 'get'},
        );
    };

    getTeamMembers = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT, options?: GetTeamMembersOpts) => {
        return this.doFetch<TeamMembership[]>(
            `${this.getTeamMembersRoute(teamId)}${buildQueryString({page, per_page: perPage, ...options})}`,
            {method: 'get'},
        );
    };

    getTeamMembersForUser = (userId: string) => {
        return this.doFetch<TeamMembership[]>(
            `${this.getUserRoute(userId)}/teams/members`,
            {method: 'get'},
        );
    };

    getTeamMember = (teamId: string, userId: string) => {
        return this.doFetch<TeamMembership>(
            `${this.getTeamMemberRoute(teamId, userId)}`,
            {method: 'get'},
        );
    };

    getTeamMembersByIds = (teamId: string, userIds: string[]) => {
        return this.doFetch<TeamMembership[]>(
            `${this.getTeamMembersRoute(teamId)}/ids`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    addToTeam = (teamId: string, userId: string) => {
        const member = {user_id: userId, team_id: teamId};
        return this.doFetch<TeamMembership>(
            `${this.getTeamMembersRoute(teamId)}`,
            {method: 'post', body: JSON.stringify(member)},
        );
    };

    addToTeamFromInvite = (token = '', inviteId = '') => {
        const query = buildQueryString({token, invite_id: inviteId});
        return this.doFetch<TeamMembership>(
            `${this.getTeamsRoute()}/members/invite${query}`,
            {method: 'post'},
        );
    };

    addUsersToTeam = (teamId: string, userIds: string[]) => {
        const members: any = [];
        userIds.forEach((id) => members.push({team_id: teamId, user_id: id}));
        return this.doFetch<TeamMembership[]>(
            `${this.getTeamMembersRoute(teamId)}/batch`,
            {method: 'post', body: JSON.stringify(members)},
        );
    };

    addUsersToTeamGracefully = (teamId: string, userIds: string[]) => {
        const members: any = [];
        userIds.forEach((id) => members.push({team_id: teamId, user_id: id}));
        return this.doFetch<TeamMemberWithError[]>(
            `${this.getTeamMembersRoute(teamId)}/batch?graceful=true`,
            {method: 'post', body: JSON.stringify(members)},
        );
    };

    joinTeam = (inviteId: string) => {
        const query = buildQueryString({invite_id: inviteId});
        return this.doFetch<TeamMembership>(
            `${this.getTeamsRoute()}/members/invite${query}`,
            {method: 'post'},
        );
    };

    removeFromTeam = (teamId: string, userId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamMemberRoute(teamId, userId)}`,
            {method: 'delete'},
        );
    };

    getTeamStats = (teamId: string) => {
        return this.doFetch<TeamStats>(
            `${this.getTeamRoute(teamId)}/stats`,
            {method: 'get'},
        );
    };

    getTotalUsersStats = () => {
        return this.doFetch<UsersStats>(
            `${this.getUsersRoute()}/stats`,
            {method: 'get'},
        );
    };

    getFilteredUsersStats = (options: GetFilteredUsersStatsOpts) => {
        return this.doFetch<UsersStats>(
            `${this.getUsersRoute()}/stats/filtered${buildQueryString(options)}`,
            {method: 'get'},
        );
    };

    invalidateAllEmailInvites = () => {
        return this.doFetch<StatusOK>(
            `${this.getTeamsRoute()}/invites/email`,
            {method: 'delete'},
        );
    };

    getTeamInviteInfo = (inviteId: string) => {
        return this.doFetch<{
            display_name: string;
            description: string;
            name: string;
            id: string;
        }>(
            `${this.getTeamsRoute()}/invite/${inviteId}`,
            {method: 'get'},
        );
    };

    updateTeamMemberRoles = (teamId: string, userId: string, roles: string[]) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamMemberRoute(teamId, userId)}/roles`,
            {method: 'put', body: JSON.stringify({roles})},
        );
    };

    sendEmailInvitesToTeam = (teamId: string, emails: string[]) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}/invite/email`,
            {method: 'post', body: JSON.stringify(emails)},
        );
    };

    sendEmailGuestInvitesToChannels = (teamId: string, channelIds: string[], emails: string[], message: string) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}/invite-guests/email`,
            {method: 'post', body: JSON.stringify({emails, channels: channelIds, message})},
        );
    };

    sendEmailInvitesToTeamGracefully = (teamId: string, emails: string[], profiles?: MemberInviteProfile[]) => {
        // Keep the historical raw-array body unless profiles are provided.
        const body = profiles?.length ? JSON.stringify({emails, profiles}) : JSON.stringify(emails);
        return this.doFetch<TeamInviteWithError[]>(
            `${this.getTeamRoute(teamId)}/invite/email?graceful=true`,
            {method: 'post', body},
        );
    };

    sendEmailInvitesToTeamAndChannelsGracefully = (
        teamId: string,
        channelIds: string[],
        emails: string[],
        message: string,
        profiles?: MemberInviteProfile[],
    ) => {
        return this.doFetch<TeamInviteWithError[]>(
            `${this.getTeamRoute(teamId)}/invite/email?graceful=true`,
            {method: 'post', body: JSON.stringify({emails, channelIds, message, profiles: profiles?.length ? profiles : undefined})},
        );
    };

    sendEmailGuestInvitesToChannelsGracefully = async (teamId: string, channelIds: string[], emails: string[], message: string, guestMagicLink = false) => {
        const queryParams = new URLSearchParams({graceful: 'true'});
        if (guestMagicLink) {
            queryParams.append('guest_magic_link', 'true');
        }
        return this.doFetch<TeamInviteWithError[]>(
            `${this.getTeamRoute(teamId)}/invite-guests/email?${queryParams.toString()}`,
            {method: 'post', body: JSON.stringify({emails, channels: channelIds, message})},
        );
    };

    getTeamIconUrl = (teamId: string, lastTeamIconUpdate: number) => {
        const params: any = {};
        if (lastTeamIconUpdate) {
            params._ = lastTeamIconUpdate;
        }

        return `${this.getTeamRoute(teamId)}/image${buildQueryString(params)}`;
    };

    setTeamIcon = (teamId: string, imageData: File) => {
        const formData = new FormData();
        formData.append('image', imageData);

        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}/image`,
            request,
        );
    };

    removeTeamIcon = (teamId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}/image`,
            {method: 'delete'},
        );
    };

    updateTeamMemberSchemeRoles = (teamId: string, userId: string, isSchemeUser: boolean, isSchemeAdmin: boolean) => {
        const body = {scheme_user: isSchemeUser, scheme_admin: isSchemeAdmin};
        return this.doFetch<StatusOK>(
            `${this.getTeamRoute(teamId)}/members/${userId}/schemeRoles`,
            {method: 'put', body: JSON.stringify(body)},
        );
    };

    // Channel Routes

    getAllChannels(
        page: number | undefined,
        perPage: number | undefined,
        notAssociatedToGroup: string | undefined,
        excludeDefaultChannels: boolean | undefined,
        includeTotalCount: false | undefined,
        includeDeleted: boolean | undefined,
        excludePolicyConstrained: boolean | undefined,
        accessControlPolicyEnforced: boolean | undefined,
        excludeAccessControlPolicyEnforced: boolean | undefined
    ): Promise<ChannelWithTeamData[]>;
    getAllChannels(
        page: number | undefined,
        perPage: number | undefined,
        notAssociatedToGroup: string | undefined,
        excludeDefaultChannels: boolean | undefined,
        includeTotalCount: true,
        includeDeleted: boolean | undefined,
        excludePolicyConstrained: boolean | undefined,
        accessControlPolicyEnforced: boolean | undefined,
        excludeAccessControlPolicyEnforced: boolean | undefined
    ): Promise<ChannelsWithTotalCount>;
    getAllChannels(
        page = 0,
        perPage = PER_PAGE_DEFAULT,
        notAssociatedToGroup = '',
        excludeDefaultChannels = false,
        includeTotalCount = false,
        includeDeleted = false,
        excludePolicyConstrained = false,
        accessControlPolicyEnforced = false,
        excludeAccessControlPolicyEnforced = false,
    ) {
        const queryData: Record<string, any> = {
            page,
            per_page: perPage,
            include_total_count: includeTotalCount,
            include_deleted: includeDeleted,
        };

        if (notAssociatedToGroup) {
            queryData.not_associated_to_group = notAssociatedToGroup;
        }

        if (excludeDefaultChannels) {
            queryData.exclude_default_channels = excludeDefaultChannels;
        }

        if (excludePolicyConstrained) {
            queryData.exclude_policy_constrained = excludePolicyConstrained;
        }

        if (accessControlPolicyEnforced) {
            queryData.access_control_policy_enforced = accessControlPolicyEnforced;
        }

        if (excludeAccessControlPolicyEnforced) {
            queryData.exclude_access_control_policy_enforced = excludeAccessControlPolicyEnforced;
        }

        return this.doFetch<ChannelWithTeamData[] | ChannelsWithTotalCount>(
            `${this.getChannelsRoute()}${buildQueryString(queryData)}`,
            {method: 'get'},
        );
    }

    createChannel = (channel: PartialExcept<Channel, 'name' | 'display_name' | 'type' | 'team_id'>) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelsRoute()}`,
            {method: 'post', body: JSON.stringify(channel)},
        );
    };

    createDirectChannel = (userIds: string[]) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelsRoute()}/direct`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    createGroupChannel = (userIds: string[]) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelsRoute()}/group`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    deleteChannel = (channelId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getChannelRoute(channelId)}`,
            {method: 'delete'},
        );
    };

    unarchiveChannel = (channelId: string) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelRoute(channelId)}/restore`,
            {method: 'post'},
        );
    };

    updateChannel = (channel: Channel) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelRoute(channel.id)}`,
            {method: 'put', body: JSON.stringify(channel)},
        );
    };

    updateChannelPrivacy = (channelId: string, privacy: any) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelRoute(channelId)}/privacy`,
            {method: 'put', body: JSON.stringify({privacy})},
        );
    };

    patchChannel = (channelId: string, channelPatch: Partial<Channel>) => {
        return this.doFetch<ServerChannel>(
            `${this.getChannelRoute(channelId)}/patch`,
            {method: 'put', body: JSON.stringify(channelPatch)},
        );
    };

    setMyChannelAutotranslation = (channelId: string, enabled: boolean) => {
        return this.doFetch<StatusOK>(
            `${this.getChannelMemberRoute(channelId, 'me')}/autotranslation`,
            {method: 'put', body: JSON.stringify({autotranslation_disabled: !enabled})},
        );
    };

    updateChannelNotifyProps = (props: any) => {
        return this.doFetch<StatusOK>(
            `${this.getChannelMemberRoute(props.channel_id, props.user_id)}/notify_props`,
            {method: 'put', body: JSON.stringify(props)},
        );
    };

    updateChannelScheme = (channelId: string, schemeId: string) => {
        const patch = {scheme_id: schemeId};

        return this.doFetch<StatusOK>(
            `${this.getChannelSchemeRoute(channelId)}`,
            {method: 'put', body: JSON.stringify(patch)},
        );
    };

    getChannel = (channelId: string, asContentReviewer = false, flaggedPostId?: string) => {
        const queryParamsArgs: Record<string, any> = {};
        if (asContentReviewer) {
            queryParamsArgs.as_content_reviewer = true;
        }

        if (flaggedPostId) {
            queryParamsArgs.flagged_post_id = flaggedPostId;
        }

        const queryParams = buildQueryString(queryParamsArgs);
        return this.doFetch<ServerChannel>(
            `${this.getChannelRoute(channelId)}${queryParams}`,
            {method: 'get'},
        );
    };

    getChannelByName = (teamId: string, channelName: string, includeDeleted = false) => {
        return this.doFetch<ServerChannel>(
            `${this.getTeamRoute(teamId)}/channels/name/${channelName}?include_deleted=${includeDeleted}`,
            {method: 'get'},
        );
    };

    getChannelByNameAndTeamName = (teamName: string, channelName: string, includeDeleted = false) => {
        return this.doFetch<ServerChannel>(
            `${this.getTeamNameRoute(teamName)}/channels/name/${channelName}?include_deleted=${includeDeleted}`,
            {method: 'get'},
        );
    };

    getChannels = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<ServerChannel[]>(
            `${this.getTeamRoute(teamId)}/channels${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getAllTeamsChannels = () => {
        return this.doFetch<ServerChannel[]>(
            `${this.getUsersRoute()}/me/channels`,
            {method: 'get'},
        );
    };

    getArchivedChannels = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<ServerChannel[]>(
            `${this.getTeamRoute(teamId)}/channels/deleted${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getRecommendedChannelsForUser = (teamId: string) => {
        return this.doFetch<ServerChannel[]>(
            `${this.getTeamRoute(teamId)}/channels/recommended`,
            {method: 'get'},
        );
    };

    getMyChannels = (teamId: string, includeDeleted = false) => {
        return this.doFetch<ServerChannel[]>(
            `${this.getUserRoute('me')}/teams/${teamId}/channels${buildQueryString({include_deleted: includeDeleted})}`,
            {method: 'get'},
        );
    };

    getAllChannelsMembers = (userId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<ChannelMembership[]>(
            `${this.getUserRoute(userId)}/channel_members${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getMyChannelMember = (channelId: string) => {
        return this.doFetch<ChannelMembership>(
            `${this.getChannelMemberRoute(channelId, 'me')}`,
            {method: 'get'},
        );
    };

    getMyChannelMembers = (teamId: string) => {
        return this.doFetch<ChannelMembership[]>(
            `${this.getUserRoute('me')}/teams/${teamId}/channels/members`,
            {method: 'get'},
        );
    };

    getChannelMembers = (channelId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<ChannelMembership[]>(
            `${this.getChannelMembersRoute(channelId)}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getChannelTimezones = (channelId: string) => {
        return this.doFetch<string[]>(
            `${this.getChannelRoute(channelId)}/timezones`,
            {method: 'get'},
        );
    };

    getChannelMember = (channelId: string, userId: string) => {
        return this.doFetch<ChannelMembership>(
            `${this.getChannelMemberRoute(channelId, userId)}`,
            {method: 'get'},
        );
    };

    getChannelMembersByIds = (channelId: string, userIds: string[]) => {
        return this.doFetch<ChannelMembership[]>(
            `${this.getChannelMembersRoute(channelId)}/ids`,
            {method: 'post', body: JSON.stringify(userIds)},
        );
    };

    // ------------------------------------------------------------------
    // Discoverable Private Channels — join request endpoints (FF gated)
    // ------------------------------------------------------------------

    // POST /channels/{id}/join_request
    // Server returns either {status: 'approved'} (immediate add via the ABAC
    // fast path) or the full ChannelJoinRequest row (pending admin review).
    // The two shapes are discriminated by the `status` field; callers branch
    // on whether `id` is present.
    requestJoinChannel = (channelId: string, message = '') => {
        return this.doFetch<ChannelJoinRequest | ChannelJoinRequestApprovalResponse>(
            `${this.getChannelJoinRequestRoute(channelId)}`,
            {method: 'post', body: JSON.stringify({message})},
        );
    };

    // GET /channels/{id}/join_request — current user's pending request for
    // this channel. The server returns 404 with no body when none exists;
    // callers should treat 404 as "no pending request" rather than an error.
    getMyChannelJoinRequest = (channelId: string) => {
        return this.doFetch<ChannelJoinRequest>(
            `${this.getChannelJoinRequestRoute(channelId)}`,
            {method: 'get'},
        );
    };

    // DELETE /channels/{id}/join_request — withdraw the current user's
    // pending request. Returns the updated row.
    withdrawMyChannelJoinRequest = (channelId: string) => {
        return this.doFetch<ChannelJoinRequest>(
            `${this.getChannelJoinRequestRoute(channelId)}`,
            {method: 'delete'},
        );
    };

    // GET /channels/{id}/join_requests — admin queue listing.
    getChannelJoinRequests = (channelId: string, opts: GetChannelJoinRequestsOptions = {}) => {
        const query = buildQueryString({
            status: opts.status,
            page: opts.page,
            per_page: opts.per_page,
        });
        return this.doFetch<ChannelJoinRequestList>(
            `${this.getChannelJoinRequestsRoute(channelId)}${query}`,
            {method: 'get'},
        );
    };

    // GET /channels/{id}/join_requests/count — pending count for the channel
    // header / LHS / RHS indicator triad.
    countPendingChannelJoinRequests = (channelId: string) => {
        return this.doFetch<{count: number}>(
            `${this.getChannelJoinRequestsRoute(channelId)}/count`,
            {method: 'get'},
        );
    };

    // PATCH /channels/{id}/join_requests/{request_id} — approve or deny.
    patchChannelJoinRequest = (channelId: string, requestId: string, patch: ChannelJoinRequestPatch) => {
        return this.doFetch<ChannelJoinRequest>(
            `${this.getChannelJoinRequestsRoute(channelId)}/${requestId}`,
            {method: 'PATCH', body: JSON.stringify(patch)},
        );
    };

    // GET /users/me/channel_join_requests — the current user's requests
    // across channels, used by the My Pending Requests tab.
    getMyChannelJoinRequests = (opts: GetChannelJoinRequestsOptions = {}) => {
        const query = buildQueryString({
            status: opts.status,
            page: opts.page,
            per_page: opts.per_page,
        });
        return this.doFetch<ChannelJoinRequestList>(
            `${this.getUserRoute('me')}/channel_join_requests${query}`,
            {method: 'get'},
        );
    };

    addToChannels = (userIds: string[], channelId: string, postRootId = '') => {
        const members = {user_ids: userIds, channel_id: channelId, post_root_id: postRootId};
        return this.doFetch<ChannelMembership[]>(
            `${this.getChannelMembersRoute(channelId)}`,
            {method: 'post', body: JSON.stringify(members)},
        );
    };

    addToChannel = (userId: string, channelId: string, postRootId = '') => {
        const member = {user_id: userId, channel_id: channelId, post_root_id: postRootId};
        return this.doFetch<ChannelMembership>(
            `${this.getChannelMembersRoute(channelId)}`,
            {method: 'post', body: JSON.stringify(member)},
        );
    };

    removeFromChannel = (userId: string, channelId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getChannelMemberRoute(channelId, userId)}`,
            {method: 'delete'},
        );
    };

    updateChannelMemberRoles = (channelId: string, userId: string, roles: string) => {
        return this.doFetch<StatusOK>(
            `${this.getChannelMemberRoute(channelId, userId)}/roles`,
            {method: 'put', body: JSON.stringify({roles})},
        );
    };

    getChannelStats = (channelId: string, includeFileCount = false) => {
        const param = includeFileCount ? '' : '?exclude_files_count=true';
        return this.doFetch<ChannelStats>(
            `${this.getChannelRoute(channelId)}/stats${param}`,
            {method: 'get'},
        );
    };

    getChannelsMemberCount = (channelIds: string[]) => {
        return this.doFetch<Record<string, number>>(
            `${this.getChannelsRoute()}/stats/member_count`,
            {method: 'post', body: JSON.stringify(channelIds)},
        );
    };

    getChannelModerations = (channelId: string) => {
        return this.doFetch<ChannelModeration[]>(
            `${this.getChannelRoute(channelId)}/moderations`,
            {method: 'get'},
        );
    };

    patchChannelModerations = (channelId: string, channelModerationsPatch: ChannelModerationPatch[]) => {
        return this.doFetch<ChannelModeration[]>(
            `${this.getChannelRoute(channelId)}/moderations/patch`,
            {method: 'put', body: JSON.stringify(channelModerationsPatch)},
        );
    };

    getChannelMemberCountsByGroup = (channelId: string, includeTimezones: boolean) => {
        return this.doFetch<ChannelMemberCountsByGroup>(
            `${this.getChannelRoute(channelId)}/member_counts_by_group?include_timezones=${includeTimezones}`,
            {method: 'get'},
        );
    };

    viewMyChannel = (channelId: string) => {
        const data = {channel_id: channelId, collapsed_threads_supported: true};
        return this.doFetch<ChannelViewResponse>(
            `${this.getChannelsRoute()}/members/me/view`,
            {method: 'post', body: JSON.stringify(data)},
        );
    };

    readMultipleChannels = (channelIds: string[]) => {
        return this.doFetch<ChannelViewResponse>(
            `${this.getChannelsRoute()}/members/me/mark_read`,
            {method: 'post', body: JSON.stringify(channelIds)},
        );
    };

    markAllInTeamAsRead = (userId: string, teamId: string) => {
        return this.doFetch<ChannelViewResponse>(
            `${this.getUserRoute(userId)}/teams/${teamId}/read`,
            {method: 'put'},
        );
    };

    markAllMessagesAsRead = (userId: string) => {
        return this.doFetch<ChannelViewResponse>(
            `${this.getChannelsRoute()}/members/${userId}/direct/read`,
            {method: 'put'},
        );
    };

    autocompleteChannels = (teamId: string, name: string) => {
        return this.doFetch<Channel[]>(
            `${this.getTeamRoute(teamId)}/channels/autocomplete${buildQueryString({name})}`,
            {method: 'get'},
        );
    };

    autocompleteChannelsForSearch = (teamId: string, name: string) => {
        return this.doFetch<Channel[]>(
            `${this.getTeamRoute(teamId)}/channels/search_autocomplete${buildQueryString({name})}`,
            {method: 'get'},
        );
    };

    searchChannels = (teamId: string, term: string) => {
        return this.doFetch<Channel[]>(
            `${this.getTeamRoute(teamId)}/channels/search`,
            {method: 'post', body: JSON.stringify({term})},
        );
    };

    searchAllChannels(term: string, opts: {page: number; per_page: number} & ChannelSearchOpts & OptsSignalExt): Promise<ChannelsWithTotalCount>;
    searchAllChannels(term: string, opts: Omit<ChannelSearchOpts, 'page' | 'per_page'> & OptsSignalExt | undefined): Promise<ChannelWithTeamData[]>;
    searchAllChannels(term: string, opts: ChannelSearchOpts & OptsSignalExt = {}) {
        const body = {
            term,
            ...opts,
        };
        const includeDeleted = Boolean(opts.include_deleted);
        const nonAdminSearch = Boolean(opts.nonAdminSearch);
        const excludeRemote = Boolean(opts.exclude_remote);
        let queryParams: {include_deleted?: boolean; system_console?: boolean; exclude_remote?: boolean} = {include_deleted: includeDeleted, exclude_remote: excludeRemote};
        if (nonAdminSearch) {
            queryParams = {system_console: false};
            delete body.nonAdminSearch;
        }
        return this.doFetch<ChannelWithTeamData[] | ChannelsWithTotalCount>(
            `${this.getChannelsRoute()}/search${buildQueryString(queryParams)}`,
            {method: 'post', body: JSON.stringify(body), signal: opts.signal},
        );
    }

    searchGroupChannels = (term: string) => {
        return this.doFetch<Channel[]>(
            `${this.getChannelsRoute()}/group/search`,
            {method: 'post', body: JSON.stringify({term})},
        );
    };

    updateChannelMemberSchemeRoles = (channelId: string, userId: string, isSchemeUser: boolean, isSchemeAdmin: boolean) => {
        const body = {scheme_user: isSchemeUser, scheme_admin: isSchemeAdmin};
        return this.doFetch<StatusOK>(
            `${this.getChannelRoute(channelId)}/members/${userId}/schemeRoles`,
            {method: 'put', body: JSON.stringify(body)},
        );
    };

    // Channel Bookmark Routes

    getChannelBookmarks = (channelId: string, bookmarksSince?: number) => {
        return this.doFetch<ChannelBookmark[]>(
            `${this.getChannelBookmarksRoute(channelId)}${buildQueryString({bookmarks_since: bookmarksSince})}`,
            {method: 'get'},
        );
    };

    createChannelBookmark = (channelId: string, channelBookmark: ChannelBookmarkCreate, connectionId: string) => {
        return this.doFetch<ChannelBookmark>(
            `${this.getChannelBookmarksRoute(channelId)}`,
            {method: 'post', body: JSON.stringify(channelBookmark), headers: {'Connection-Id': connectionId}},
        );
    };

    deleteChannelBookmark = (channelId: string, channelBookmarkId: string, connectionId: string) => {
        return this.doFetch<ChannelBookmark>(
            `${this.getChannelBookmarkRoute(channelId, channelBookmarkId)}`,
            {method: 'delete', headers: {'Connection-Id': connectionId}},
        );
    };

    updateChannelBookmark = (channelId: string, channelBookmarkId: string, patch: ChannelBookmarkPatch, connectionId: string) => {
        return this.doFetch<UpdateChannelBookmarkResponse>(
            `${this.getChannelBookmarkRoute(channelId, channelBookmarkId)}`,
            {method: 'PATCH', body: JSON.stringify(patch), headers: {'Connection-Id': connectionId}},
        );
    };

    updateChannelBookmarkSortOrder = (channelId: string, channelBookmarkId: string, newOrder: number, connectionId: string) => {
        return this.doFetch<ChannelBookmark[]>(
            `${this.getChannelBookmarksRoute(channelId)}/${channelBookmarkId}/sort_order`,
            {method: 'post', body: JSON.stringify(newOrder), headers: {'Connection-Id': connectionId}},
        );
    };

    //  Channel Category Routes

    getChannelCategories = (userId: string, teamId: string) => {
        return this.doFetch<OrderedChannelCategories>(
            `${this.getChannelCategoriesRoute(userId, teamId)}`,
            {method: 'get'},
        );
    };

    createChannelCategory = (userId: string, teamId: string, category: Partial<ChannelCategory>) => {
        return this.doFetch<ChannelCategory>(
            `${this.getChannelCategoriesRoute(userId, teamId)}`,
            {method: 'post', body: JSON.stringify(category)},
        );
    };

    updateChannelCategories = (userId: string, teamId: string, categories: ChannelCategory[]) => {
        return this.doFetch<ChannelCategory[]>(
            `${this.getChannelCategoriesRoute(userId, teamId)}`,
            {method: 'put', body: JSON.stringify(categories)},
        );
    };

    getChannelCategoryOrder = (userId: string, teamId: string) => {
        return this.doFetch<string[]>(
            `${this.getChannelCategoriesRoute(userId, teamId)}/order`,
            {method: 'get'},
        );
    };

    updateChannelCategoryOrder = (userId: string, teamId: string, categoryOrder: string[]) => {
        return this.doFetch<string[]>(
            `${this.getChannelCategoriesRoute(userId, teamId)}/order`,
            {method: 'put', body: JSON.stringify(categoryOrder)},
        );
    };

    getChannelCategory = (userId: string, teamId: string, categoryId: string) => {
        return this.doFetch<ChannelCategory>(
            `${this.getChannelCategoriesRoute(userId, teamId)}/${categoryId}`,
            {method: 'get'},
        );
    };

    updateChannelCategory = (userId: string, teamId: string, category: ChannelCategory) => {
        return this.doFetch<ChannelCategory>(
            `${this.getChannelCategoriesRoute(userId, teamId)}/${category.id}`,
            {method: 'put', body: JSON.stringify(category)},
        );
    };

    deleteChannelCategory = (userId: string, teamId: string, categoryId: string) => {
        return this.doFetch<ChannelCategory>(
            `${this.getChannelCategoriesRoute(userId, teamId)}/${categoryId}`,
            {method: 'delete'},
        );
    };

    // Managed Channel Categories Routes

    getManagedCategories = (teamId: string) => {
        return this.doFetch<Record<string, string>>(
            `${this.getTeamRoute(teamId)}/channels/managed_categories`,
            {method: 'get'},
        );
    };

    getPropertyValues = <T>(groupName: string, objectType: string, targetId: string) => {
        return this.doFetch<Array<PropertyValue<T>>>(
            `${this.getBaseRoute()}/properties/groups/${groupName}/${objectType}/values/${targetId}`,
            {method: 'get'},
        );
    };

    getSystemPropertyValues = <T>(groupName: string) => {
        return this.doFetch<Array<PropertyValue<T>>>(
            `${this.getBaseRoute()}/properties/groups/${groupName}/system/values`,
            {method: 'get'},
        );
    };

    patchSystemPropertyValues = <T>(groupName: string, items: Array<{field_id: string; value: T}>) => {
        return this.doFetch<Array<PropertyValue<T>>>(
            `${this.getBaseRoute()}/properties/groups/${groupName}/system/values`,
            {method: 'PATCH', body: JSON.stringify(items)},
        );
    };

    patchPropertyValues = <T>(groupName: string, objectType: string, targetId: string, items: Array<{field_id: string; value: T}>) => {
        return this.doFetch<Array<PropertyValue<T>>>(
            `${this.getBaseRoute()}/properties/groups/${groupName}/${objectType}/values/${targetId}`,
            {method: 'PATCH', body: JSON.stringify(items)},
        );
    };

    // Remote Clusters Routes

    getRemoteClusters = (options: {
        excludePlugins?: boolean;
        notInChannel?: string;
        onlyConfirmed?: boolean;
    } = {}) => {
        const params: Record<string, string | boolean> = {};
        if (options.excludePlugins !== undefined) {
            params.exclude_plugins = options.excludePlugins;
        }
        if (options.notInChannel) {
            params.not_in_channel = options.notInChannel;
        }
        if (options.onlyConfirmed !== undefined) {
            params.only_confirmed = options.onlyConfirmed;
        }
        return this.doFetch<RemoteCluster[]>(
            `${this.getRemoteClustersRoute()}${buildQueryString(params)}`,
            {method: 'GET'},
        );
    };

    getRemoteCluster = (remoteId: string) => {
        return this.doFetch<RemoteCluster>(
            `${this.getRemoteClusterRoute(remoteId)}`,
            {method: 'GET'},
        );
    };

    createRemoteCluster = (remoteCluster: PartialExcept<RemoteClusterWithPassword, 'name' | 'display_name'>) => {
        return this.doFetch<{invite: string; password: string; remote_cluster: RemoteCluster}>(
            `${this.getRemoteClustersRoute()}`,
            {method: 'POST', body: JSON.stringify(remoteCluster)},
        );
    };

    patchRemoteCluster = (remoteId: string, patch: Partial<RemoteClusterPatch>) => {
        return this.doFetch<RemoteCluster>(
            `${this.getRemoteClusterRoute(remoteId)}`,
            {method: 'PATCH', body: JSON.stringify(patch)},
        );
    };

    deleteRemoteCluster = (remoteId: string) => {
        return this.doFetch(
            `${this.getRemoteClusterRoute(remoteId)}`,
            {method: 'DELETE'},
        );
    };

    acceptInviteRemoteCluster = (remoteClusterAcceptInvite: RemoteClusterAcceptInvite) => {
        return this.doFetch<RemoteCluster>(
            `${this.getRemoteClustersRoute()}/accept_invite`,
            {method: 'POST', body: JSON.stringify(remoteClusterAcceptInvite)},
        );
    };

    generateInviteRemoteCluster = (remoteId: string, remoteCluster: Partial<Pick<RemoteClusterWithPassword, 'password'>>) => {
        return this.doFetch<string>(
            `${this.getRemoteClusterRoute(remoteId)}/generate_invite`,
            {method: 'POST', body: JSON.stringify(remoteCluster)},
        );
    };

    // Shared Channels Routes

    getSharedChannelRemotes = (
        remoteId: string,
        filters: {
            exclude_remote?: boolean;
            exclude_home?: boolean;
            include_deleted?: boolean;
            include_unconfirmed?: boolean;
            exclude_confirmed?: boolean;
        },
    ) => {
        return this.doFetch<SharedChannelRemote[]>(
            `${this.getRemoteClusterRoute(remoteId)}/sharedchannelremotes${buildQueryString(filters)}`,
            {method: 'GET'},
        );
    };

    getSharedChannelRemoteInfos = (channelId: string) => {
        return this.doFetch<RemoteClusterInfo[]>(
            `${this.getBaseRoute()}/sharedchannels/${channelId}/remotes`,
            {method: 'GET'},
        );
    };

    getRemoteClusterInfo = (remoteId: string, includeDeleted?: boolean) => {
        return this.doFetch<RemoteClusterInfo>(
            `${this.getBaseRoute()}/sharedchannels/remote_info/${remoteId}${buildQueryString({include_deleted: includeDeleted})}`,
            {method: 'GET'},
        );
    };

    sharedChannelRemoteInvite = (remoteId: string, channelId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getRemoteClusterRoute(remoteId)}/channels/${channelId}/invite`,
            {method: 'POST'},
        );
    };

    sharedChannelRemoteUninvite = (remoteId: string, channelId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getRemoteClusterRoute(remoteId)}/channels/${channelId}/uninvite`,
            {method: 'POST'},
        );
    };

    // System Properties Routes

    getCustomProfileAttributeFields = async () => {
        return this.doFetch<UserPropertyField[]>(
            `${this.getCustomProfileAttributeFieldsRoute()}`,
            {method: 'GET'},
        );
    };

    createCustomProfileAttributeField = async (patch: UserPropertyFieldPatch) => {
        return this.doFetch<UserPropertyField>(
            `${this.getCustomProfileAttributeFieldsRoute()}`,
            {method: 'POST', body: JSON.stringify(patch)},
        );
    };

    patchCustomProfileAttributeField = async (fieldId: string, patch: UserPropertyFieldPatch) => {
        return this.doFetch<UserPropertyField>(
            `${this.getCustomProfileAttributeFieldRoute(fieldId)}`,
            {method: 'PATCH', body: JSON.stringify(patch)},
        );
    };

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    deleteCustomProfileAttributeField = async (fieldId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getCustomProfileAttributeFieldRoute(fieldId)}`,
            {method: 'DELETE'},
        );
    };

    updateCustomProfileAttributeValues = (attributeValues: Record<string, string | string[]>) => {
        return this.doFetch<Record<string, string | string[]>>(
            `${this.getCustomProfileAttributeValuesRoute()}`,
            {method: 'PATCH', body: JSON.stringify(attributeValues)},
        );
    };

    updateUserCustomProfileAttributesValues = (userID: string, attributeValues: Record<string, string | string[]>) => {
        return this.doFetch<Record<string, string | string[]>>(
            `${this.getUserRoute(userID)}/custom_profile_attributes`,
            {method: 'PATCH', body: JSON.stringify(attributeValues)},
        );
    };

    // Generic Property Field Routes

    getPropertyFields = async (groupName: string, objectType: string, query: PropertyFieldsQuery) => {
        const params = new URLSearchParams();

        if (query.channelId) {
            params.set('channel_id', query.channelId);
        }
        if (query.teamId) {
            params.set('team_id', query.teamId);
        }

        if (query.targetType) {
            params.set('target_type', query.targetType);
        }
        if (query.targetId) {
            params.set('target_id', query.targetId);
        }

        if (query.perPage) {
            params.set('per_page', String(query.perPage));
        }
        if (query.cursorId) {
            params.set('cursor_id', query.cursorId);
        }
        if (query.cursorCreateAt !== undefined) {
            params.set('cursor_create_at', String(query.cursorCreateAt));
        }

        return this.doFetch<PropertyField[]>(
            `${this.getPropertyFieldsRoute(groupName, objectType)}?${params.toString()}`,
            {method: 'GET'},
        );
    };

    // One page of a property field's options, in creation order. has_more is
    // the only signal that ends a listing — a short page does not, and an
    // empty page does not either. Resume from next_cursor_create_at and
    // next_cursor_id, which name the last candidate examined, not the last
    // option returned. The two request cursor halves go together — the
    // server refuses a request carrying only one of them.
    getPropertyFieldOptions = async (
        groupName: string,
        objectType: string,
        fieldId: string,
        options?: {perPage?: number; cursorId?: string; cursorCreateAt?: number},
    ) => {
        const params = new URLSearchParams();
        params.set('per_page', String(options?.perPage ?? PROPERTY_FIELD_OPTIONS_PER_PAGE_DEFAULT));
        if (options?.cursorId) {
            params.set('cursor_id', options.cursorId);
        }
        if (options?.cursorCreateAt) {
            params.set('cursor_create_at', String(options.cursorCreateAt));
        }
        return this.doFetch<PropertyFieldOptionPage>(
            `${this.getPropertyFieldOptionsRoute(groupName, objectType, fieldId)}?${params.toString()}`,
            {method: 'GET'},
        );
    };

    searchGlobalAttributes = async (search: GlobalAttributeListingSearch) => {
        return this.doFetch<GlobalAttributeListing>(
            `${this.getBaseRoute()}/properties/groups/access_control/attributes/search`,
            {method: 'POST', body: JSON.stringify(search)},
        );
    };

    createPropertyField = async (groupName: string, objectType: string, field: Partial<PropertyField> & Record<string, unknown>) => {
        return this.doFetch<PropertyField>(
            `${this.getPropertyFieldsRoute(groupName, objectType)}`,
            {method: 'POST', body: JSON.stringify(field)},
        );
    };

    patchPropertyField = async (groupName: string, objectType: string, fieldId: string, patch: Partial<PropertyField> & Record<string, unknown>) => {
        return this.doFetch<PropertyField>(
            `${this.getPropertyFieldRoute(groupName, objectType, fieldId)}`,
            {method: 'PATCH', body: JSON.stringify(patch)},
        );
    };

    deletePropertyField = async (groupName: string, objectType: string, fieldId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPropertyFieldRoute(groupName, objectType, fieldId)}`,
            {method: 'DELETE'},
        );
    };

    // getChannelsMissingAttributeValue returns a paginated list of active
    // channels lacking a value for the given channel attribute field.
    getChannelsMissingAttributeValue = async (groupName: string, fieldId: string, page: number, perPage: number) => {
        const params = new URLSearchParams({page: String(page), per_page: String(perPage)});
        return this.doFetch<ChannelsMissingAttributeValueList>(
            `${this.getPropertyFieldRoute(groupName, 'channel', fieldId)}/missing_values?${params.toString()}`,
            {method: 'GET'},
        );
    };

    // getChannelAttributeComplianceSummary returns the compliance counts used
    // by the Required toggle's banner. An empty fieldId reports the count for
    // every active channel (create mode).
    getChannelAttributeComplianceSummary = async (groupName: string, fieldId?: string) => {
        return this.doFetch<ChannelAttributeComplianceSummary>(
            `${this.channelAttributeMissingValuesRoute(groupName, fieldId)}/summary`,
            {method: 'GET'},
        );
    };

    // notifyChannelAdminsOfMissingAttributeValue sends a batched system-bot DM
    // to every unique channel admin of a channel missing a value for fieldId.
    notifyChannelAdminsOfMissingAttributeValue = async (groupName: string, fieldId: string) => {
        return this.doFetch<ChannelAttributeNotifyResult>(
            `${this.channelAttributeMissingValuesRoute(groupName, fieldId)}/notify`,
            {method: 'POST'},
        );
    };

    getUserCustomProfileAttributesValues = async (userID: string) => {
        const data = await this.doFetch<Record<string, string>>(
            `${this.getUserRoute(userID)}/custom_profile_attributes`,
            {method: 'GET'},
        );
        return data;
    };

    // Post Routes

    createPost = async (post: PartialExcept<Post, 'channel_id' | 'message'>) => {
        const result = await this.doFetch<Post>(
            `${this.getPostsRoute()}`,
            {method: 'post', body: JSON.stringify(post)},
        );
        const analyticsData = {channel_id: result.channel_id, post_id: result.id, user_actual_id: result.user_id, root_id: result.root_id} as PostAnalytics;
        if (post.metadata?.priority) {
            analyticsData.priority = post.metadata.priority.priority;
            analyticsData.requested_ack = post.metadata.priority.requested_ack;
            analyticsData.persistent_notifications = post.metadata.priority.persistent_notifications;
        }
        return result;
    };

    createPostEphemeral = async (userID: string, post: PartialExcept<Post, 'channel_id' | 'message'>) => {
        const result = await this.doFetch<Post>(
            `${this.getPostsRoute()}/ephemeral`,
            {method: 'post', body: JSON.stringify({user_id: userID, post})},
        );
        return result;
    };

    updatePost = (post: Post) => {
        return this.doFetch<Post>(
            `${this.getPostRoute(post.id)}`,
            {method: 'put', body: JSON.stringify(post)},
        );
    };

    getPost = (postId: string, includeDeleted?: boolean, retainContent?: boolean, propertyGroup?: string) => {
        return this.doFetch<Post>(
            `${this.getPostRoute(postId)}${buildQueryString({include_deleted: includeDeleted, retain_content: retainContent, propertyGroup})}`,
            {method: 'get'},
        );
    };

    patchPost = (postPatch: PartialExcept<Post, 'id'>) => {
        return this.doFetch<Post>(
            `${this.getPostRoute(postPatch.id)}/patch`,
            {method: 'put', body: JSON.stringify(postPatch)},
        );
    };

    deletePost = (postId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPostRoute(postId)}`,
            {method: 'delete'},
        );
    };

    revealBurnOnReadPost = (postId: string) => {
        return this.doFetch<Post>(
            `${this.getPostRoute(postId)}/reveal`,
            {method: 'get'},
        );
    };

    burnPostNow = (postId: string) => {
        return this.doFetch(
            `${this.getPostRoute(postId)}/burn`,
            {method: 'delete'},
        );
    };

    getPostThread = (postId: string, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        // this is to ensure we have backwards compatibility for `getPostThread`
        return this.getPaginatedPostThread(postId, {fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup});
    };

    getPaginatedPostThread = async (postId: string, options: FetchPaginatedThreadOptions): Promise<PaginatedPostList> => {
        // getting all option parameters with defaults from the options object and spread the rest
        const {
            fetchThreads = true,
            collapsedThreads = false,
            collapsedThreadsExtended = false,
            direction = 'down',
            fetchAll = false,
            perPage = fetchAll ? undefined : PER_PAGE_DEFAULT,
            ...rest
        } = options;

        return this.doFetch<PaginatedPostList>(
            `${this.getPostRoute(postId)}/thread${buildQueryString({skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, direction, perPage, ...rest})}`,
            {method: 'get'},
        );
    };

    getPosts = (channelId: string, page = 0, perPage = PER_PAGE_DEFAULT, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        return this.doFetch<PostList>(
            `${this.getChannelRoute(channelId)}/posts${buildQueryString({page, per_page: perPage, skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup})}`,
            {method: 'get'},
        );
    };

    getPostsUnread = (channelId: string, userId: string, limitAfter = DEFAULT_LIMIT_AFTER, limitBefore = DEFAULT_LIMIT_BEFORE, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        return this.doFetch<PostList>(
            `${this.getUserRoute(userId)}/channels/${channelId}/posts/unread${buildQueryString({limit_after: limitAfter, limit_before: limitBefore, skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup})}`,
            {method: 'get'},
        );
    };

    getPostsSince = (channelId: string, since: number, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        return this.doFetch<PostList>(
            `${this.getChannelRoute(channelId)}/posts${buildQueryString({since, skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup})}`,
            {method: 'get'},
        );
    };

    getPostsBefore = (channelId: string, postId: string, page = 0, perPage = PER_PAGE_DEFAULT, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        return this.doFetch<PostList>(
            `${this.getChannelRoute(channelId)}/posts${buildQueryString({before: postId, page, per_page: perPage, skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup})}`,
            {method: 'get'},
        );
    };

    getPostsAfter = (channelId: string, postId: string, page = 0, perPage = PER_PAGE_DEFAULT, fetchThreads = true, collapsedThreads = false, collapsedThreadsExtended = false, propertyGroup?: string) => {
        return this.doFetch<PostList>(
            `${this.getChannelRoute(channelId)}/posts${buildQueryString({after: postId, page, per_page: perPage, skipFetchThreads: !fetchThreads, collapsedThreads, collapsedThreadsExtended, propertyGroup})}`,
            {method: 'get'},
        );
    };

    getUserThreads = (
        userId: UserProfile['id'] = 'me',
        teamId: Team['id'],
        {
            before = '',
            after = '',
            perPage = PER_PAGE_DEFAULT,
            extended = false,
            deleted = false,
            unread = false,
            since = 0,
            totalsOnly = false,
            threadsOnly = false,
        },
    ) => {
        return this.doFetch<UserThreadList>(
            `${this.getUserThreadsRoute(userId, teamId)}${buildQueryString({before, after, per_page: perPage, extended, deleted, unread, since, totalsOnly, threadsOnly})}`,
            {method: 'get'},
        );
    };

    getUserThread = (userId: string, teamId: string, threadId: string, extended = false) => {
        const url = `${this.getUserThreadRoute(userId, teamId, threadId)}`;
        return this.doFetch<UserThreadWithPost>(
            `${url}${buildQueryString({extended})}`,
            {method: 'get'},
        );
    };

    updateThreadsReadForUser = (userId: string, teamId: string) => {
        const url = `${this.getUserThreadsRoute(userId, teamId)}/read`;
        return this.doFetch<StatusOK>(
            url,
            {method: 'put'},
        );
    };

    updateThreadReadForUser = (userId: string, teamId: string, threadId: string, timestamp: number) => {
        const url = `${this.getUserThreadRoute(userId, teamId, threadId)}/read/${timestamp}`;
        return this.doFetch<UserThread>(
            url,
            {method: 'put'},
        );
    };

    markThreadAsUnreadForUser = (userId: string, teamId: string, threadId: string, postId: string) => {
        const url = `${this.getUserThreadRoute(userId, teamId, threadId)}/set_unread/${postId}`;
        return this.doFetch<UserThread>(
            url,
            {method: 'post'},
        );
    };

    updateThreadFollowForUser = (userId: string, teamId: string, threadId: string, state: boolean) => {
        const url = this.getUserThreadRoute(userId, teamId, threadId) + '/following';
        return this.doFetch<StatusOK>(
            url,
            {method: state ? 'put' : 'delete'},
        );
    };

    getFileInfosForPost = (postId: string) => {
        return this.doFetch<FileInfo[]>(
            `${this.getPostRoute(postId)}/files/info`,
            {method: 'get'},
        );
    };

    getFileInfo = (fileId: string) => {
        return this.doFetch<FileInfo>(
            `${this.getFileRoute(fileId)}/info`,
            {method: 'get'},
        );
    };

    getFlaggedPosts = (userId: string, channelId = '', teamId = '', page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<PostList>(
            `${this.getUserRoute(userId)}/posts/flagged${buildQueryString({channel_id: channelId, team_id: teamId, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getPinnedPosts = (channelId: string) => {
        return this.doFetch<PostList>(
            `${this.getChannelRoute(channelId)}/pinned`,
            {method: 'get'},
        );
    };

    markPostAsUnread = (userId: string, postId: string) => {
        return this.doFetch<ChannelUnread>(
            `${this.getUserRoute(userId)}/posts/${postId}/set_unread`,
            {method: 'post', body: JSON.stringify({collapsed_threads_supported: true})},
        );
    };

    addPostReminder = (userId: string, postId: string, timestamp: number) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/posts/${postId}/reminder`,
            {method: 'post', body: JSON.stringify({target_time: timestamp})},
        );
    };

    pinPost = (postId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPostRoute(postId)}/pin`,
            {method: 'post'},
        );
    };

    unpinPost = (postId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPostRoute(postId)}/unpin`,
            {method: 'post'},
        );
    };

    getPostInfo = (postId: string) => {
        return this.doFetch<PostInfo>(
            `${this.getPostRoute(postId)}/info`,
            {method: 'get'},
        );
    };

    getPostsByIds = (postIds: string[]) => {
        return this.doFetch<Post[]>(
            `${this.getPostsRoute()}/ids`,
            {method: 'post', body: JSON.stringify(postIds)},
        );
    };

    getPostEditHistory = (postId: string) => {
        return this.doFetch<Post[]>(
            `${this.getPostRoute(postId)}/edit_history`,
            {method: 'get'},
        );
    };

    addReaction = (userId: string, postId: string, emojiName: string) => {
        return this.doFetch<Reaction>(
            `${this.getReactionsRoute()}`,
            {method: 'post', body: JSON.stringify({user_id: userId, post_id: postId, emoji_name: emojiName})},
        );
    };

    removeReaction = (userId: string, postId: string, emojiName: string) => {
        return this.doFetch<StatusOK>(
            `${this.getUserRoute(userId)}/posts/${postId}/reactions/${emojiName}`,
            {method: 'delete'},
        );
    };

    getReactionsForPost = (postId: string) => {
        return this.doFetch<Reaction[]>(
            `${this.getPostRoute(postId)}/reactions`,
            {method: 'get'},
        );
    };

    searchPostsWithParams = (teamId: string, params: any) => {
        let route = `${this.getPostsRoute()}/search`;
        if (teamId) {
            route = `${this.getTeamRoute(teamId)}/posts/search`;
        }

        return this.doFetch<PostSearchResults>(
            route,
            {method: 'post', body: JSON.stringify(params)},
        );
    };

    searchPosts = (teamId: string, terms: string, isOrSearch: boolean) => {
        return this.searchPostsWithParams(teamId, {terms, is_or_search: isOrSearch});
    };

    searchFilesWithParams = (teamId: string, params: any) => {
        let route = `${this.getFilesRoute()}/search`;
        if (teamId) {
            route = `${this.getTeamRoute(teamId)}/files/search`;
        }

        return this.doFetch<FileSearchResults>(
            route,
            {method: 'post', body: JSON.stringify(params)},
        );
    };

    searchFiles = (teamId: string, terms: string, isOrSearch: boolean) => {
        return this.searchFilesWithParams(teamId, {terms, is_or_search: isOrSearch});
    };

    doPostAction = (postId: string, actionId: string, selectedOption = '') => {
        return this.doPostActionWithCookie(postId, actionId, '', selectedOption, undefined, '');
    };

    doPostActionWithCookie = (postId: string, actionId: string, actionCookie: string, selectedOption = '', query?: Record<string, string>, integrationFormat = '') => {
        const msg: any = {
            selected_option: selectedOption,
        };
        if (actionCookie !== '') {
            msg.cookie = actionCookie;
        }
        if (query && Object.keys(query).length > 0) {
            msg.query = query;
        }
        if (integrationFormat) {
            msg.integration_format = integrationFormat;
        }
        return this.doFetch<PostActionResponse>(
            `${this.getPostRoute(postId)}/actions/${encodeURIComponent(actionId)}`,
            {method: 'post', body: JSON.stringify(msg)},
        );
    };

    // Files Routes

    getFileUrl(fileId: string, timestamp: number) {
        let url = `${this.getFileRoute(fileId)}`;
        if (timestamp) {
            url += `?${timestamp}`;
        }

        return url;
    }

    getFileThumbnailUrl(fileId: string, timestamp: number) {
        let url = `${this.getFileRoute(fileId)}/thumbnail`;
        if (timestamp) {
            url += `?${timestamp}`;
        }

        return url;
    }

    getFilePreviewUrl(fileId: string, timestamp: number) {
        let url = `${this.getFileRoute(fileId)}/preview`;
        if (timestamp) {
            url += `?${timestamp}`;
        }

        return url;
    }

    uploadFile = (fileFormData: any, isBookmark?: boolean) => {
        const request: any = {
            method: 'post',
            body: fileFormData,
        };

        return this.doFetch<FileUploadResponse>(
            `${this.getFilesRoute()}${buildQueryString({bookmark: isBookmark})}`,
            request,
        );
    };

    getFilePublicLink = (fileId: string) => {
        return this.doFetch<{
            link: string;
        }>(
            `${this.getFileRoute(fileId)}/link`,
            {method: 'get'},
        );
    };

    acknowledgePost = (postId: string, userId: string) => {
        return this.doFetch<PostAcknowledgement>(
            `${this.getUserRoute(userId)}/posts/${postId}/ack`,
            {method: 'post'},
        );
    };

    unacknowledgePost = (postId: string, userId: string) => {
        return this.doFetch<null>(
            `${this.getUserRoute(userId)}/posts/${postId}/ack`,
            {method: 'delete'},
        );
    };

    // Preference Routes

    savePreferences = (userId: string, preferences: PreferenceType[]) => {
        return this.doFetch<StatusOK>(
            `${this.getPreferencesRoute(userId)}`,
            {method: 'put', body: JSON.stringify(preferences)},
        );
    };

    getMyPreferences = () => {
        return this.doFetch<PreferenceType>(
            `${this.getPreferencesRoute('me')}`,
            {method: 'get'},
        );
    };

    getUserPreferences = (userId: string) => {
        return this.doFetch<PreferenceType[]>(
            `${this.getPreferencesRoute(userId)}`,
            {method: 'get'},
        );
    };

    deletePreferences = (userId: string, preferences: PreferenceType[]) => {
        return this.doFetch<StatusOK>(
            `${this.getPreferencesRoute(userId)}/delete`,
            {method: 'post', body: JSON.stringify(preferences)},
        );
    };

    // General Routes

    ping = (getServerStatus?: boolean, deviceId?: string) => {
        return this.doFetch<{
            status: string;
            ActiveSearchBackend: string;
            database_status: string;
            filestore_status: string;
            root_status: boolean;
        }>(
            `${this.getBaseRoute()}/system/ping${buildQueryString({get_server_status: getServerStatus, device_id: deviceId, use_rest_semantics: true})}`,
            {method: 'get'},
        );
    };

    upgradeToEnterprise = async () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/upgrade_to_enterprise`,
            {method: 'post'},
        );
    };

    upgradeToEnterpriseStatus = async () => {
        return this.doFetch<{
            percentage: number;
            error: string | null;
        }>(
            `${this.getBaseRoute()}/upgrade_to_enterprise/status`,
            {method: 'get'},
        );
    };

    isAllowedToUpgradeToEnterprise = async () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/upgrade_to_enterprise/allowed`,
            {method: 'get'},
        );
    };

    restartServer = async () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/restart`,
            {method: 'post'},
        );
    };

    logClientError = (message: string, level = LogLevel.Error) => {
        const url = `${this.getBaseRoute()}/logs`;

        if (!this.enableLogging) {
            throw new ClientError(this.getUrl(), {
                message: 'Logging disabled.',
                url,
            });
        }

        return this.doFetch<{
            message: string;
        }>(
            url,
            {method: 'post', body: JSON.stringify({message, level})},
        );
    };

    getClientConfig = () => {
        // Keep format=old for compatibility with pre-v11 servers
        return this.doFetch<ClientConfig>(
            `${this.getBaseRoute()}/config/client?format=old`,
            {method: 'get'},
        );
    };

    getClientLicenseOld = () => {
        return this.doFetch<ClientLicense>(
            `${this.getBaseRoute()}/license/client`,
            {method: 'get'},
        );
    };

    getLicenseLoadMetric = () => {
        return this.doFetch<{
            load: number;
        }>(
            `${this.getBaseRoute()}/license/load_metric`,
            {method: 'get'},
        );
    };

    setFirstAdminVisitMarketplaceStatus = async () => {
        return this.doFetch<StatusOK>(
            `${this.getPluginsRoute()}/marketplace/first_admin_visit`,
            {method: 'post', body: JSON.stringify({first_admin_visit_marketplace_status: true})},
        );
    };

    getFirstAdminVisitMarketplaceStatus = async () => {
        return this.doFetch<SystemSetting>(
            `${this.getPluginsRoute()}/marketplace/first_admin_visit`,
            {method: 'get'},
        );
    };

    getFirstAdminSetupComplete = async () => {
        return this.doFetch<SystemSetting>(
            `${this.getSystemRoute()}/onboarding/complete`,
            {method: 'get'},
        );
    };

    getTranslations = (url: string) => {
        return this.doFetch<Record<string, string>>(
            url,
            {method: 'get'},
        );
    };

    getWebSocketUrl = () => {
        return `${this.getBaseRoute()}/websocket`;
    };

    // Integration Routes

    createIncomingWebhook = (hook: PartialExcept<IncomingWebhook, 'channel_id'>) => {
        return this.doFetch<IncomingWebhook>(
            `${this.getIncomingHooksRoute()}`,
            {method: 'post', body: JSON.stringify(hook)},
        );
    };

    getIncomingWebhook = (hookId: string) => {
        return this.doFetch<IncomingWebhook>(
            `${this.getIncomingHookRoute(hookId)}`,
            {method: 'get'},
        );
    };

    getIncomingWebhooks = (teamId = '', page = 0, perPage = PER_PAGE_DEFAULT, includeTotalCount = false) => {
        const queryParams: any = {
            page,
            per_page: perPage,
            include_total_count: includeTotalCount,
        };

        if (teamId) {
            queryParams.team_id = teamId;
        }

        return this.doFetch<IncomingWebhook[] | IncomingWebhooksWithCount>(
            `${this.getIncomingHooksRoute()}${buildQueryString(queryParams)}`,
            {method: 'get'},
        );
    };

    removeIncomingWebhook = (hookId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getIncomingHookRoute(hookId)}`,
            {method: 'delete'},
        );
    };

    updateIncomingWebhook = (hook: IncomingWebhook) => {
        return this.doFetch<IncomingWebhook>(
            `${this.getIncomingHookRoute(hook.id)}`,
            {method: 'put', body: JSON.stringify(hook)},
        );
    };

    createOutgoingWebhook = (hook: OutgoingWebhook) => {
        return this.doFetch<OutgoingWebhook>(
            `${this.getOutgoingHooksRoute()}`,
            {method: 'post', body: JSON.stringify(hook)},
        );
    };

    getOutgoingWebhook = (hookId: string) => {
        return this.doFetch<OutgoingWebhook>(
            `${this.getOutgoingHookRoute(hookId)}`,
            {method: 'get'},
        );
    };

    getOutgoingWebhooks = (channelId = '', teamId = '', page = 0, perPage = PER_PAGE_DEFAULT) => {
        const queryParams: any = {
            page,
            per_page: perPage,
        };

        if (channelId) {
            queryParams.channel_id = channelId;
        }

        if (teamId) {
            queryParams.team_id = teamId;
        }

        return this.doFetch<OutgoingWebhook[]>(
            `${this.getOutgoingHooksRoute()}${buildQueryString(queryParams)}`,
            {method: 'get'},
        );
    };

    removeOutgoingWebhook = (hookId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getOutgoingHookRoute(hookId)}`,
            {method: 'delete'},
        );
    };

    updateOutgoingWebhook = (hook: OutgoingWebhook) => {
        return this.doFetch<OutgoingWebhook>(
            `${this.getOutgoingHookRoute(hook.id)}`,
            {method: 'put', body: JSON.stringify(hook)},
        );
    };

    regenOutgoingHookToken = (id: string) => {
        return this.doFetch<OutgoingWebhook>(
            `${this.getOutgoingHookRoute(id)}/regen_token`,
            {method: 'post'},
        );
    };

    getCommandsList = (teamId: string) => {
        return this.doFetch<Command[]>(
            `${this.getCommandsRoute()}?team_id=${teamId}`,
            {method: 'get'},
        );
    };

    getCommandAutocompleteSuggestionsList = (userInput: string, teamId: string, commandArgs: CommandArgs) => {
        return this.doFetch<AutocompleteSuggestion[]>(
            `${this.getTeamRoute(teamId)}/commands/autocomplete_suggestions${buildQueryString({...commandArgs, user_input: userInput})}`,
            {method: 'get'},
        );
    };

    getAutocompleteCommandsList = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Command[]>(
            `${this.getTeamRoute(teamId)}/commands/autocomplete${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getCustomTeamCommands = (teamId: string) => {
        return this.doFetch<Command[]>(
            `${this.getCommandsRoute()}?team_id=${teamId}&custom_only=true`,
            {method: 'get'},
        );
    };

    executeCommand = (command: string, commandArgs: CommandArgs) => {
        return this.doFetch<CommandResponse>(
            `${this.getCommandsRoute()}/execute`,
            {method: 'post', body: JSON.stringify({command, ...commandArgs})},
        );
    };

    addCommand = (command: Command) => {
        return this.doFetch<Command>(
            `${this.getCommandsRoute()}`,
            {method: 'post', body: JSON.stringify(command)},
        );
    };

    editCommand = (command: Command) => {
        return this.doFetch<Command>(
            `${this.getCommandsRoute()}/${command.id}`,
            {method: 'put', body: JSON.stringify(command)},
        );
    };

    regenCommandToken = (id: string) => {
        return this.doFetch<{
            token: string;
        }>(
            `${this.getCommandsRoute()}/${id}/regen_token`,
            {method: 'put'},
        );
    };

    deleteCommand = (id: string) => {
        return this.doFetch<StatusOK>(
            `${this.getCommandsRoute()}/${id}`,
            {method: 'delete'},
        );
    };

    createOAuthApp = (app: PartialExcept<OAuthApp, 'name' | 'description' | 'callback_urls' | 'homepage'>) => {
        return this.doFetch<OAuthApp>(
            `${this.getOAuthAppsRoute()}`,
            {method: 'post', body: JSON.stringify(app)},
        );
    };

    editOAuthApp = (app: OAuthApp) => {
        return this.doFetch<OAuthApp>(
            `${this.getOAuthAppsRoute()}/${app.id}`,
            {method: 'put', body: JSON.stringify(app)},
        );
    };

    getOAuthApps = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<OAuthApp[]>(
            `${this.getOAuthAppsRoute()}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getAppsOAuthAppIDs = () => {
        return this.doFetch<string[]>(
            `${this.getAppsProxyRoute()}/api/v1/oauth-app-ids`,
            {method: 'get'},
        );
    };

    getAppsBotIDs = () => {
        return this.doFetch<string[]>(
            `${this.getAppsProxyRoute()}/api/v1/bot-ids`,
            {method: 'get'},
        );
    };

    getOAuthApp = (appId: string) => {
        return this.doFetch<OAuthApp>(
            `${this.getOAuthAppRoute(appId)}`,
            {method: 'get'},
        );
    };

    getOutgoingOAuthConnections = (teamId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<OutgoingOAuthConnection[]>(
            `${this.getOutgoingOAuthConnectionsRoute()}${buildQueryString({team_id: teamId, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getOutgoingOAuthConnectionsForAudience = (teamId: string, audience: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<OutgoingOAuthConnection[]>(
            `${this.getOutgoingOAuthConnectionsRoute()}${buildQueryString({team_id: teamId, page, per_page: perPage, audience})}`,
            {method: 'get'},
        );
    };

    getOutgoingOAuthConnection = (teamId: string, connectionId: string) => {
        return this.doFetch<OutgoingOAuthConnection>(
            `${this.getOutgoingOAuthConnectionRoute(connectionId)}${buildQueryString({team_id: teamId})}`,
            {method: 'get'},
        );
    };

    createOutgoingOAuthConnection = (teamId: string, connection: OutgoingOAuthConnection) => {
        return this.doFetch<OutgoingOAuthConnection>(
            `${this.getOutgoingOAuthConnectionsRoute()}${buildQueryString({team_id: teamId})}`,
            {method: 'post', body: JSON.stringify(connection)},
        );
    };

    editOutgoingOAuthConnection = (teamId: string, connection: OutgoingOAuthConnection) => {
        return this.doFetch<OutgoingOAuthConnection>(
            `${this.getOutgoingOAuthConnectionsRoute()}/${connection.id}${buildQueryString({team_id: teamId})}`,
            {method: 'put', body: JSON.stringify(connection)},
        );
    };

    validateOutgoingOAuthConnection = (teamId: string, connection: OutgoingOAuthConnection) => {
        return this.doFetch<OutgoingOAuthConnection>(
            `${this.getOutgoingOAuthConnectionsRoute()}/validate${buildQueryString({team_id: teamId})}`,
            {method: 'post', body: JSON.stringify(connection)},
        );
    };

    getOAuthAppInfo = (appId: string) => {
        return this.doFetch<OAuthApp>(
            `${this.getOAuthAppRoute(appId)}/info`,
            {method: 'get'},
        );
    };

    deleteOAuthApp = (appId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getOAuthAppRoute(appId)}`,
            {method: 'delete'},
        );
    };

    regenOAuthAppSecret = (appId: string) => {
        return this.doFetch<OAuthApp>(
            `${this.getOAuthAppRoute(appId)}/regen_secret`,
            {method: 'post'},
        );
    };

    deleteOutgoingOAuthConnection = (connectionId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getOutgoingOAuthConnectionRoute(connectionId)}`,
            {method: 'delete'},
        );
    };

    submitInteractiveDialog = (data: DialogSubmission) => {
        return this.doFetch<SubmitDialogResponse>(
            `${this.getBaseRoute()}/actions/dialogs/submit`,
            {method: 'post', body: JSON.stringify(data)},
        );
    };

    executeDialogAction = (url: string, context?: Record<string, string>, channelId?: string, teamId?: string) => {
        return this.doFetch<PostActionResponse>(
            `${this.getBaseRoute()}/actions/dialogs/execute`,
            {method: 'post', body: JSON.stringify({url, context, channel_id: channelId, team_id: teamId})},
        );
    };

    lookupInteractiveDialog = (data: DialogSubmission) => {
        return this.doFetch<{items: Array<{text: string; value: string}>}>(
            `${this.getBaseRoute()}/actions/dialogs/lookup`,
            {method: 'post', body: JSON.stringify(data)},
        );
    };

    // Emoji Routes

    createCustomEmoji = (emoji: PartialExcept<CustomEmoji, 'name' | 'creator_id'>, imageData: File) => {
        const formData = new FormData();
        formData.append('image', imageData);
        formData.append('emoji', JSON.stringify(emoji));
        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<CustomEmoji>(
            `${this.getEmojisRoute()}`,
            request,
        );
    };

    getCustomEmoji = (id: string) => {
        return this.doFetch<CustomEmoji>(
            `${this.getEmojisRoute()}/${id}`,
            {method: 'get'},
        );
    };

    getCustomEmojiByName = (name: string) => {
        return this.doFetch<CustomEmoji>(
            `${this.getEmojisRoute()}/name/${name}`,
            {method: 'get'},
        );
    };

    getCustomEmojisByNames = (names: string[]) => {
        return this.doFetch<CustomEmoji[]>(
            `${this.getEmojisRoute()}/names`,
            {method: 'post', body: JSON.stringify(names)},
        );
    };

    getCustomEmojis = (page = 0, perPage = PER_PAGE_DEFAULT, sort = '') => {
        return this.doFetch<CustomEmoji[]>(
            `${this.getEmojisRoute()}${buildQueryString({page, per_page: perPage, sort})}`,
            {method: 'get'},
        );
    };

    deleteCustomEmoji = (emojiId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getEmojiRoute(emojiId)}`,
            {method: 'delete'},
        );
    };

    getSystemEmojiImageUrl = (filename: string) => {
        const extension = filename.endsWith('.png') ? '' : '.png';
        return `${this.url}/static/emoji/${filename}${extension}`;
    };

    getCustomEmojiImageUrl = (id: string) => {
        return `${this.getEmojiRoute(id)}/image`;
    };

    searchCustomEmoji = (term: string, options = {}) => {
        return this.doFetch<CustomEmoji[]>(
            `${this.getEmojisRoute()}/search`,
            {method: 'post', body: JSON.stringify({term, ...options})},
        );
    };

    autocompleteCustomEmoji = (name: string) => {
        return this.doFetch<CustomEmoji[]>(
            `${this.getEmojisRoute()}/autocomplete${buildQueryString({name})}`,
            {method: 'get'},
        );
    };

    // Data Retention

    getDataRetentionPolicy = () => {
        return this.doFetch<DataRetentionPolicy>(
            `${this.getDataRetentionRoute()}/policy`,
            {method: 'get'},
        );
    };

    getDataRetentionCustomPolicies = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<GetDataRetentionCustomPoliciesRequest>(
            `${this.getDataRetentionRoute()}/policies${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getDataRetentionCustomPolicy = (id: string) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}`,
            {method: 'get'},
        );
    };

    deleteDataRetentionCustomPolicy = (id: string) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}`,
            {method: 'delete'},
        );
    };

    searchDataRetentionCustomPolicyChannels = (policyId: string, term: string, opts: ChannelSearchOpts) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${policyId}/channels/search`,
            {method: 'post', body: JSON.stringify({term, ...opts})},
        );
    };

    searchDataRetentionCustomPolicyTeams = (policyId: string, term: string, opts: TeamSearchOpts) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${policyId}/teams/search`,
            {method: 'post', body: JSON.stringify({term, ...opts})},
        );
    };

    getDataRetentionCustomPolicyTeams = (id: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Team[]>(
            `${this.getDataRetentionRoute()}/policies/${id}/teams${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getDataRetentionCustomPolicyChannels = (id: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<{channels: Channel[]; total_count: number}>(
            `${this.getDataRetentionRoute()}/policies/${id}/channels${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    createDataRetentionPolicy = (policy: CreateDataRetentionCustomPolicy) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies`,
            {method: 'post', body: JSON.stringify(policy)},
        );
    };

    updateDataRetentionPolicy = (id: string, policy: PatchDataRetentionCustomPolicy) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}`,
            {method: 'PATCH', body: JSON.stringify(policy)},
        );
    };
    addDataRetentionPolicyTeams = (id: string, teams: string[]) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}/teams`,
            {method: 'post', body: JSON.stringify(teams)},
        );
    };
    removeDataRetentionPolicyTeams = (id: string, teams: string[]) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}/teams`,
            {method: 'delete', body: JSON.stringify(teams)},
        );
    };
    addDataRetentionPolicyChannels = (id: string, channels: string[]) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}/channels`,
            {method: 'post', body: JSON.stringify(channels)},
        );
    };
    removeDataRetentionPolicyChannels = (id: string, channels: string[]) => {
        return this.doFetch<DataRetentionCustomPolicies>(
            `${this.getDataRetentionRoute()}/policies/${id}/channels`,
            {method: 'delete', body: JSON.stringify(channels)},
        );
    };

    // Jobs Routes
    getJob = (id: string) => {
        return this.doFetch<Job>(
            `${this.getJobsRoute()}/${id}`,
            {method: 'get'},
        );
    };

    getJobs = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Job[]>(
            `${this.getJobsRoute()}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getJobsByType = (type: string, page = 0, perPage = PER_PAGE_DEFAULT, teamId?: string, policyId?: string) => {
        const params: Record<string, any> = {page, per_page: perPage};
        if (teamId) {
            params.team_id = teamId;
        }
        if (policyId) {
            params.policy_id = policyId;
        }
        return this.doFetch<Job[]>(
            `${this.getJobsRoute()}/type/${type}${buildQueryString(params)}`,
            {method: 'get'},
        );
    };

    createJob = (job: JobTypeBase & {data?: any}) => {
        return this.doFetch<Job>(
            `${this.getJobsRoute()}`,
            {method: 'post', body: JSON.stringify(job)},
        );
    };

    cancelJob = (id: string) => {
        return this.doFetch<StatusOK>(
            `${this.getJobsRoute()}/${id}/cancel`,
            {method: 'post'},
        );
    };

    // Recaps Routes
    createRecap = (request: CreateRecapRequest) => {
        return this.doFetch<Recap>(
            `${this.getRecapsRoute()}`,
            {method: 'post', body: JSON.stringify(request)},
        );
    };

    getRecaps = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Recap[]>(
            `${this.getRecapsRoute()}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getRecap = (recapId: string) => {
        return this.doFetch<Recap>(
            `${this.getRecapsRoute()}/${recapId}`,
            {method: 'get'},
        );
    };

    markRecapAsRead = (recapId: string) => {
        return this.doFetch<Recap>(
            `${this.getRecapsRoute()}/${recapId}/read`,
            {method: 'post'},
        );
    };

    markRecapsAsViewed = () => {
        return this.doFetch<{recap_ids: string[]}>(
            `${this.getRecapsRoute()}/mark_viewed`,
            {method: 'post'},
        );
    };

    regenerateRecap = (recapId: string) => {
        return this.doFetch<Recap>(
            `${this.getRecapsRoute()}/${recapId}/regenerate`,
            {method: 'post'},
        );
    };

    deleteRecap = (recapId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getRecapsRoute()}/${recapId}`,
            {method: 'delete'},
        );
    };

    getRecapLimitStatus = () => {
        return this.doFetch<RecapLimitStatus>(
            `${this.getRecapsRoute()}/limit_status`,
            {method: 'get'},
        );
    };

    // Scheduled Recaps Routes
    createScheduledRecap = (input: ScheduledRecapInput) => {
        return this.doFetch<ScheduledRecap>(
            `${this.getScheduledRecapsRoute()}`,
            {method: 'post', body: JSON.stringify(input)},
        );
    };

    getScheduledRecaps = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<ScheduledRecap[]>(
            `${this.getScheduledRecapsRoute()}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getScheduledRecap = (id: string) => {
        return this.doFetch<ScheduledRecap>(
            `${this.getScheduledRecapsRoute()}/${id}`,
            {method: 'get'},
        );
    };

    updateScheduledRecap = (id: string, input: ScheduledRecapInput) => {
        return this.doFetch<ScheduledRecap>(
            `${this.getScheduledRecapsRoute()}/${id}`,
            {method: 'put', body: JSON.stringify(input)},
        );
    };

    deleteScheduledRecap = (id: string) => {
        return this.doFetch<void>(
            `${this.getScheduledRecapsRoute()}/${id}`,
            {method: 'delete'},
        );
    };

    pauseScheduledRecap = (id: string) => {
        return this.doFetch<ScheduledRecap>(
            `${this.getScheduledRecapsRoute()}/${id}/pause`,
            {method: 'post'},
        );
    };

    resumeScheduledRecap = (id: string) => {
        return this.doFetch<ScheduledRecap>(
            `${this.getScheduledRecapsRoute()}/${id}/resume`,
            {method: 'post'},
        );
    };

    // Admin Routes

    getLogs = (logFilter: LogFilterQuery, page = 0, perPage = LOGS_PER_PAGE_DEFAULT) => {
        return this.doFetch<LogsByNode>(
            `${this.getBaseRoute()}/logs/query${buildQueryString({page, logs_per_page: perPage})}`,
            {method: 'post', body: JSON.stringify(logFilter)},
        );
    };

    getPlainLogs = (page = 0, perPage = LOGS_PER_PAGE_DEFAULT) => {
        return this.doFetch<string[]>(
            `${this.getBaseRoute()}/logs${buildQueryString({page, logs_per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getAudits = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Audit[]>(
            `${this.getBaseRoute()}/audits${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getConfig = () => {
        return this.doFetch<AdminConfig>(
            `${this.getBaseRoute()}/config`,
            {method: 'get'},
        );
    };

    updateConfig = (config: AdminConfig) => {
        return this.doFetch<AdminConfig>(
            `${this.getBaseRoute()}/config`,
            {method: 'put', body: JSON.stringify(config)},
        );
    };

    patchConfig = (patch: DeepPartial<AdminConfig>) => {
        return this.doFetch<AdminConfig>(
            `${this.getBaseRoute()}/config/patch`,
            {method: 'put', body: JSON.stringify(patch)},
        );
    };

    reloadConfig = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/config/reload`,
            {method: 'post'},
        );
    };

    // Agent Routes
    getAgents = () => {
        return this.doFetch<Agent[]>(
            `${this.getAgentsRoute()}`,
            {method: 'get'},
        );
    };

    getAgentsStatus = () => {
        return this.doFetch<{available: boolean; reason?: string}>(
            `${this.getAgentsRoute()}/status`,
            {method: 'get'},
        );
    };

    getLLMServices = () => {
        return this.doFetch<LLMService[]>(
            `${this.getBaseRoute()}/llmservices`,
            {method: 'get'},
        );
    };

    getEnvironmentConfig = () => {
        return this.doFetch<EnvironmentConfig>(
            `${this.getBaseRoute()}/config/environment`,
            {method: 'get'},
        );
    };

    sendTestNotificaiton = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/notifications/test`,
            {method: 'post'},
        );
    };

    testEmail = (config?: AdminConfig) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/email/test`,
            {method: 'post', body: JSON.stringify(config)},
        );
    };

    testSiteURL = (siteURL: string) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/site_url/test`,
            {method: 'post', body: JSON.stringify({site_url: siteURL})},
        );
    };

    /**
     * @deprecated Use testFileStoreConnection instead. The /file/s3_test
     * endpoint is kept for backwards compatibility but now routes through
     * the same backend-agnostic handler.
     */
    testS3Connection = (config?: AdminConfig) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/file/s3_test`,
            {method: 'post', body: JSON.stringify(config)},
        );
    };

    testFileStoreConnection = (config?: AdminConfig) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/file/test`,
            {method: 'post', body: JSON.stringify(config)},
        );
    };

    invalidateCaches = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/caches/invalidate`,
            {method: 'post'},
        );
    };

    recycleDatabase = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/database/recycle`,
            {method: 'post'},
        );
    };

    createComplianceReport = (job: Partial<Compliance>) => {
        return this.doFetch<Compliance>(
            `${this.getBaseRoute()}/compliance/reports`,
            {method: 'post', body: JSON.stringify(job)},
        );
    };

    getComplianceReport = (reportId: string) => {
        return this.doFetch<Compliance>(
            `${this.getBaseRoute()}/compliance/reports/${reportId}`,
            {method: 'get'},
        );
    };

    getComplianceReports = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Compliance[]>(
            `${this.getBaseRoute()}/compliance/reports${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    uploadBrandImage = (imageData: File) => {
        const formData = new FormData();
        formData.append('image', imageData);
        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<StatusOK>(
            `${this.getBrandRoute()}/image`,
            request,
        );
    };

    deleteBrandImage = () => {
        return this.doFetch<StatusOK>(
            `${this.getBrandRoute()}/image`,
            {method: 'delete'},
        );
    };

    getClusterStatus = () => {
        return this.doFetch<ClusterInfo[]>(
            `${this.getBaseRoute()}/cluster/status`,
            {method: 'get'},
        );
    };

    testLdap = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/test`,
            {method: 'post'},
        );
    };

    testLdapConnection = (settings: LdapSettings) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/test_connection`,
            {method: 'post', body: JSON.stringify(settings)},
        );
    };

    testLdapDiagnostics = (settings: LdapSettings, testType: LdapDiagnosticTestType) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/test_diagnostics?test=${testType}`,
            {method: 'post', body: JSON.stringify(settings)},
        );
    };

    testLdapFilters = (settings: LdapSettings) => {
        return this.testLdapDiagnostics(settings, LdapDiagnosticTestType.FILTERS);
    };

    testLdapAttributes = (settings: LdapSettings) => {
        return this.testLdapDiagnostics(settings, LdapDiagnosticTestType.ATTRIBUTES);
    };

    testLdapGroupAttributes = (settings: LdapSettings) => {
        return this.testLdapDiagnostics(settings, LdapDiagnosticTestType.GROUP_ATTRIBUTES);
    };

    syncLdap = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/sync`,
            {method: 'post'},
        );
    };

    getLdapGroups = (page = 0, perPage = PER_PAGE_DEFAULT, opts = {}) => {
        const query = {page, per_page: perPage, ...opts};
        return this.doFetch<{
            count: number;
            groups: MixedUnlinkedGroup[];
        }>(
            `${this.getBaseRoute()}/ldap/groups${buildQueryString(query)}`,
            {method: 'get'},
        );
    };

    linkLdapGroup = (key: string) => {
        return this.doFetch<Group>(
            `${this.getBaseRoute()}/ldap/groups/${encodeURI(key)}/link`,
            {method: 'post'},
        );
    };

    unlinkLdapGroup = (key: string) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/groups/${encodeURI(key)}/link`,
            {method: 'delete'},
        );
    };

    getSamlCertificateStatus = () => {
        return this.doFetch<SamlCertificateStatus>(
            `${this.getBaseRoute()}/saml/certificate/status`,
            {method: 'get'},
        );
    };

    uploadPublicSamlCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/public`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    uploadPrivateSamlCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/private`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    uploadPublicLdapCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/certificate/public`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    uploadPrivateLdapCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/certificate/private`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    uploadIdpSamlCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/idp`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    uploadAuditLogCertificate = (fileData: File) => {
        const formData = new FormData();
        formData.append('certificate', fileData);

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/audit_logs/certificate`,
            {
                method: 'post',
                body: formData,
            },
        );
    };

    removeAuditLogCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/audit_logs/certificate`,
            {method: 'delete'},
        );
    };

    deletePublicSamlCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/public`,
            {method: 'delete'},
        );
    };

    deletePrivateSamlCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/private`,
            {method: 'delete'},
        );
    };

    deletePublicLdapCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/certificate/public`,
            {method: 'delete'},
        );
    };

    deletePrivateLdapCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/ldap/certificate/private`,
            {method: 'delete'},
        );
    };

    deleteIdpSamlCertificate = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/idp`,
            {method: 'delete'},
        );
    };

    testElasticsearch = (config?: AdminConfig) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/elasticsearch/test`,
            {method: 'post', body: JSON.stringify(config)},
        );
    };

    purgeElasticsearchIndexes = (indexes?: string[]) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/elasticsearch/purge_indexes${indexes && indexes.length > 0 ? '?index=' + indexes.join(',') : ''}`,
            {method: 'post'},
        );
    };

    uploadLicense = (fileData: File) => {
        const formData = new FormData();
        formData.append('license', fileData);

        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<License>(
            `${this.getBaseRoute()}/license`,
            request,
        );
    };

    previewLicense = (fileData: File) => {
        const formData = new FormData();
        formData.append('license', fileData);

        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<License>(
            `${this.getBaseRoute()}/license/preview`,
            request,
        );
    };

    requestTrialLicense = (body: RequestLicenseBody) => {
        return this.doFetchWithResponse<ClientLicense>(
            `${this.getBaseRoute()}/trial-license`,
            {method: 'POST', body: JSON.stringify(body)},
        );
    };

    removeLicense = () => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/license`,
            {method: 'delete'},
        );
    };

    getPrevTrialLicense = () => {
        return this.doFetch<ClientLicense>(
            `${this.getBaseRoute()}/trial-license/prev`,
            {method: 'get'},
        );
    };

    getAnalytics = (name = 'standard', teamId = '') => {
        return this.doFetch<AnalyticsRow[]>(
            `${this.getBaseRoute()}/analytics/old${buildQueryString({name, team_id: teamId})}`,
            {method: 'get'},
        );
    };

    // Role Routes

    getRole = (roleId: string) => {
        return this.doFetch<Role>(
            `${this.getRolesRoute()}/${roleId}`,
            {method: 'get'},
        );
    };

    getRoleByName = (roleName: string) => {
        return this.doFetch<Role>(
            `${this.getRolesRoute()}/name/${roleName}`,
            {method: 'get'},
        );
    };

    getRolesByNames = (rolesNames: string[]) => {
        return this.doFetch<Role[]>(
            `${this.getRolesRoute()}/names`,
            {method: 'post', body: JSON.stringify(rolesNames)},
        );
    };

    patchRole = (roleId: string, rolePatch: Partial<Role>) => {
        return this.doFetch<Role>(
            `${this.getRolesRoute()}/${roleId}/patch`,
            {method: 'put', body: JSON.stringify(rolePatch)},
        );
    };

    // Scheme Routes

    getSchemes = (scope = '', page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Scheme[]>(
            `${this.getSchemesRoute()}${buildQueryString({scope, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    createScheme = (scheme: PartialExcept<Scheme, 'display_name' | 'scope'>) => {
        return this.doFetch<Scheme>(
            `${this.getSchemesRoute()}`,
            {method: 'post', body: JSON.stringify(scheme)},
        );
    };

    getScheme = (schemeId: string) => {
        return this.doFetch<Scheme>(
            `${this.getSchemesRoute()}/${schemeId}`,
            {method: 'get'},
        );
    };

    deleteScheme = (schemeId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getSchemesRoute()}/${schemeId}`,
            {method: 'delete'},
        );
    };

    patchScheme = (schemeId: string, schemePatch: Partial<Scheme>) => {
        return this.doFetch<Scheme>(
            `${this.getSchemesRoute()}/${schemeId}/patch`,
            {method: 'put', body: JSON.stringify(schemePatch)},
        );
    };

    getSchemeTeams = (schemeId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Team[]>(
            `${this.getSchemesRoute()}/${schemeId}/teams${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getSchemeChannels = (schemeId: string, page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Channel[]>(
            `${this.getSchemesRoute()}/${schemeId}/channels${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    // Plugin Routes

    uploadPlugin = async (fileData: File, force = false) => {
        const formData = new FormData();
        if (force) {
            formData.append('force', 'true');
        }
        formData.append('plugin', fileData);

        const request: any = {
            method: 'post',
            body: formData,
        };

        return this.doFetch<PluginManifest>(
            this.getPluginsRoute(),
            request,
        );
    };

    installPluginFromUrl = (pluginDownloadUrl: string, force = false) => {
        const queryParams = {plugin_download_url: pluginDownloadUrl, force};

        return this.doFetch<PluginManifest>(
            `${this.getPluginsRoute()}/install_from_url${buildQueryString(queryParams)}`,
            {method: 'post'},
        );
    };

    getPlugins = () => {
        return this.doFetch<PluginsResponse>(
            this.getPluginsRoute(),
            {method: 'get'},
        );
    };

    getRemoteMarketplacePlugins = (filter: string) => {
        return this.doFetch<MarketplacePlugin[]>(
            `${this.getPluginsMarketplaceRoute()}${buildQueryString({filter: filter || '', remote_only: true})}`,
            {method: 'get'},
        );
    };

    getMarketplacePlugins = (filter: string, localOnly = false) => {
        return this.doFetch<MarketplacePlugin[]>(
            `${this.getPluginsMarketplaceRoute()}${buildQueryString({filter: filter || '', local_only: localOnly})}`,
            {method: 'get'},
        );
    };

    installMarketplacePlugin = (id: string) => {
        return this.doFetch<MarketplacePlugin>(
            `${this.getPluginsMarketplaceRoute()}`,
            {method: 'post', body: JSON.stringify({id})},
        );
    };

    getMarketplaceApps = (filter: string) => {
        return this.doFetch<MarketplaceApp[]>(
            `${this.getAppsProxyRoute()}/api/v1/marketplace${buildQueryString({filter: filter || ''})}`,
            {method: 'get'},
        );
    };

    getPluginStatuses = () => {
        return this.doFetch<PluginStatus[]>(
            `${this.getPluginsRoute()}/statuses`,
            {method: 'get'},
        );
    };

    removePlugin = (pluginId: string) => {
        return this.doFetch<StatusOK>(
            this.getPluginRoute(pluginId),
            {method: 'delete'},
        );
    };

    getWebappPlugins = () => {
        return this.doFetch<ClientPluginManifest[]>(
            `${this.getPluginsRoute()}/webapp`,
            {method: 'get'},
        );
    };

    enablePlugin = (pluginId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPluginRoute(pluginId)}/enable`,
            {method: 'post'},
        );
    };

    disablePlugin = (pluginId: string) => {
        return this.doFetch<StatusOK>(
            `${this.getPluginRoute(pluginId)}/disable`,
            {method: 'post'},
        );
    };

    // Groups
    linkGroupSyncable = (groupID: string, syncableID: string, syncableType: string, patch: Partial<SyncablePatch>) => {
        return this.doFetch<GroupSyncable>(
            `${this.getGroupRoute(groupID)}/${syncableType}s/${syncableID}/link`,
            {method: 'post', body: JSON.stringify(patch)},
        );
    };

    unlinkGroupSyncable = (groupID: string, syncableID: string, syncableType: string) => {
        return this.doFetch<StatusOK>(
            `${this.getGroupRoute(groupID)}/${syncableType}s/${syncableID}/link`,
            {method: 'delete'},
        );
    };

    getGroupSyncables = (groupID: string, syncableType: string) => {
        return this.doFetch<GroupSyncable[]>(
            `${this.getGroupRoute(groupID)}/${syncableType}s`,
            {method: 'get'},
        );
    };

    getGroup = (groupID: string, includeMemberCount = false) => {
        return this.doFetch<Group>(
            `${this.getGroupRoute(groupID)}${buildQueryString({include_member_count: includeMemberCount})}`,
            {method: 'get'},
        );
    };

    getGroupStats = (groupID: string) => {
        return this.doFetch<GroupStats>(
            `${this.getGroupRoute(groupID)}/stats`,
            {method: 'get'},
        );
    };

    getGroups = (opts: GetGroupsForUserParams | GetGroupsParams) => {
        return this.doFetch<Group[]>(
            `${this.getGroupsRoute()}${buildQueryString(opts)}`,
            {method: 'get'},
        );
    };

    getGroupsByUserId = (userID: string) => {
        return this.doFetch<Group[]>(
            `${this.getUsersRoute()}/${userID}/groups`,
            {method: 'get'},
        );
    };

    getGroupsByNames = (names: string[]) => {
        return this.doFetch<Group[]>(
            `${this.getGroupsRoute()}/names`,
            {method: 'post', body: JSON.stringify(names)},
        );
    };

    getGroupsNotAssociatedToTeam = (teamID: string, q = '', page = 0, perPage = PER_PAGE_DEFAULT, source = 'ldap', onlySyncableSources = false) => {
        return this.doFetch<Group[]>(
            `${this.getGroupsRoute()}${buildQueryString({not_associated_to_team: teamID, page, per_page: perPage, q, include_member_count: true, group_source: source, only_syncable_sources: onlySyncableSources})}`,
            {method: 'get'},
        );
    };

    getGroupsNotAssociatedToChannel = (channelID: string, q = '', page = 0, perPage = PER_PAGE_DEFAULT, filterParentTeamPermitted = false, source = 'ldap', onlySyncableSources = false) => {
        const query = {
            not_associated_to_channel: channelID,
            page,
            per_page: perPage,
            q,
            include_member_count: true,
            filter_parent_team_permitted: filterParentTeamPermitted,
            group_source: source,
            only_syncable_sources: onlySyncableSources,
        };
        return this.doFetch<Group[]>(
            `${this.getGroupsRoute()}${buildQueryString(query)}`,
            {method: 'get'},
        );
    };

    createGroupWithUserIds = (group: GroupCreateWithUserIds) => {
        return this.doFetch<Group>(
            this.getGroupsRoute(),
            {method: 'post', body: JSON.stringify(group)},
        );
    };

    addUsersToGroup = (groupId: string, userIds: string[]) => {
        return this.doFetch<GroupMember[]>(
            `${this.getGroupRoute(groupId)}/members`,
            {method: 'post', body: JSON.stringify({user_ids: userIds})},
        );
    };

    removeUsersFromGroup = (groupId: string, userIds: string[]) => {
        return this.doFetch<GroupMember[]>(
            `${this.getGroupRoute(groupId)}/members`,
            {method: 'delete', body: JSON.stringify({user_ids: userIds})},
        );
    };

    searchGroups = (params: GroupSearchParams) => {
        return this.doFetch<Group[]>(
            `${this.getGroupsRoute()}${buildQueryString(params)}`,
            {method: 'get'},
        );
    };

    executeAppCall = async (call: AppCallRequest, trackAsSubmit: boolean) => {
        const callCopy: AppCallRequest = {
            ...call,
            context: {
                ...call.context,
                track_as_submit: trackAsSubmit,
                user_agent: 'webapp',
            },
        };
        return this.doFetch<AppCallResponse>(
            `${this.getAppsProxyRoute()}/api/v1/call`,
            {method: 'post', body: JSON.stringify(callCopy)},
        );
    };

    getAppsBindings = async (channelID: string, teamID: string) => {
        const params = {
            channel_id: channelID,
            team_id: teamID,
            user_agent: 'webapp',
        };

        return this.doFetch<AppBinding[]>(
            `${this.getAppsProxyRoute()}/api/v1/bindings${buildQueryString(params)}`,
            {method: 'get'},
        );
    };

    getGroupsAssociatedToTeam = (teamID: string, q = '', page = 0, perPage = PER_PAGE_DEFAULT, filterAllowReference = false) => {
        return this.doFetch<{
            groups: Group[];
            total_group_count: number;
        }>(
            `${this.getBaseRoute()}/teams/${teamID}/groups${buildQueryString({page, per_page: perPage, q, include_member_count: true, filter_allow_reference: filterAllowReference})}`,
            {method: 'get'},
        );
    };

    getGroupsAssociatedToChannel = (channelID: string, q = '', page = 0, perPage = PER_PAGE_DEFAULT, filterAllowReference = false) => {
        return this.doFetch<{
            groups: Group[];
            total_group_count: number;
        }>(
            `${this.getBaseRoute()}/channels/${channelID}/groups${buildQueryString({page, per_page: perPage, q, include_member_count: true, filter_allow_reference: filterAllowReference})}`,
            {method: 'get'},
        );
    };

    getAllGroupsAssociatedToTeam = (teamID: string, filterAllowReference = false, includeMemberCount = false) => {
        return this.doFetch<GroupsWithCount>(
            `${this.getBaseRoute()}/teams/${teamID}/groups${buildQueryString({paginate: false, filter_allow_reference: filterAllowReference, include_member_count: includeMemberCount})}`,
            {method: 'get'},
        );
    };

    getAllGroupsAssociatedToChannelsInTeam = (teamID: string, filterAllowReference = false) => {
        return this.doFetch<{
            groups: RelationOneToOne<Channel, Group>;
        }>(
            `${this.getBaseRoute()}/teams/${teamID}/groups_by_channels${buildQueryString({paginate: false, filter_allow_reference: filterAllowReference})}`,
            {method: 'get'},
        );
    };

    getAllGroupsAssociatedToChannel = (channelID: string, filterAllowReference = false, includeMemberCount = false) => {
        return this.doFetch<GroupsWithCount>(
            `${this.getBaseRoute()}/channels/${channelID}/groups${buildQueryString({paginate: false, filter_allow_reference: filterAllowReference, include_member_count: includeMemberCount})}`,
            {method: 'get'},
        );
    };

    patchGroupSyncable = (groupID: string, syncableID: string, syncableType: string, patch: Partial<SyncablePatch>) => {
        return this.doFetch<GroupSyncable>(
            `${this.getGroupRoute(groupID)}/${syncableType}s/${syncableID}/patch`,
            {method: 'put', body: JSON.stringify(patch)},
        );
    };

    patchGroup = (groupID: string, patch: GroupPatch | CustomGroupPatch) => {
        return this.doFetch<Group>(
            `${this.getGroupRoute(groupID)}/patch`,
            {method: 'put', body: JSON.stringify(patch)},
        );
    };

    archiveGroup = (groupId: string) => {
        return this.doFetch<Group>(
            `${this.getGroupRoute(groupId)}`,
            {method: 'delete'},
        );
    };

    restoreGroup = (groupId: string) => {
        return this.doFetch<Group>(
            `${this.getGroupRoute(groupId)}/restore`,
            {method: 'post'},
        );
    };

    createGroupTeamsAndChannels = (userID: string) => {
        return this.doFetch<Group>(
            `${this.getBaseRoute()}/ldap/users/${userID}/group_sync_memberships`,
            {method: 'post'},
        );
    };

    // Bot Routes

    createBot = (bot: Partial<Bot>) => {
        return this.doFetch<Bot>(
            `${this.getBotsRoute()}`,
            {method: 'post', body: JSON.stringify(bot)},
        );
    };

    patchBot = (botUserId: string, botPatch: Partial<BotPatch>) => {
        return this.doFetch<Bot>(
            `${this.getBotRoute(botUserId)}`,
            {method: 'put', body: JSON.stringify(botPatch)},
        );
    };

    getBot = (botUserId: string) => {
        return this.doFetch<Bot>(
            `${this.getBotRoute(botUserId)}`,
            {method: 'get'},
        );
    };

    getBots = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Bot[]>(
            `${this.getBotsRoute()}${buildQueryString({page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getBotsIncludeDeleted = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Bot[]>(
            `${this.getBotsRoute()}${buildQueryString({include_deleted: true, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    getBotsOrphaned = (page = 0, perPage = PER_PAGE_DEFAULT) => {
        return this.doFetch<Bot[]>(
            `${this.getBotsRoute()}${buildQueryString({only_orphaned: true, page, per_page: perPage})}`,
            {method: 'get'},
        );
    };

    disableBot = (botUserId: string) => {
        return this.doFetch<Bot>(
            `${this.getBotRoute(botUserId)}/disable`,
            {method: 'post'},
        );
    };

    enableBot = (botUserId: string) => {
        return this.doFetch<Bot>(
            `${this.getBotRoute(botUserId)}/enable`,
            {method: 'post'},
        );
    };

    assignBot = (botUserId: string, newOwnerId: string) => {
        return this.doFetch<Bot>(
            `${this.getBotRoute(botUserId)}/assign/${newOwnerId}`,
            {method: 'post'},
        );
    };

    // Cloud routes
    getCloudProducts = (includeLegacyProducts?: boolean) => {
        let query = '';
        if (includeLegacyProducts) {
            query = '?include_legacy=true';
        }
        return this.doFetch<Product[]>(
            `${this.getCloudRoute()}/products${query}`, {method: 'get'},
        );
    };

    cwsAvailabilityCheck = () => {
        return this.doFetch<{status: string}>(
            `${this.getCloudRoute()}/check-cws-connection`,
            {method: 'get'},
        );
    };

    getCloudCustomer = () => {
        return this.doFetch<CloudCustomer>(
            `${this.getCloudRoute()}/customer`, {method: 'get'},
        );
    };

    notifyAdmin = (req: NotifyAdminRequest) => {
        return this.doFetchWithResponse<StatusOK>(
            `${this.getUsersRoute()}/notify-admin`,
            {method: 'post', body: JSON.stringify(req)},
        );
    };

    validateBusinessEmail = (email = '') => {
        return this.doFetchWithResponse<ValidBusinessEmail>(
            `${this.getCloudRoute()}/validate-business-email`,
            {method: 'post', body: JSON.stringify({email})},
        );
    };

    validateWorkspaceBusinessEmail = () => {
        return this.doFetchWithResponse<ValidBusinessEmail>(
            `${this.getCloudRoute()}/validate-workspace-business-email`,
            {method: 'post'},
        );
    };

    getSubscription = () => {
        return this.doFetch<Subscription>(
            `${this.getCloudRoute()}/subscription`,
            {method: 'get'},
        );
    };

    getInstallation = () => {
        return this.doFetch<Installation>(
            `${this.getCloudRoute()}/installation`,
            {method: 'get'},
        );
    };

    getCloudPreviewModalData = () => {
        return this.doFetch<PreviewModalContentData[]>(
            `${this.getCloudRoute()}/preview/modal_data`,
            {method: 'get'},
        );
    };

    getCloudLimits = () => {
        return this.doFetch<Limits>(
            `${this.getCloudRoute()}/limits`,
            {method: 'get'},
        );
    };

    getPostsUsage = () => {
        return this.doFetch<PostsUsageResponse>(
            `${this.getUsageRoute()}/posts`,
            {method: 'get'},
        );
    };

    getFilesUsage = () => {
        return this.doFetch<FilesUsageResponse>(
            `${this.getUsageRoute()}/storage`,
            {method: 'get'},
        );
    };

    getTeamsUsage = () => {
        return this.doFetch<TeamsUsageResponse>(
            `${this.getUsageRoute()}/teams`,
            {method: 'get'},
        );
    };

    teamMembersMinusGroupMembers = (teamID: string, groupIDs: string[], page: number, perPage: number) => {
        const query = `group_ids=${groupIDs.join(',')}&page=${page}&per_page=${perPage}`;
        return this.doFetch<UsersWithGroupsAndCount>(
            `${this.getTeamRoute(teamID)}/members_minus_group_members?${query}`,
            {method: 'get'},
        );
    };

    channelMembersMinusGroupMembers = (channelID: string, groupIDs: string[], page: number, perPage: number) => {
        const query = `group_ids=${groupIDs.join(',')}&page=${page}&per_page=${perPage}`;
        return this.doFetch<UsersWithGroupsAndCount>(
            `${this.getChannelRoute(channelID)}/members_minus_group_members?${query}`,
            {method: 'get'},
        );
    };

    getSamlMetadataFromIdp = (samlMetadataURL: string) => {
        return this.doFetch<SamlMetadataResponse>(
            `${this.getBaseRoute()}/saml/metadatafromidp`, {method: 'post', body: JSON.stringify({saml_metadata_url: samlMetadataURL})},
        );
    };

    setSamlIdpCertificateFromMetadata = (certData: string) => {
        const request: any = {
            method: 'post',
            body: certData,
        };

        request.headers = {
            'Content-Type': 'application/x-pem-file',
        };

        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/saml/certificate/idp`,
            request,
        );
    };

    getInProductNotices = (teamId: string, client: string, clientVersion: string) => {
        return this.doFetch<ProductNotices>(
            `${this.getNoticesRoute()}/${teamId}?client=${client}&clientVersion=${clientVersion}`,
            {method: 'get'},
        );
    };

    updateNoticesAsViewed = (noticeIds: string[]) => {
        // Only one notice is marked as viewed at a time so using 0 index
        return this.doFetch<StatusOK>(
            `${this.getNoticesRoute()}/view`,
            {method: 'put', body: JSON.stringify(noticeIds)},
        );
    };

    getAncillaryPermissions = (subsectionPermissions: string[]) => {
        return this.doFetch<string[]>(
            `${this.getPermissionsRoute()}/ancillary`,
            {method: 'post', body: JSON.stringify(subsectionPermissions)},
        );
    };

    completeSetup = (completeOnboardingRequest: CompleteOnboardingRequest) => {
        return this.doFetch<StatusOK>(
            `${this.getSystemRoute()}/onboarding/complete`,
            {method: 'post', body: JSON.stringify(completeOnboardingRequest)},
        );
    };

    getAppliedSchemaMigrations = () => {
        return this.doFetch<SchemaMigration[]>(
            `${this.getSystemRoute()}/schema/version`,
            {method: 'get'},
        );
    };

    getCallsChannelState = (channelId: string) => {
        return this.doFetch<{enabled: boolean; id: string}>(
            `${this.url}/plugins/${'com.mattermost.calls'}/${channelId}`,
            {method: 'get'},
        );
    };

    getAIRewrittenMessage = (agentId: string, message: string, action?: string, customPrompt?: string, rootId?: string) => {
        const body: {agent_id: string; message: string; action?: string; custom_prompt?: string; root_id?: string} = {
            agent_id: agentId,
            message,
        };
        if (action) {
            body.action = action;
        }
        if (customPrompt) {
            body.custom_prompt = customPrompt;
        }
        if (rootId) {
            body.root_id = rootId;
        }
        return this.doFetch<{rewritten_text: string; changes_made: string[]}>(
            `${this.getPostsRoute()}/rewrite`,
            {method: 'post', body: JSON.stringify(body)},
        ).then((response) => response.rewritten_text);
    };

    // Client Helpers

    protected doFetch = async <ClientDataResponse>(url: string, options: Options): Promise<ClientDataResponse> => {
        const {data} = await this.doFetchWithResponse<ClientDataResponse>(url, options);

        return data;
    };

    private doFetchWithResponse = async <ClientDataResponse>(url: string, options: Options): Promise<ClientResponse<ClientDataResponse>> => {
        const response = await fetch(url, this.getOptions(options));
        const headers = parseAndMergeNestedHeaders(response.headers);

        let data;
        try {
            const contentType = headers.get('Content-Type');
            if (contentType === 'application/json') {
                data = await response.json();
            } else if (contentType === 'application/x-ndjson') {
                const text = await response.text();
                const objects = text.trim().split('\n');
                data = objects.map((obj) => JSON.parse(obj));
            } else if (contentType === 'application/zip' || contentType?.startsWith('text/csv')) {
                data = await response.blob();
            } else {
                data = await response.text();
            }
        } catch (err) {
            throw new ClientError(this.getUrl(), {
                message: 'Received invalid response from the server.',
                url,
            }, err);
        }

        if (headers.has(HEADER_X_VERSION_ID) && !headers.get('Cache-Control')) {
            const serverVersion = headers.get(HEADER_X_VERSION_ID);
            if (serverVersion && this.serverVersion !== serverVersion) {
                this.serverVersion = serverVersion;
            }
        }

        if (headers.has(HEADER_X_CLUSTER_ID)) {
            const clusterId = headers.get(HEADER_X_CLUSTER_ID);
            if (clusterId && this.clusterId !== clusterId) {
                this.clusterId = clusterId;
            }
        }

        if (response.ok || options.ignoreStatus) {
            return {
                response,
                headers,
                data,
            };
        }

        const msg = data.message || '';

        if (this.logToConsole) {
            console.error(msg); // eslint-disable-line no-console
        }

        throw new ClientError(this.getUrl(), {
            message: msg,
            server_error_id: data.id,
            status_code: data.status_code,
            detailed_error: data.detailed_error,
            props: data.props,
            url,
        });
    };

    upsertDraft = async (draft: Draft, connectionId: string) => {
        const result = await this.doFetch<Draft>(
            `${this.getDraftsRoute()}`,
            {
                method: 'post',
                body: JSON.stringify(draft),
                headers: {
                    'Connection-Id': `${connectionId}`,
                },
            },
        );

        return result;
    };

    getUserDrafts = (teamId: Team['id']) => {
        return this.doFetch<Draft[]>(
            `${this.getUserRoute('me')}/teams/${teamId}/drafts`,
            {method: 'get'},
        );
    };

    deleteDraft = (channelId: Channel['id'], rootId = '', connectionId: string) => {
        let endpoint = `${this.getUserRoute('me')}/channels/${channelId}/drafts`;
        if (rootId !== '') {
            endpoint += `/${rootId}`;
        }

        return this.doFetch<null>(
            endpoint,
            {
                method: 'delete',
                headers: {
                    'Connection-Id': `${connectionId}`,
                },
            },
        );
    };

    getIPFilters = () => {
        return this.doFetch<AllowedIPRange[]>(
            `${this.getBaseRoute()}/ip_filtering`,
            {method: 'get'},
        );
    };

    getCurrentIP = () => {
        return this.doFetch<FetchIPResponse>(
            `${this.getBaseRoute()}/ip_filtering/my_ip`,
            {method: 'get'},
        );
    };

    applyIPFilters = (filters: AllowedIPRanges) => {
        return this.doFetch<AllowedIPRange[]>(
            `${this.getBaseRoute()}/ip_filtering`,
            {method: 'post', body: JSON.stringify(filters)},
        );
    };

    getGroupMessageMembersCommonTeams = (channelId: string) => {
        return this.doFetchWithResponse<Team[]>(
            `${this.getChannelRoute(channelId)}/common_teams`,
            {method: 'get'},
        );
    };

    convertGroupMessageToPrivateChannel = (channelId: string, teamId: string, displayName: string, name: string) => {
        const body = {
            channel_id: channelId,
            team_id: teamId,
            display_name: displayName,
            name,
        };
        return this.doFetchWithResponse<Channel>(
            `${this.getChannelRoute(channelId)}/convert_to_channel?team_id=${teamId}`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    // Schedule Post methods
    createScheduledPost = (schedulePost: PartialExcept<ScheduledPost, 'channel_id' | 'message' | 'scheduled_at'>, connectionId: string) => {
        return this.doFetchWithResponse<ScheduledPost>(
            `${this.getPostsRoute()}/schedule`,
            {method: 'post', body: JSON.stringify(schedulePost), headers: {'Connection-Id': connectionId}},
        );
    };

    // get user's current team's scheduled posts
    getScheduledPosts = (teamId: string, includeDirectChannels: boolean) => {
        return this.doFetchWithResponse<{[key: string]: ScheduledPost[]}>(
            `${this.getPostsRoute()}/scheduled/team/${teamId}?includeDirectChannels=${includeDirectChannels}`,
            {method: 'get'},
        );
    };

    updateScheduledPost = (schedulePost: ScheduledPost, connectionId: string) => {
        return this.doFetchWithResponse<ScheduledPost>(
            `${this.getPostsRoute()}/schedule/${schedulePost.id}`,
            {method: 'put', body: JSON.stringify(schedulePost), headers: {'Connection-Id': connectionId}},
        );
    };

    deleteScheduledPost = (userId: string, schedulePostId: string, connectionId: string) => {
        return this.doFetchWithResponse<ScheduledPost>(
            `${this.getPostsRoute()}/schedule/${schedulePostId}`,
            {method: 'delete', headers: {'Connection-Id': connectionId}},
        );
    };

    restorePostVersion = (postId: string, restoreVersionId: string, connectionId: string) => {
        return this.doFetchWithResponse<Post>(
            `${this.getPostRoute(postId)}/restore/${restoreVersionId}`,
            {method: 'post', headers: {'Connection-Id': connectionId}},
        );
    };

    getAccessControlPolicy = (id: string, channelId?: string, teamId?: string) => {
        let url = `${this.getBaseRoute()}/access_control_policies/${id}`;
        const params = new URLSearchParams();
        if (channelId) {
            params.set('channelId', channelId);
        }
        if (teamId) {
            params.set('team_id', teamId);
        }
        const qs = params.toString();
        if (qs) {
            url += `?${qs}`;
        }
        return this.doFetch<AccessControlPolicy>(
            url,
            {method: 'get'},
        );
    };

    updateOrCreateAccessControlPolicy = (policy: AccessControlPolicy, teamId?: string) => {
        const teamParam = teamId ? `?team_id=${encodeURIComponent(teamId)}` : '';
        return this.doFetch<AccessControlPolicy>(
            `${this.getBaseRoute()}/access_control_policies${teamParam}`,
            {method: 'put', body: JSON.stringify(policy)},
        );
    };

    deleteAccessControlPolicy = (id: string, teamId?: string) => {
        const teamParam = teamId ? `?team_id=${encodeURIComponent(teamId)}` : '';
        return this.doFetch<AccessControlPolicy>(
            `${this.getBaseRoute()}/access_control_policies/${id}${teamParam}`,
            {method: 'delete'},
        );
    };

    getAccessControlPolicies = (after: string, limit: number) => {
        return this.doFetch<AccessControlPoliciesResult>(
            `${this.getBaseRoute()}/access_control_policies/search`,
            {method: 'post', body: JSON.stringify({type: 'parent', cursor: {id: after}, limit, actions: ['membership']})},
        );
    };

    getChildPolicies = (parentId: string, after: string, limit: number) => {
        return this.doFetch<AccessControlPoliciesResult>(
            `${this.getBaseRoute()}/access_control_policies/search`,
            {method: 'post', body: JSON.stringify({parent_id: parentId, cursor: {id: after}, limit})},
        );
    };

    getChannelsForAccessControlPolicy = (policyId: string, after: string, limit: number, teamId?: string) => {
        const teamParam = teamId ? `&team_id=${encodeURIComponent(teamId)}` : '';
        return this.doFetch<AccessControlPolicyChannelsResult>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/resources/channels?after=${after}&limit=${limit}${teamParam}`,
            {method: 'get'},
        );
    };

    searchAccessControlPolicies = (term: string, type: string, after: string, limit: number, teamId?: string) => {
        return this.doFetch<AccessControlPoliciesResult>(
            `${this.getBaseRoute()}/access_control_policies/search`,
            {method: 'post', body: JSON.stringify({term, type, cursor: {id: after}, limit, include_children: true, ...(teamId && {team_id: teamId})})},
        );
    };

    searchPermissionPolicies = (term: string, after: string, limit: number) => {
        return this.doFetch<AccessControlPoliciesResult>(
            `${this.getBaseRoute()}/access_control_policies/search`,
            {method: 'post', body: JSON.stringify({term, type: 'permission', cursor: {id: after}, limit})},
        );
    };

    searchAccessControlDecisionActions = (resourceType: string, resourceId: string, actions?: string[]) => {
        const body: ActionSearchRequest = {resource: {type: resourceType, id: resourceId}};
        if (actions && actions.length > 0) {
            body.actions = actions;
        }
        return this.doFetch<ActionSearchResponse>(
            `${this.getBaseRoute()}/access_control/decisions/actions/search`,
            {method: 'post', body: JSON.stringify(body)},
        );
    };

    searchChildAccessControlPolicyChannels = (policyId: string, term: string, opts: ChannelSearchOpts, teamId?: string) => {
        const teamParam = teamId ? `?team_id=${encodeURIComponent(teamId)}` : '';
        return this.doFetch<ChannelsWithTotalCount>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/resources/channels/search${teamParam}`,
            {method: 'post', body: JSON.stringify({term, ...opts})},
        );
    };

    assignChannelsToAccessControlPolicy = (policyId: string, channelIds: string[], teamId?: string) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/assign`,
            {method: 'post', body: JSON.stringify({channel_ids: channelIds, ...(teamId && {team_id: teamId})})},
        );
    };

    unassignChannelsFromAccessControlPolicy = (policyId: string, channelIds: string[], teamId?: string) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/unassign`,
            {method: 'delete', body: JSON.stringify({channel_ids: channelIds, ...(teamId && {team_id: teamId})})},
        );
    };

    assignTeamsToAccessControlPolicy = (policyId: string, teamIds: string[]) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/assign`,
            {method: 'post', body: JSON.stringify({team_ids: teamIds})},
        );
    };

    unassignTeamsFromAccessControlPolicy = (policyId: string, teamIds: string[]) => {
        return this.doFetch<StatusOK>(
            `${this.getBaseRoute()}/access_control_policies/${policyId}/unassign`,
            {method: 'delete', body: JSON.stringify({team_ids: teamIds})},
        );
    };

    getTeamAccessControlPolicy = (teamId: string) => {
        return this.doFetch<{policy: AccessControlPolicy | null; enforced: boolean; parent_policies?: AccessControlPolicy[]}>(
            `${this.getTeamRoute(teamId)}/access_control/policy`,
            {method: 'get'},
        );
    };

    getTeamAccessControlAttributes = (teamId: string) => {
        return this.doFetch<AccessControlAttributes>(
            `${this.getTeamRoute(teamId)}/access_control/attributes`,
            {method: 'get'},
        );
    };

    // getProfilesMatchingTeamPolicy returns only users who satisfy the team's
    // ABAC membership policy and are not yet members, for the policy-filtered
    // invite candidate list.
    getProfilesMatchingTeamPolicy = (teamId: string, perPage = PER_PAGE_DEFAULT, cursorId = '', term = '') => {
        const queryStringObj: any = {
            not_in_team: teamId,
            per_page: perPage,
            abac_match_only: true,
        };
        if (cursorId) {
            queryStringObj.cursor_id = cursorId;
        }
        if (term) {
            queryStringObj.term = term;
        }

        return this.doFetch<UserProfile[]>(
            `${this.getUsersRoute()}${buildQueryString(queryStringObj)}`,
            {method: 'get'},
        );
    };

    createAccessControlSyncJob = (jobData: {[key: string]: string}) => {
        const job = {
            type: 'access_control_sync' as JobType,
            data: jobData,
        };
        return this.createJob(job);
    };

    createAccessControlTeamSyncJob = (jobData: {[key: string]: string}) => {
        const job = {
            type: 'access_control_team_sync' as JobType,
            data: jobData,
        };
        return this.createJob(job);
    };

    getAccessControlFields = (after: string, limit: number, channelId?: string, teamId?: string, includeResourceFields?: boolean) => {
        const params = new URLSearchParams({after, limit: limit.toString()});
        if (channelId) {
            params.append('channelId', channelId);
        }
        if (teamId) {
            params.append('team_id', teamId);
        }

        // Parent policies reference resource.attributes.* (channel-object-type
        // fields) without a single channel to scope by; ask for them explicitly.
        if (includeResourceFields) {
            params.append('include_resource_fields', 'true');
        }

        return this.doFetch<UserPropertyField[]>(
            `${this.getBaseRoute()}/access_control_policies/cel/autocomplete/fields?${params.toString()}`,
            {method: 'get'},
        );
    };

    // membership also rejects what the membership sync can't evaluate.
    checkAccessControlExpression = (expression: string, channelId?: string, teamId?: string, membership?: boolean) => {
        const requestBody: {expression: string; channelId?: string; teamId?: string; membership?: boolean} = {expression};
        if (channelId) {
            requestBody.channelId = channelId;
        }
        if (teamId) {
            requestBody.teamId = teamId;
        }
        if (membership) {
            requestBody.membership = true;
        }

        return this.doFetch<CELExpressionError[]>(
            `${this.getBaseRoute()}/access_control_policies/cel/check`,
            {method: 'post', body: JSON.stringify(requestBody)},
        );
    };

    testAccessControlExpression = (expression: string, term: string, after: string, limit: number, channelId?: string, teamId?: string) => {
        const requestBody: {expression: string; term: string; after: string; limit: number; channelId?: string; teamId?: string} = {
            expression, term, after, limit,
        };
        if (channelId) {
            requestBody.channelId = channelId;
        }
        if (teamId) {
            requestBody.teamId = teamId;
        }

        return this.doFetch<AccessControlTestResult>(
            `${this.getBaseRoute()}/access_control_policies/cel/test`,
            {method: 'post', body: JSON.stringify(requestBody)},
        );
    };

    /**
     * Simulate the dual-lane PDP decision for a draft (unsaved) policy
     * against an explicit set of users. The server compiles the draft
     * in-memory, layers on persisted higher-scoped permission policies,
     * and returns per-user, per-action ALLOW/DENY decisions plus blame
     * attribution. Backs the picker-based "Simulate access" modal in
     * the System Console and Channel Settings.
     */
    simulateAccessControlPolicyForUsers = (params: PolicySimulationByUsersParams) => {
        return this.doFetch<PolicySimulationResponse>(
            `${this.getBaseRoute()}/access_control_policies/cel/simulate_users`,
            {method: 'post', body: JSON.stringify(params)},
        );
    };

    expressionToVisualFormat = (expression: string, channelId?: string, teamId?: string) => {
        const requestBody: {expression: string; channelId?: string; teamId?: string} = {expression};
        if (channelId) {
            requestBody.channelId = channelId;
        }
        if (teamId) {
            requestBody.teamId = teamId;
        }

        return this.doFetch<AccessControlVisualAST>(
            `${this.getBaseRoute()}/access_control_policies/cel/visual_ast`,
            {method: 'post', body: JSON.stringify(requestBody)},
        );
    };

    validateExpressionAgainstRequester = (expression: string, channelId?: string, teamId?: string) => {
        const requestBody: {expression: string; channelId?: string; teamId?: string} = {expression};
        if (channelId !== undefined) {
            requestBody.channelId = channelId;
        }
        if (teamId) {
            requestBody.teamId = teamId;
        }

        return this.doFetch<{requester_matches: boolean}>(
            `${this.getBaseRoute()}/access_control_policies/cel/validate_requester`,
            {method: 'post', body: JSON.stringify(requestBody)},
        );
    };

    getChannelAccessControlAttributes = (channelId: string) => {
        return this.doFetch<AccessControlAttributes>(
            `${this.getChannelRoute(channelId)}/access_control/attributes`,
            {method: 'get'},
        );
    };

    updateAccessControlPoliciesActive = (states: AccessControlPolicyActiveUpdate[], teamId?: string) => {
        const body: Record<string, unknown> = {entries: states};
        if (teamId) {
            body.team_id = teamId;
        }
        return this.doFetch<AccessControlPolicy[]>(
            `${this.getBaseRoute()}/access_control_policies/activate`,
            {method: 'put', body: JSON.stringify(body)},
        );
    };

    getTeamContentFlaggingStatus = (teamId: string) => {
        return this.doFetch<{enabled: boolean}>(
            `${this.getContentFlaggingRoute()}/team/${teamId}/status`,
            {method: 'get'},
        );
    };

    getContentFlaggingConfig = (teamId?: string) => {
        return this.doFetch<ContentFlaggingConfig>(
            `${this.getContentFlaggingRoute()}/flag/config${buildQueryString({team_id: teamId})}`,
            {method: 'get'},
        );
    };

    flagPost = (postId: string, reason: string, comment?: string) => {
        return this.doFetch<StatusOK>(
            `${this.getContentFlaggingRoute()}/post/${postId}/flag`,
            {
                method: 'post',
                body: JSON.stringify({reason, comment}),
            },
        );
    };

    removeFlaggedPost = (postId: string, comment?: string) => {
        return this.doFetch<StatusOK>(
            `${this.getContentFlaggingRoute()}/post/${postId}/remove`,
            {
                method: 'put',
                body: JSON.stringify({comment}),
            },
        );
    };

    keepFlaggedPost = (postId: string, comment?: string) => {
        return this.doFetch<StatusOK>(
            `${this.getContentFlaggingRoute()}/post/${postId}/keep`,
            {
                method: 'put',
                body: JSON.stringify({comment}),
            },
        );
    };

    getPostContentFlaggingFields = () => {
        return this.doFetch<NameMappedPropertyFields>(
            `${this.getContentFlaggingRoute()}/fields`,
            {method: 'get'},
        );
    };

    getPostContentFlaggingValues = (postId: string) => {
        return this.doFetch<Array<PropertyValue<unknown>>>(
            `${this.getContentFlaggingRoute()}/post/${postId}/field_values`,
            {method: 'get'},
        );
    };

    getFlaggedPost = (postId: string) => {
        return this.doFetch<Post>(
            `${this.getContentFlaggingRoute()}/post/${postId}`,
            {method: 'get'},
        );
    };

    searchContentFlaggingReviewers = (term: string, teamId: string) => {
        return this.doFetch<UserProfile[]>(
            `${this.getContentFlaggingRoute()}/team/${teamId}/reviewers/search${buildQueryString({term})}`,
            {method: 'get'},
        );
    };

    setContentFlaggingReviewer = (postId: string, reviewerId: string) => {
        return this.doFetch(
            `${this.getContentFlaggingRoute()}/post/${postId}/assign/${reviewerId}`,
            {method: 'post'},
        );
    };

    saveContentFlaggingConfig = (config: ContentFlaggingSettings) => {
        return this.doFetch(
            `${this.getContentFlaggingRoute()}/config`,
            {method: 'put', body: JSON.stringify(config)},
        );
    };

    getAdminContentFlaggingConfig = () => {
        return this.doFetch<ContentFlaggingSettings>(
            `${this.getContentFlaggingRoute()}/config`,
            {method: 'get'},
        );
    };

    getDeliveryTrackingConfig = () => {
        return this.doFetch<DeliveryTrackingConfig>(
            `${this.getDeliveryTrackingRoute()}/config`,
            {method: 'get'},
        );
    };

    saveDeliveryTrackingConfig = (config: DeliveryTrackingConfig) => {
        return this.doFetch<StatusOK>(
            `${this.getDeliveryTrackingRoute()}/config`,
            {method: 'put', body: JSON.stringify(config)},
        );
    };

    getHealthFindings = (filter: HealthFindingFilter = {}) => {
        return this.doFetch<HealthFindingList>(
            `${this.getHealthFindingsRoute()}${buildQueryString(filter)}`,
            {method: 'get'},
        );
    };

    muteHealthFinding = (fingerprint: string) => {
        return this.doFetch<StatusOK>(
            `${this.getHealthFindingsRoute()}/${fingerprint}/mute`,
            {method: 'post'},
        );
    };

    unmuteHealthFinding = (fingerprint: string) => {
        return this.doFetch<StatusOK>(
            `${this.getHealthFindingsRoute()}/${fingerprint}/mute`,
            {method: 'delete'},
        );
    };

    getFlaggedPostReportUrl = (postId: string) => {
        return `${this.getContentFlaggingRoute()}/post/${postId}/report`;
    };

    generateFlaggedPostReport = (postId: string, comment: string, action?: 'keep' | 'remove', signal?: AbortSignal): Promise<Blob> => {
        return this.doFetch<Blob>(
            this.getFlaggedPostReportUrl(postId),
            {
                method: 'post',
                body: JSON.stringify({comment, action}),
                signal,
            },
        );
    };

    getPostExposureReportUrl = (postId: string) => {
        return `${this.getContentFlaggingRoute()}/post/${postId}/exposure_report`;
    };

    generatePostExposureReport = async (postId: string, signal?: AbortSignal): Promise<{blob: Blob; filename: string}> => {
        const {data, headers} = await this.doFetchWithResponse<Blob>(
            this.getPostExposureReportUrl(postId),
            {
                method: 'post',
                signal,
            },
        );

        return {
            blob: data,
            filename: extractFilenameFromContentDisposition(
                headers.get('Content-Disposition'),
                `post-exposure-${postId}-${Date.now()}.csv`,
            ),
        };
    };
}

export function parseAndMergeNestedHeaders(originalHeaders: any) {
    const headers = new Map();
    let nestedHeaders = new Map();
    originalHeaders.forEach((val: string, key: string) => {
        const capitalizedKey = key.replace(/\b[a-z]/g, (l) => l.toUpperCase());
        let realVal = val;
        if (val && val.match(/\n\S+:\s\S+/)) {
            const nestedHeaderStrings = val.split('\n');
            realVal = nestedHeaderStrings.shift() as string;
            const moreNestedHeaders = new Map(
                nestedHeaderStrings.map((h: any) => h.split(/:\s/)),
            );
            nestedHeaders = new Map([...nestedHeaders, ...moreNestedHeaders]);
        }
        headers.set(capitalizedKey, realVal);
    });
    return new Map([...headers, ...nestedHeaders]);
}

export class ClientError extends Error implements ServerError {
    url?: string;
    server_error_id?: string;
    status_code?: number;
    detailed_error?: string;
    props?: Record<string, string>;

    constructor(baseUrl: string, data: ServerError, cause?: any) {
        super(data.message + ': ' + cleanUrlForLogging(baseUrl, data.url || ''), {cause});

        this.message = data.message;
        this.url = data.url;
        this.server_error_id = data.server_error_id;
        this.status_code = data.status_code;
        this.detailed_error = data.detailed_error;
        this.props = data.props;

        // Ensure message is treated as a property of this class when object spreading. Without this,
        // copying the object by using `{...error}` would not include the message.
        Object.defineProperty(this, 'message', {enumerable: true});
    }
}
