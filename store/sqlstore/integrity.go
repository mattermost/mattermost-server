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

func getOrphanedRecords(ss *SQLStore, cfg relationalCheckConfig) ([]model.OrphanedRecord, error) {
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

func checkParentChildIntegrity(ss *SQLStore, config relationalCheckConfig) model.IntegrityCheckResult {
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

func checkChannelsCommandWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsChannelMemberHistoryIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "ChannelMemberHistory",
		childIDAttr:  "",
	})
}

func checkChannelsChannelMembersIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "ChannelMembers",
		childIDAttr:  "",
	})
}

func checkChannelsIncomingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsOutgoingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkChannelsPostsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Channels",
		parentIDAttr: "ChannelId",
		childName:    "Posts",
		childIDAttr:  "Id",
	})
}

func checkCommandsCommandWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Commands",
		parentIDAttr: "CommandId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkPostsFileInfoIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Posts",
		parentIDAttr: "PostId",
		childName:    "FileInfo",
		childIDAttr:  "Id",
	})
}

func checkPostsPostsParentIDIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Posts",
		parentIDAttr:       "ParentId",
		childName:          "Posts",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkPostsPostsRootIDIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Posts",
		parentIDAttr:       "RootId",
		childName:          "Posts",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkPostsReactionsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Posts",
		parentIDAttr: "PostId",
		childName:    "Reactions",
		childIDAttr:  "",
	})
}

func checkSchemesChannelsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Schemes",
		parentIDAttr:       "SchemeId",
		childName:          "Channels",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkSchemesTeamsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Schemes",
		parentIDAttr:       "SchemeId",
		childName:          "Teams",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkSessionsAuditsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Sessions",
		parentIDAttr:       "SessionId",
		childName:          "Audits",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkTeamsChannelsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
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

func checkTeamsCommandsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "Commands",
		childIDAttr:  "Id",
	})
}

func checkTeamsIncomingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkTeamsOutgoingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkTeamsTeamMembersIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Teams",
		parentIDAttr: "TeamId",
		childName:    "TeamMembers",
		childIDAttr:  "",
	})
}

func checkUsersAuditsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Users",
		parentIDAttr:       "UserId",
		childName:          "Audits",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkUsersCommandWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "CommandWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersChannelMemberHistoryIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "ChannelMemberHistory",
		childIDAttr:  "",
	})
}

func checkUsersChannelMembersIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "ChannelMembers",
		childIDAttr:  "",
	})
}

func checkUsersChannelsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:         "Users",
		parentIDAttr:       "CreatorId",
		childName:          "Channels",
		childIDAttr:        "Id",
		canParentIDBeEmpty: true,
	})
}

func checkUsersCommandsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "Commands",
		childIDAttr:  "Id",
	})
}

func checkUsersCompliancesIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Compliances",
		childIDAttr:  "Id",
	})
}

func checkUsersEmojiIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "Emoji",
		childIDAttr:  "Id",
	})
}

func checkUsersFileInfoIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "FileInfo",
		childIDAttr:  "Id",
	})
}

func checkUsersIncomingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "IncomingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersOAuthAccessDataIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "OAuthAccessData",
		childIDAttr:  "Token",
	})
}

func checkUsersOAuthAppsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "OAuthApps",
		childIDAttr:  "Id",
	})
}

func checkUsersOAuthAuthDataIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "OAuthAuthData",
		childIDAttr:  "Code",
	})
}

func checkUsersOutgoingWebhooksIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "CreatorId",
		childName:    "OutgoingWebhooks",
		childIDAttr:  "Id",
	})
}

func checkUsersPostsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Posts",
		childIDAttr:  "Id",
	})
}

func checkUsersPreferencesIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Preferences",
		childIDAttr:  "",
	})
}

func checkUsersReactionsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Reactions",
		childIDAttr:  "",
	})
}

func checkUsersSessionsIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Sessions",
		childIDAttr:  "Id",
	})
}

func checkUsersStatusIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "Status",
		childIDAttr:  "",
	})
}

func checkUsersTeamMembersIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "TeamMembers",
		childIDAttr:  "",
	})
}

func checkUsersUserAccessTokensIntegrity(ss *SQLStore) model.IntegrityCheckResult {
	return checkParentChildIntegrity(ss, relationalCheckConfig{
		parentName:   "Users",
		parentIDAttr: "UserId",
		childName:    "UserAccessTokens",
		childIDAttr:  "Id",
	})
}

func checkChannelsIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkChannelsCommandWebhooksIntegrity(ss)
	results <- checkChannelsChannelMemberHistoryIntegrity(ss)
	results <- checkChannelsChannelMembersIntegrity(ss)
	results <- checkChannelsIncomingWebhooksIntegrity(ss)
	results <- checkChannelsOutgoingWebhooksIntegrity(ss)
	results <- checkChannelsPostsIntegrity(ss)
}

func checkCommandsIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkCommandsCommandWebhooksIntegrity(ss)
}

func checkPostsIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkPostsFileInfoIntegrity(ss)
	results <- checkPostsPostsParentIDIntegrity(ss)
	results <- checkPostsPostsRootIDIntegrity(ss)
	results <- checkPostsReactionsIntegrity(ss)
}

func checkSchemesIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkSchemesChannelsIntegrity(ss)
	results <- checkSchemesTeamsIntegrity(ss)
}

func checkSessionsIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkSessionsAuditsIntegrity(ss)
}

func checkTeamsIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
	results <- checkTeamsChannelsIntegrity(ss)
	results <- checkTeamsCommandsIntegrity(ss)
	results <- checkTeamsIncomingWebhooksIntegrity(ss)
	results <- checkTeamsOutgoingWebhooksIntegrity(ss)
	results <- checkTeamsTeamMembersIntegrity(ss)
}

func checkUsersIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
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

func CheckRelationalIntegrity(ss *SQLStore, results chan<- model.IntegrityCheckResult) {
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
