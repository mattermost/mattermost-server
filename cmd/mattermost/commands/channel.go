// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

var ChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Management of channels",
}

var ChannelCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a channel",
	Long:  `Create a channel.`,
	Example: `  channel create --team myteam --name mynewchannel --display_name "My New Channel"
  channel create --team myteam --name mynewprivatechannel --display_name "My New Private Channel" --private`,
	RunE: createChannelCmdF,
}

var ChannelRenameCmd = &cobra.Command{
	Use:     "rename",
	Short:   "Rename a channel",
	Long:    `Rename a channel.`,
	Example: `"  channel rename myteam:mychannel newchannelname --display_name "New Display Name"`,
	Args:    cobra.MinimumNArgs(2),
	RunE:    renameChannelCmdF,
}

var RemoveChannelUsersCmd = &cobra.Command{
	Use:   "remove [channel] [users]",
	Short: "Remove users from channel",
	Long:  "Remove some users from channel",
	Example: `  channel remove myteam:mychannel user@example.com username
  channel remove myteam:mychannel --all-users`,
	RunE: removeChannelUsersCmdF,
}

var AddChannelUsersCmd = &cobra.Command{
	Use:     "add [channel] [users]",
	Short:   "Add users to channel",
	Long:    "Add some users to channel",
	Example: "  channel add myteam:mychannel user@example.com username",
	Args:    cobra.MinimumNArgs(2),
	RunE:    addChannelUsersCmdF,
}

var ArchiveChannelsCmd = &cobra.Command{
	Use:   "archive [channels]",
	Short: "Archive channels",
	Long: `Archive some channels.
Archive a channel along with all related information including posts from the database.
Channels can be specified by [team]:[channel]. ie. myteam:mychannel or by channel ID.`,
	Example: "  channel archive myteam:mychannel",
	Args:    cobra.MinimumNArgs(1),
	RunE:    archiveChannelsCmdF,
}

var DeleteChannelsCmd = &cobra.Command{
	Use:   "delete [channels]",
	Short: "Delete channels",
	Long: `Permanently delete some channels.
Permanently deletes a channel along with all related information including posts from the database.
Channels can be specified by [team]:[channel]. ie. myteam:mychannel or by channel ID.`,
	Example: "  channel delete myteam:mychannel",
	Args:    cobra.MinimumNArgs(1),
	RunE:    deleteChannelsCmdF,
}

var ListChannelsCmd = &cobra.Command{
	Use:   "list [teams]",
	Short: "List all channels on specified teams.",
	Long: `List all channels on specified teams.
Archived channels are appended with ' (archived)'.
Private channels are appended with ' (private)'.`,
	Example: "  channel list myteam",
	Args:    cobra.MinimumNArgs(1),
	RunE:    listChannelsCmdF,
}

var MoveChannelsCmd = &cobra.Command{
	Use:   "move [team] [channels] --username [user]",
	Short: "Moves channels to the specified team",
	Long: `Moves the provided channels to the specified team.
Validates that all users in the channel belong to the target team. Incoming/Outgoing webhooks are moved along with the channel.
Channels can be specified by [team]:[channel]. ie. myteam:mychannel or by channel ID.`,
	Example: "  channel move newteam oldteam:mychannel --username myusername",
	Args:    cobra.MinimumNArgs(2),
	RunE:    moveChannelsCmdF,
}

var RestoreChannelsCmd = &cobra.Command{
	Use:   "restore [channels]",
	Short: "Restore some channels",
	Long: `Restore a previously deleted channel
Channels can be specified by [team]:[channel]. ie. myteam:mychannel or by channel ID.`,
	Example: "  channel restore myteam:mychannel",
	Args:    cobra.MinimumNArgs(1),
	RunE:    restoreChannelsCmdF,
}

