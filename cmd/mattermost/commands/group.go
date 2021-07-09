// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

var GroupCmd = &cobra.Command{
	Use:   "group",
	Short: "Management of groups",
}

var ChannelGroupCmd = &cobra.Command{
	Use:   "channel",
	Short: "Management of channel groups",
}

var ChannelGroupEnableCmd = &cobra.Command{
	Use:     "enable [team]:[channel]",
	Short:   "Enables group constraint on the specified channel",
	Example: "  group channel enable myteam:mychannel",
	Args:    cobra.ExactArgs(1),
	RunE:    channelGroupEnableCmdF,
}

var ChannelGroupDisableCmd = &cobra.Command{
	Use:     "disable [team]:[channel]",
	Short:   "Disables group constraint on the specified channel",
	Example: "  group channel disable myteam:mychannel",
	Args:    cobra.ExactArgs(1),
	RunE:    channelGroupDisableCmdF,
}

var ChannelGroupStatusCmd = &cobra.Command{
	Use:     "status [team]:[channel]",
	Short:   "Shows the group constraint status of the specified channel",
	Example: "  group channel status myteam:mychannel",
	Args:    cobra.ExactArgs(1),
	RunE:    channelGroupStatusCmdF,
}

var ChannelGroupListCmd = &cobra.Command{
	Use:     "list [team]:[channel]",
	Short:   "List channel groups",
	Long:    "Lists the groups associated with a channel",
	Example: "  group channel list myteam:mychannel",
	Args:    cobra.ExactArgs(1),
	RunE:    channelGroupListCmdF,
}

var TeamGroupCmd = &cobra.Command{
	Use:   "team",
	Short: "Management of team groups",
}

var TeamGroupEnableCmd = &cobra.Command{
	Use:     "enable [team]",
	Short:   "Enables group constraint on the specified team",
	Example: "  group team enable myteam",
	Args:    cobra.ExactArgs(1),
	RunE:    teamGroupEnableCmdF,
}

var TeamGroupDisableCmd = &cobra.Command{
	Use:     "disable [team]",
	Short:   "Disables group constraint on the specified team",
	Example: "  group team disable myteam",
	Args:    cobra.ExactArgs(1),
	RunE:    teamGroupDisableCmdF,
}

var TeamGroupStatusCmd = &cobra.Command{
	Use:     "status [team]",
	Short:   "Shows the group constraint status of the specified team",
	Example: "  group team status myteam",
	Args:    cobra.ExactArgs(1),
	RunE:    teamGroupStatusCmdF,
}

var TeamGroupListCmd = &cobra.Command{
	Use:     "list [team]",
	Short:   "List team groups",
	Long:    "Lists the groups associated with a team",
	Example: "  group team list myteam",
	Args:    cobra.ExactArgs(1),
	RunE:    teamGroupListCmdF,
}

func init() {
	ChannelGroupCmd.AddCommand(
		ChannelGroupEnableCmd,
		ChannelGroupDisableCmd,
		ChannelGroupStatusCmd,
		ChannelGroupListCmd,
	)

	TeamGroupCmd.AddCommand(
		TeamGroupEnableCmd,
		TeamGroupDisableCmd,
		TeamGroupStatusCmd,
		TeamGroupListCmd,
	)

	GroupCmd.AddCommand(
		ChannelGroupCmd,
		TeamGroupCmd,
	)

	RootCmd.AddCommand(GroupCmd)
}

func channelGroupEnableCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	if channel.Type != model.ChannelTypePrivate {
		return errors.New("Channel '" + args[0] + "' is not private. It cannot be group-constrained")
	}

	groups, _, appErr := a.GetGroupsByChannel(channel.ID, model.GroupSearchOpts{})
	if appErr != nil {
		return appErr
	}

	if len(groups) == 0 {
		return errors.New("Channel '" + args[0] + "' has no groups associated. It cannot be group-constrained")
	}

	channel.GroupConstrained = model.NewBool(true)
	if _, appErr = a.UpdateChannel(channel); appErr != nil {
		return appErr
	}

	auditRec := a.MakeAuditRecord("channelGroupEnable", audit.Success)
	auditRec.AddMeta("channel", channel)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func channelGroupDisableCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	channel.GroupConstrained = model.NewBool(false)
	if _, appErr := a.UpdateChannel(channel); appErr != nil {
		return appErr
	}

	auditRec := a.MakeAuditRecord("channelGroupDisable", audit.Success)
	auditRec.AddMeta("channel", channel)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func channelGroupStatusCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	if channel.IsGroupConstrained() {
		fmt.Println("Enabled")
	} else {
		fmt.Println("Disabled")
	}

	return nil
}

func channelGroupListCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	groups, _, appErr := a.GetGroupsByChannel(channel.ID, model.GroupSearchOpts{})
	if appErr != nil {
		return appErr
	}

	for _, group := range groups {
		fmt.Println(group.DisplayName)
	}

	return nil
}

func teamGroupEnableCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find team '" + args[0] + "'")
	}

	groups, _, appErr := a.GetGroupsByTeam(team.ID, model.GroupSearchOpts{})
	if appErr != nil {
		return appErr
	}

	if len(groups) == 0 {
		return errors.New("Team '" + args[0] + "' has no groups associated. It cannot be group-constrained")
	}

	team.GroupConstrained = model.NewBool(true)
	if _, appErr = a.UpdateTeam(team); appErr != nil {
		return appErr
	}

	auditRec := a.MakeAuditRecord("teamGroupEnable", audit.Success)
	auditRec.AddMeta("team", team)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func teamGroupDisableCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find team '" + args[0] + "'")
	}

	team.GroupConstrained = model.NewBool(false)
	if _, appErr := a.UpdateTeam(team); appErr != nil {
		return appErr
	}

	auditRec := a.MakeAuditRecord("teamGroupDisable", audit.Success)
	auditRec.AddMeta("team", team)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func teamGroupStatusCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find team '" + args[0] + "'")
	}

	if team.IsGroupConstrained() {
		fmt.Println("Enabled")
	} else {
		fmt.Println("Disabled")
	}

	return nil
}

func teamGroupListCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find team '" + args[0] + "'")
	}

	groups, _, appErr := a.GetGroupsByTeam(team.ID, model.GroupSearchOpts{})
	if appErr != nil {
		return appErr
	}

	for _, group := range groups {
		fmt.Println(group.DisplayName)
	}

	return nil
}
