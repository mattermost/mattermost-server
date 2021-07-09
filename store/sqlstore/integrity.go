// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/Masterminds/squirrel"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type relationalCheckConfig struct {
	parentName         string
	parentIDAttr       string
	childName          string
	childIDAttr        string
	canParentIDBeEmpty bool
	sortRecords        bool
	filter             interface{}
}

func getOrphanedRecords(ss *SqlStore, cfg relationalCheckConfig) ([]model.OrphanedRecord, error) {
	var records []model.OrphanedRecord

	sub := ss.getQueryBuilder().
		Select("TRUE").
		From(cfg.parentName + " AS PT").
		Prefix("NOT EXISTS (").
		Suffix(")").
		Where("PT.id = CT." + cfg.parentIDAttr)

	main := ss.getQueryBuilder().
		Select().
		Column("CT." + cfg.parentIDAttr + " AS ParentId").
		From(cfg.childName + " AS CT").
		Where(sub)

	if cfg.childIDAttr != "" {
		main = main.Column("CT." + cfg.childIDAttr + " AS ChildId")
	}

	if cfg.canParentIDBeEmpty {
		main = main.Where(sq.NotEq{"CT." + cfg.parentIDAttr: ""})
	}

	if cfg.filter != nil {
		main = main.Where(cfg.filter)
	}

	if cfg.sortRecords {
		main = main.OrderBy("CT." + cfg.parentIDAttr)
	}

	query, args, _ := main.ToSql()

	_, err := ss.GetMaster().Select(&records, query, args...)

	return records, err
}

func checkParentChildIntegrity(ss *SqlStore, config relationalCheckConfig) model.IntegrityCheckResult {
	var result model.IntegrityCheckResult
	var data model.RelationalIntegrityCheckData

	config.sortRecords = true
	data.Records, result.Err = getOrphanedRecords(ss, config)
	if result.Err != nil {
		mlog.Error("Error while getting orphaned records", mlog.Err(result.Err))
		return result
	}
	data.ParentName = config.parentName
	data.ChildName = config.childName
	data.ParentIDAttr = config.parentIDAttr
	data.ChildIDAttr = config.childIDAttr
	result.Data = data

	return result
}

func checkChannelsCommandWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsChannelMemberHistoryIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "ChannelMemberHistory",
		childIDAttr:  "",
	})
}

func checkChannelsChannelMembersIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "ChannelMembers",
		childIDAttr:  "",
	})
}

func checkChannelsIncomingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsOutgoingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsPostsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "Posts",
		childIDAttr:  "Id",
	})
}

func checkCommandsCommandWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Commands",
		parentIDAttr: "CommandId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkPostsFileInfoIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Posts",
		parentIDAttr: "PostId",
		childName:    "FileInfo",
		childIDAttr:  "Id",
	})
}

func checkPostsPostsParentIDIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Posts",
		parentIDAttr:       "ParentId",
		childName:          "Posts",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkPostsPostsRootIDIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Posts",
		parentIDAttr:       "RootId",
		childName:          "Posts",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkPostsReactionsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Posts",
		parentIDAttr: "PostId",
		childName:    "Reactions",
		childIDAttr:  "",
	})
}

func checkSchemesChannelsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Schemes",
		parentIDAttr:       "SchemeId",
		childName:          "Channels",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkSchemesTeamsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Schemes",
		parentIDAttr:       "SchemeId",
		childName:          "Teams",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkSessionsAuditsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Sessions",
		parentIDAttr:       "SessionId",
		childName:          "Audits",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkTeamsChannelsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	res1 := checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "Channels",
		childIDAttr:  "Id",
		filter:       sq.NotEq{"CT.Type": []string{model.ChannelTypeDirect, model.ChannelTypeGroup}},
	})
	res2 := checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Teams",
		parentIDAttr:       "TeamId",
		childName:          "Channels",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
		filter:             sq.Eq{"CT.Type": []string{model.ChannelTypeDirect, model.ChannelTypeGroup}},
	})
	data1 := res1.Data.(model.RelationalIntegrityCheckData)
	data2 := res2.Data.(model.RelationalIntegrityCheckData)
	data1.Records = append(data1.Records, data2.Records...)
	res1.Data = data1
	return res1
}

func checkTeamsCommandsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "Commands",
		childIDAttr:  "Id",
	})
}

func checkTeamsIncomingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkTeamsOutgoingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkTeamsTeamMembersIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "TeamMembers",
		childIDAttr:  "",
	})
}

func checkUsersAuditsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Users",
		parentIDAttr:       "UserId",
		childName:          "Audits",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkUsersCommandWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersChannelMemberHistoryIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "ChannelMemberHistory",
		childIDAttr:  "",
	})
}

func checkUsersChannelMembersIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "ChannelMembers",
		childIDAttr:  "",
	})
}

func checkUsersChannelsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Users",
		parentIDAttr:       "CreatorId",
		childName:          "Channels",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkUsersCommandsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "Commands",
		childIDAttr:  "Id",
	})
}

func checkUsersCompliancesIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Compliances",
		childIDAttr:  "Id",
	})
}

func checkUsersEmojiIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "Emoji",
		childIDAttr:  "Id",
	})
}

func checkUsersFileInfoIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "FileInfo",
		childIDAttr:  "Id",
	})
}

func checkUsersIncomingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersOAuthAccessDataIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "OAuthAccessData",
		childIDAttr:  "Token",
	})
}

func checkUsersOAuthAppsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "OAuthApps",
		childIDAttr:  "Id",
	})
}

func checkUsersOAuthAuthDataIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "OAuthAuthData",
		childIDAttr:  "Code",
	})
}

func checkUsersOutgoingWebhooksIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersPostsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Posts",
		childIDAttr:  "Id",
	})
}

func checkUsersPreferencesIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Preferences",
		childIDAttr:  "",
	})
}

func checkUsersReactionsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Reactions",
		childIDAttr:  "",
	})
}

func checkUsersSessionsIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Sessions",
		childIDAttr:  "Id",
	})
}

func checkUsersStatusIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Status",
		childIDAttr:  "",
	})
}

func checkUsersTeamMembersIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "TeamMembers",
		childIDAttr:  "",
	})
}

func checkUsersUserAccessTokensIntegrity(ss *SqlStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "UserAccessTokens",
		childIDAttr:  "Id",
	})
}

func checkChannelsIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkChannelsCommandWebhooksIntegrity(ss)
	results <- checkChannelsChannelMemberHistoryIntegrity(ss)
	results <- checkChannelsChannelMembersIntegrity(ss)
	results <- checkChannelsIncomingWebhooksIntegrity(ss)
	results <- checkChannelsOutgoingWebhooksIntegrity(ss)
	results <- checkChannelsPostsIntegrity(ss)
}

func checkCommandsIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkCommandsCommandWebhooksIntegrity(ss)
}

func checkPostsIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkPostsFileInfoIntegrity(ss)
	results <- checkPostsPostsParentIDIntegrity(ss)
	results <- checkPostsPostsRootIDIntegrity(ss)
	results <- checkPostsReactionsIntegrity(ss)
}

func checkSchemesIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkSchemesChannelsIntegrity(ss)
	results <- checkSchemesTeamsIntegrity(ss)
}

func checkSessionsIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkSessionsAuditsIntegrity(ss)
}

func checkTeamsIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkTeamsChannelsIntegrity(ss)
	results <- checkTeamsCommandsIntegrity(ss)
	results <- checkTeamsIncomingWebhooksIntegrity(ss)
	results <- checkTeamsOutgoingWebhooksIntegrity(ss)
	results <- checkTeamsTeamMembersIntegrity(ss)
}

func checkUsersIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	results <- checkUsersAuditsIntegrity(ss)
	results <- checkUsersCommandWebhooksIntegrity(ss)
	results <- checkUsersChannelMemberHistoryIntegrity(ss)
	results <- checkUsersChannelMembersIntegrity(ss)
	results <- checkUsersChannelsIntegrity(ss)
	results <- checkUsersCommandsIntegrity(ss)
	results <- checkUsersCompliancesIntegrity(ss)
	results <- checkUsersEmojiIntegrity(ss)
	results <- checkUsersFileInfoIntegrity(ss)
	results <- checkUsersIncomingWebhooksIntegrity(ss)
	results <- checkUsersOAuthAccessDataIntegrity(ss)
	results <- checkUsersOAuthAppsIntegrity(ss)
	results <- checkUsersOAuthAuthDataIntegrity(ss)
	results <- checkUsersOutgoingWebhooksIntegrity(ss)
	results <- checkUsersPostsIntegrity(ss)
	results <- checkUsersPreferencesIntegrity(ss)
	results <- checkUsersReactionsIntegrity(ss)
	results <- checkUsersSessionsIntegrity(ss)
	results <- checkUsersStatusIntegrity(ss)
	results <- checkUsersTeamMembersIntegrity(ss)
	results <- checkUsersUserAccessTokensIntegrity(ss)
}

func CheckRelationalIntegrity(ss *SqlStore, results chan<- model.IntegrityCheckResult) {
	mlog.Info("Starting relational integrity checks...")
	checkChannelsIntegrity(ss, results)
	checkCommandsIntegrity(ss, results)
	checkPostsIntegrity(ss, results)
	checkSchemesIntegrity(ss, results)
	checkSessionsIntegrity(ss, results)
	checkTeamsIntegrity(ss, results)
	checkUsersIntegrity(ss, results)
	mlog.Info("Done with relational integrity checks")
	close(results)
}