var ModifyChannelCmd = &cobra.Command{
	Use:   "modify [channel] [flags] --username [user]",
	Short: "Modify a channel's public/private type",
	Long: `Change the public/private type of a channel.
Channel can be specified by [team]:[channel]. ie. myteam:mychannel or by channel ID.`,
	Example: "  channel modify myteam:mychannel --private --username myusername",
	Args:    cobra.MinimumNArgs(1),
	RunE:    modifyChannelCmdF,
}

var SearchChannelCmd = &cobra.Command{
	Use:   "search [channel]\n  mattermost search --team [team] [channel]",
	Short: "Search a channel",
	Long: `Search a channel by channel name.
Channel can be specified by team. ie. --team myTeam myChannel or by team ID.`,
	Example: `  channel search myChannel
  channel search --team myTeam myChannel`,
	Args: cobra.ExactArgs(1),
	RunE: searchChannelCmdF,
}

func init() {
	ChannelCreateCmd.Flags().String("name", "", "Channel Name")
	ChannelCreateCmd.Flags().String("display_name", "", "Channel Display Name")
	ChannelCreateCmd.Flags().String("team", "", "Team name or ID")
	ChannelCreateCmd.Flags().String("header", "", "Channel header")
	ChannelCreateCmd.Flags().String("purpose", "", "Channel purpose")
	ChannelCreateCmd.Flags().Bool("private", false, "Create a private channel.")

	MoveChannelsCmd.Flags().String("username", "", "Required. Username who is moving the channel.")

	DeleteChannelsCmd.Flags().Bool("confirm", false, "Confirm you really want to delete the channels.")

	ModifyChannelCmd.Flags().Bool("private", false, "Convert the channel to a private channel")
	ModifyChannelCmd.Flags().Bool("public", false, "Convert the channel to a public channel")
	ModifyChannelCmd.Flags().String("username", "", "Required. Username who changes the channel privacy.")

	ChannelRenameCmd.Flags().String("display_name", "", "Channel Display Name")
	SearchChannelCmd.Flags().String("team", "", "Team name or ID")

	RemoveChannelUsersCmd.Flags().Bool("all-users", false, "Remove all users from the indicated channel.")

	ChannelCmd.AddCommand(
		ChannelCreateCmd,
		RemoveChannelUsersCmd,
		AddChannelUsersCmd,
		ArchiveChannelsCmd,
		DeleteChannelsCmd,
		ListChannelsCmd,
		MoveChannelsCmd,
		RestoreChannelsCmd,
		ModifyChannelCmd,
		ChannelRenameCmd,
		SearchChannelCmd,
	)

	RootCmd.AddCommand(ChannelCmd)
}

func createChannelCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	name, errn := command.Flags().GetString("name")
	if errn != nil || name == "" {
		return errors.New("Name is required")
	}
	displayname, errdn := command.Flags().GetString("display_name")
	if errdn != nil || displayname == "" {
		return errors.New("Display Name is required")
	}
	teamArg, errteam := command.Flags().GetString("team")
	if errteam != nil || teamArg == "" {
		return errors.New("Team is required")
	}
	header, _ := command.Flags().GetString("header")
	purpose, _ := command.Flags().GetString("purpose")
	useprivate, _ := command.Flags().GetBool("private")

	channelType := model.CHANNEL_OPEN
	if useprivate {
		channelType = model.CHANNEL_PRIVATE
	}

	team := getTeamFromTeamArg(a, teamArg)
	if team == nil {
		return errors.New("Unable to find team: " + teamArg)
	}

	channel := &model.Channel{
		TeamId:      team.Id,
		Name:        name,
		DisplayName: displayname,
		Header:      header,
		Purpose:     purpose,
		Type:        channelType,
		CreatorId:   "",
	}

	createdChannel, errCreatedChannel := a.CreateChannel(&request.Context{}, channel, false)
	if errCreatedChannel != nil {
		return errCreatedChannel
	}

	auditRec := a.MakeAuditRecord("createChannel", audit.Success)
	auditRec.AddMeta("channel", createdChannel)
	auditRec.AddMeta("team", team)
	a.LogAuditRec(auditRec, nil)

	CommandPrettyPrintln("Id: " + createdChannel.Id)
	CommandPrettyPrintln("Name: " + createdChannel.Name)
	CommandPrettyPrintln("Display Name: " + createdChannel.DisplayName)
	return nil
}

func removeChannelUsersCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	allUsers, _ := command.Flags().GetBool("all-users")

	if allUsers && len(args) != 1 {
		return errors.New("individual users must not be specified in conjunction with the --all-users flag")
	}

	if !allUsers && len(args) < 2 {
		return errors.New("you must specify some users to remove from the channel, or use the --all-users flag to remove them all")
	}

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	if allUsers {
		removeAllUsersFromChannel(a, channel)
	} else {
		users := getUsersFromUserArgs(a, args[1:])
		for i, user := range users {
			removeUserFromChannel(a, channel, user, args[i+1])
		}
	}
	return nil
}

func removeUserFromChannel(a *app.App, channel *model.Channel, user *model.User, userArg string) {
	if user == nil {
		CommandPrintErrorln("Can't find user '" + userArg + "'")
		return
	}
	if err := a.RemoveUserFromChannel(&request.Context{}, user.Id, "", channel); err != nil {
		CommandPrintErrorln("Unable to remove '" + userArg + "' from " + channel.Name + ". Error: " + err.Error())
		return
	}

	auditRec := a.MakeAuditRecord("removeUserFromChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("user", user)
	a.LogAuditRec(auditRec, nil)
}

func removeAllUsersFromChannel(a *app.App, channel *model.Channel) {
	if err := a.PermanentDeleteMembersByChannel(channel.Id); err != nil {
		CommandPrintErrorln("Unable to remove all users from " + channel.Name + ". Error: " + err.Error())
		return
	}

	auditRec := a.MakeAuditRecord("removeAllUsersFromChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	a.LogAuditRec(auditRec, nil)
}

func addChannelUsersCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	users := getUsersFromUserArgs(a, args[1:])
	for i, user := range users {
		addUserToChannel(a, channel, user, args[i+1])
	}
	return nil
}

func addUserToChannel(a *app.App, channel *model.Channel, user *model.User, userArg string) {
	if user == nil {
		CommandPrintErrorln("Can't find user '" + userArg + "'")
		return
	}
	if _, err := a.AddUserToChannel(user, channel, false); err != nil {
		CommandPrintErrorln("Unable to add '" + userArg + "' from " + channel.Name + ". Error: " + err.Error())
		return
	}

	auditRec := a.MakeAuditRecord("addUserToChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("user", user)
	a.LogAuditRec(auditRec, nil)
}

func archiveChannelsCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channels := getChannelsFromChannelArgs(a, args)
	for i, channel := range channels {
		if channel == nil {
			CommandPrintErrorln("Unable to find channel '" + args[i] + "'")
			continue
		}
		if err := a.DeleteChannel(request.EmptyContext(), channel, ""); err != nil {
			CommandPrintErrorln("Unable to archive channel '" + channel.Name + "' error: " + err.Error())
			continue
		}
		auditRec := a.MakeAuditRecord("archiveChannel", audit.Success)
		auditRec.AddMeta("channel", channel)
		a.LogAuditRec(auditRec, nil)
	}
	return nil
}

func deleteChannelsCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	confirmFlag, _ := command.Flags().GetBool("confirm")
	if !confirmFlag {
		var confirm string
		CommandPrettyPrintln("Are you sure you want to delete the channels specified?  All data will be permanently deleted? (YES/NO): ")
		fmt.Scanln(&confirm)
		if confirm != "YES" {
			return errors.New("ABORTED: You did not answer YES exactly, in all capitals.")
		}
	}

	channels := getChannelsFromChannelArgs(a, args)
	for i, channel := range channels {
		if channel == nil {
			CommandPrintErrorln("Unable to find channel '" + args[i] + "'")
			continue
		}
		if err := deleteChannel(a, channel); err != nil {
			CommandPrintErrorln("Unable to delete channel '" + channel.Name + "' error: " + err.Error())
		} else {
			CommandPrettyPrintln("Deleted channel '" + channel.Name + "'")

			auditRec := a.MakeAuditRecord("deleteChannel", audit.Success)
			auditRec.AddMeta("channel", channel)
			a.LogAuditRec(auditRec, nil)
		}
	}
	return nil
}

func deleteChannel(a *app.App, channel *model.Channel) *model.AppError {
	return a.PermanentDeleteChannel(channel)
}

func moveChannelsCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find destination team '" + args[0] + "'")
	}

	username, erru := command.Flags().GetString("username")
	if erru != nil || username == "" {
		return errors.New("Username is required.")
	}
	user := getUserFromUserArg(a, username)

	channels := getChannelsFromChannelArgs(a, args[1:])
	for i, channel := range channels {
		if channel == nil {
			CommandPrintErrorln("Unable to find channel '" + args[i+1] + "'")
			continue
		}
		originTeamID := channel.TeamId
		if err := moveChannel(a, team, channel, user); err != nil {
			CommandPrintErrorln("Unable to move channel '" + channel.Name + "' error: " + err.Error())
		} else {
			CommandPrettyPrintln("Moved channel '" + channel.Name + "' to " + team.Name + "(" + team.Id + ") from " + originTeamID + ".")
		}
	}
	return nil
}

func moveChannel(a *app.App, team *model.Team, channel *model.Channel, user *model.User) *model.AppError {
	oldTeamId := channel.TeamId

	if err := a.RemoveAllDeactivatedMembersFromChannel(channel); err != nil {
		return err
	}

	if err := a.MoveChannel(&request.Context{}, team, channel, user); err != nil {
		return err
	}

	auditRec := a.MakeAuditRecord("moveChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("team", team)
	a.LogAuditRec(auditRec, nil)

	incomingWebhooks, err := a.GetIncomingWebhooksForTeamPage(oldTeamId, 0, 10000000)
	if err != nil {
		return err
	}
	for _, webhook := range incomingWebhooks {
		if webhook.ChannelId == channel.Id {
			webhook.TeamId = team.Id
			if _, err := a.Srv().Store.Webhook().UpdateIncoming(webhook); err != nil {
				CommandPrintErrorln("Failed to move incoming webhook '" + webhook.Id + "' to new team.")
			}
		}
	}

	outgoingWebhooks, err := a.GetOutgoingWebhooksForTeamPage(oldTeamId, 0, 10000000)
	if err != nil {
		return err
	}
	for _, webhook := range outgoingWebhooks {
		if webhook.ChannelId == channel.Id {
			webhook.TeamId = team.Id
			if _, err := a.Srv().Store.Webhook().UpdateOutgoing(webhook); err != nil {
				CommandPrintErrorln("Failed to move outgoing webhook '" + webhook.Id + "' to new team.")
			}
		}
	}

	return nil
}

func listChannelsCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	teams := getTeamsFromTeamArgs(a, args)
	for i, team := range teams {
		if team == nil {
			CommandPrintErrorln("Unable to find team '" + args[i] + "'")
			continue
		}
		if channels, chanErr := a.GetAllChannels(0, 999999, model.ChannelSearchOpts{
			TeamIds:        []string{team.Id},
			IncludeDeleted: true,
			Public:         true,
			Private:        true,
		}); chanErr != nil {
			CommandPrintErrorln("Unable to list channels for '" + args[i] + "'")
		} else {
			for _, channel := range *channels {
				output := channel.Name
				if channel.DeleteAt > 0 {
					output += " (archived)"
				}
				if channel.Type == model.CHANNEL_PRIVATE {
					output += " (private)"
				}
				CommandPrettyPrintln(output)
			}
		}
	}

	return nil
}

func restoreChannelsCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channels := getChannelsFromChannelArgs(a, args)
	for i, channel := range channels {
		if channel == nil {
			CommandPrintErrorln("Unable to find channel '" + args[i] + "'")
			continue
		}
		if _, err := a.RestoreChannel(request.EmptyContext(), channel, ""); err != nil {
			CommandPrintErrorln("Unable to restore channel '" + args[i] + "'")
			continue
		}
		auditRec := a.MakeAuditRecord("restoreChannel", audit.Success)
		auditRec.AddMeta("channel", channel)
		a.LogAuditRec(auditRec, nil)
	}

	suffix := ""
	if len(channels) > 1 {
		suffix = "s"
	}
	CommandPrintln("Successfully restored channel" + suffix)
	return nil
}

func modifyChannelCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	username, erru := command.Flags().GetString("username")
	if erru != nil || username == "" {
		return errors.New("Username is required.")
	}

	public, _ := command.Flags().GetBool("public")
	private, _ := command.Flags().GetBool("private")

	if public == private {
		return errors.New("You must specify only one of --public or --private")
	}

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	if !(channel.Type == model.CHANNEL_OPEN || channel.Type == model.CHANNEL_PRIVATE) {
		return errors.New("You can only change the type of public/private channels.")
	}

	channel.Type = model.CHANNEL_OPEN
	if private {
		channel.Type = model.CHANNEL_PRIVATE
	}

	user := getUserFromUserArg(a, username)
	if user == nil {
		return fmt.Errorf("Unable to find user: '%v'", username)
	}

	updatedChannel, errUpdate := a.UpdateChannelPrivacy(&request.Context{}, channel, user)
	if errUpdate != nil {
		return errors.Wrapf(err, "Failed to update channel ('%s') privacy", args[0])
	}

	auditRec := a.MakeAuditRecord("modifyChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("user", user)
	auditRec.AddMeta("update", updatedChannel)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func renameChannelCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	var newDisplayName, newChannelName string
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	channel := getChannelFromChannelArg(a, args[0])
	if channel == nil {
		return errors.New("Unable to find channel '" + args[0] + "'")
	}

	newChannelName = args[1]
	newDisplayName, errdn := command.Flags().GetString("display_name")
	if errdn != nil {
		return errdn
	}

	updatedChannel, errch := a.RenameChannel(channel, newChannelName, newDisplayName)
	if errch != nil {
		return errors.Wrapf(errch, "Error in updating channel from %s to %s", channel.Name, newChannelName)
	}

	auditRec := a.MakeAuditRecord("renameChannel", audit.Success)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("update", updatedChannel)
	a.LogAuditRec(auditRec, nil)

	return nil
}

func searchChannelCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return errors.Wrap(err, "failed to InitDBCommandContextCobra")
	}
	defer a.Srv().Shutdown()

	var channel *model.Channel

	if teamArg, _ := command.Flags().GetString("team"); teamArg != "" {
		team := getTeamFromTeamArg(a, teamArg)
		if team == nil {
			CommandPrettyPrintln(fmt.Sprintf("Team %s is not found", teamArg))
			return nil
		}

		var aErr *model.AppError
		channel, aErr = a.GetChannelByName(args[0], team.Id, true)
		if aErr != nil || channel == nil {
			CommandPrettyPrintln(fmt.Sprintf("Channel %s is not found in team %s", args[0], teamArg))
			return nil
		}
	} else {
		teams, aErr := a.GetAllTeams()
		if aErr != nil {
			return errors.Wrap(err, "failed to GetAllTeams")
		}

		for _, team := range teams {
			channel, _ = a.GetChannelByName(args[0], team.Id, true)
			if channel != nil && channel.Name == args[0] {
				break
			}
		}

		if channel == nil {
			CommandPrettyPrintln(fmt.Sprintf("Channel %s is not found in any team", args[0]))
			return nil
		}
	}

	output := fmt.Sprintf(`Channel Name: %s, Display Name: %s, Channel ID: %s`, channel.Name, channel.DisplayName, channel.Id)
	if channel.DeleteAt > 0 {
		output += " (archived)"
	}
	if channel.Type == model.CHANNEL_PRIVATE {
		output += " (private)"
	}
	CommandPrettyPrintln(output)

	return nil
}
