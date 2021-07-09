// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

var CommandCmd = &cobra.Command{
	Use:   "command",
	Short: "Management of slash commands",
}

var CommandCreateCmd = &cobra.Command{
	Use:     "create [team]",
	Short:   "Create a custom slash command",
	Long:    `Create a custom slash command for the specified team.`,
	Args:    cobra.MinimumNArgs(1),
	Example: `  command create myteam --title MyCommand --description "My Command Description" --trigger-word mycommand --url http://localhost:8000/my-slash-handler --creator myusername --response-username my-bot-username --icon http://localhost:8000/my-slash-handler-bot-icon.png --autocomplete --post`,
	RunE:    createCommandCmdF,
}

var CommandShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "Show a custom slash command",
	Long:    `Show a custom slash command. Commands can be specified by command ID.`,
	Args:    cobra.ExactArgs(1),
	Example: `  command show commandID`,
	RunE:    showCommandCmdF,
}

var CommandMoveCmd = &cobra.Command{
	Use:     "move",
	Short:   "Move a slash command to a different team",
	Long:    `Move a slash command to a different team. Commands can be specified by [team]:[command-trigger-word]. ie. myteam:trigger or by command ID.`,
	Example: `  command move newteam oldteam:command`,
	RunE:    moveCommandCmdF,
}

var CommandListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all commands on specified teams.",
	Long:    `List all commands on specified teams.`,
	Example: ` command list myteam`,
	RunE:    listCommandCmdF,
}

var CommandDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a slash command",
	Long:    `Delete a slash command. Commands can be specified by command ID.`,
	Example: `  command delete commandID`,
	Args:    cobra.ExactArgs(1),
	RunE:    deleteCommandCmdF,
}

var CommandModifyCmd = &cobra.Command{
	Use:     "modify",
	Short:   "Modify a slash command",
	Long:    `Modify a slash command. Commands can be specified by command ID.`,
	Example: `  command modify commandID`,
	Args:    cobra.MinimumNArgs(1),
	RunE:    modifyCommandCmdF,
}

func init() {
	CommandCreateCmd.Flags().String("title", "", "Command Title")
	CommandCreateCmd.Flags().String("description", "", "Command Description")
	CommandCreateCmd.Flags().String("trigger-word", "", "Command Trigger Word (required)")
	CommandCreateCmd.MarkFlagRequired("trigger-word")
	CommandCreateCmd.Flags().String("url", "", "Command Callback URL (required)")
	CommandCreateCmd.MarkFlagRequired("url")
	CommandCreateCmd.Flags().String("creator", "", "Command Creator's Username (required)")
	CommandCreateCmd.MarkFlagRequired("creator")
	CommandCreateCmd.Flags().String("response-username", "", "Command Response Username")
	CommandCreateCmd.Flags().String("icon", "", "Command Icon URL")
	CommandCreateCmd.Flags().Bool("autocomplete", false, "Show Command in autocomplete list")
	CommandCreateCmd.Flags().String("autocompleteDesc", "", "Short Command Description for autocomplete list")
	CommandCreateCmd.Flags().String("autocompleteHint", "", "Command Arguments displayed as help in autocomplete list")
	CommandCreateCmd.Flags().Bool("post", false, "Use POST method for Callback URL")

	CommandModifyCmd.Flags().String("title", "", "Command Title")
	CommandModifyCmd.Flags().String("description", "", "Command Description")
	CommandModifyCmd.Flags().String("trigger-word", "", "Command Trigger Word")
	CommandModifyCmd.Flags().String("url", "", "Command Callback URL")
	CommandModifyCmd.Flags().String("creator", "", "Command Creator's Username")
	CommandModifyCmd.Flags().String("response-username", "", "Command Response Username")
	CommandModifyCmd.Flags().String("icon", "", "Command Icon URL")
	CommandModifyCmd.Flags().Bool("autocomplete", false, "Show Command in autocomplete list")
	CommandModifyCmd.Flags().String("autocompleteDesc", "", "Short Command Description for autocomplete list")
	CommandModifyCmd.Flags().String("autocompleteHint", "", "Command Arguments displayed as help in autocomplete list")
	CommandModifyCmd.Flags().Bool("post", false, "Use POST method for Callback URL")

	CommandCmd.AddCommand(
		CommandCreateCmd,
		CommandShowCmd,
		CommandMoveCmd,
		CommandListCmd,
		CommandDeleteCmd,
		CommandModifyCmd,
	)
	RootCmd.AddCommand(CommandCmd)
}

func createCommandCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("unable to find team '" + args[0] + "'")
	}

	// get the creator
	creator, _ := command.Flags().GetString("creator")
	user := getUserFromUserArg(a, creator)
	if user == nil {
		return errors.New("unable to find user '" + creator + "'")
	}

	// check if creator has permission to create slash commands
	if !a.HasPermissionToTeam(user.ID, team.ID, model.PermissionManageSlashCommands) {
		return errors.New("the creator must be a user who has permissions to manage slash commands")
	}

	title, _ := command.Flags().GetString("title")
	description, _ := command.Flags().GetString("description")
	trigger, _ := command.Flags().GetString("trigger-word")

	if strings.HasPrefix(trigger, "/") {
		return errors.New("a trigger word cannot begin with a /")
	}
	if strings.Contains(trigger, " ") {
		return errors.New("a trigger word must not contain spaces")
	}

	url, _ := command.Flags().GetString("url")
	responseUsername, _ := command.Flags().GetString("response-username")
	icon, _ := command.Flags().GetString("icon")
	autocomplete, _ := command.Flags().GetBool("autocomplete")
	autocompleteDesc, _ := command.Flags().GetString("autocompleteDesc")
	autocompleteHint, _ := command.Flags().GetString("autocompleteHint")
	post, errp := command.Flags().GetBool("post")
	method := "P"
	if errp != nil || !post {
		method = "G"
	}

	newCommand := &model.Command{
		CreatorID:        user.ID,
		TeamID:           team.ID,
		Trigger:          trigger,
		Method:           method,
		Username:         responseUsername,
		IconURL:          icon,
		AutoComplete:     autocomplete,
		AutoCompleteDesc: autocompleteDesc,
		AutoCompleteHint: autocompleteHint,
		DisplayName:      title,
		Description:      description,
		URL:              url,
	}

	createdCommand, errCreate := a.CreateCommand(newCommand)
	if errCreate != nil {
		return errors.New("unable to create command '" + newCommand.DisplayName + "'. " + errCreate.Error())
	}
	CommandPrettyPrintln("created command '" + newCommand.DisplayName + "'")

	auditRec := a.MakeAuditRecord("createCommand", audit.Success)
	auditRec.AddMeta("user", user)
	auditRec.AddMeta("command", createdCommand)
	a.LogAuditRec(auditRec, nil)
	return nil
}

func showCommandCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	slashCommand := getCommandFromCommandArg(a, args[0])
	if slashCommand == nil {
		command.SilenceUsage = true
		return errors.New("Unable to find command '" + args[0] + "'")
	}
	// pretty print
	fmt.Printf("%s", prettyPrintStruct(*slashCommand))

	return nil
}

func moveCommandCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	if len(args) < 2 {
		return errors.New("Enter the destination team and at least one command to move.")
	}

	team := getTeamFromTeamArg(a, args[0])
	if team == nil {
		return errors.New("Unable to find destination team '" + args[0] + "'")
	}

	commands := getCommandsFromCommandArgs(a, args[1:])
	for i, command := range commands {
		if command == nil {
			CommandPrintErrorln("Unable to find command '" + args[i+1] + "'")
			continue
		}
		if err := moveCommand(a, team, command); err != nil {
			CommandPrintErrorln("Unable to move command '" + command.DisplayName + "' error: " + err.Error())
		} else {
			CommandPrettyPrintln("Moved command '" + command.DisplayName + "'")

			auditRec := a.MakeAuditRecord("moveCommand", audit.Success)
			auditRec.AddMeta("team", team)
			auditRec.AddMeta("command", command)
			a.LogAuditRec(auditRec, nil)
		}
	}
	return nil
}

func moveCommand(a *app.App, team *model.Team, command *model.Command) *model.AppError {
	return a.MoveCommand(team, command)
}

func listCommandCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	var teams []*model.Team
	if len(args) < 1 {
		teamList, err := a.GetAllTeams()
		if err != nil {
			return err
		}
		teams = teamList
	} else {
		teams = getTeamsFromTeamArgs(a, args)
	}

	for i, team := range teams {
		if team == nil {
			CommandPrintErrorln("Unable to find team '" + args[i] + "'")
			continue
		}
		commands, err := a.Srv().Store.Command().GetByTeam(team.ID)
		if err != nil {
			CommandPrintErrorln("Unable to list commands for '" + args[i] + "'")
			continue
		}
		for _, command := range commands {
			commandListItem := fmt.Sprintf("%s: %s (team: %s)", command.ID, command.DisplayName, team.Name)
			CommandPrettyPrintln(commandListItem)
		}
	}
	return nil
}

func deleteCommandCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	slashCommand := getCommandFromCommandArg(a, args[0])
	if slashCommand == nil {
		command.SilenceUsage = true
		return errors.New("Unable to find command '" + args[0] + "'")
	}

	if err := a.DeleteCommand(slashCommand.ID); err != nil {
		command.SilenceUsage = true
		return errors.New("Unable to delete command '" + slashCommand.ID + "' error: " + err.Error())
	}
	CommandPrettyPrintln("Deleted command '" + slashCommand.ID + "' (" + slashCommand.DisplayName + ")")

	auditRec := a.MakeAuditRecord("deleteCommand", audit.Success)
	auditRec.AddMeta("command", slashCommand)
	a.LogAuditRec(auditRec, nil)
	return nil
}

func modifyCommandCmdF(command *cobra.Command, args []string) (cmdError error) {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	oldCommand := getCommandFromCommandArg(a, args[0])
	if oldCommand == nil {
		command.SilenceUsage = true
		return errors.New("Unable to find command '" + args[0] + "'")
	}
	modifiedCommand := oldCommand

	auditRec := a.MakeAuditRecord("modifyCommand", audit.Fail)
	defer func() { a.LogAuditRec(auditRec, cmdError) }()
	auditRec.AddMeta("command", oldCommand)

	// get creator user
	creator, _ := command.Flags().GetString("creator")
	if creator != "" {
		user := getUserFromUserArg(a, creator)
		if user == nil {
			return errors.New("unable to find user '" + creator + "'")
		}

		// check if creator has permission to create slash commands
		if !a.HasPermissionToTeam(user.ID, modifiedCommand.TeamID, model.PermissionManageSlashCommands) {
			return errors.New("the creator must be a user who has permissions to manage slash commands")
		}

		modifiedCommand.CreatorID = user.ID
	}

	title, _ := command.Flags().GetString("title")
	if title != "" {
		modifiedCommand.DisplayName = title
	}

	description, _ := command.Flags().GetString("description")
	if description != "" {
		modifiedCommand.Description = description
	}

	trigger, _ := command.Flags().GetString("trigger-word")
	if trigger != "" {
		if strings.HasPrefix(trigger, "/") {
			return errors.New("a trigger word cannot begin with a /")
		}
		if strings.Contains(trigger, " ") {
			return errors.New("a trigger word must not contain spaces")
		}
		modifiedCommand.Trigger = trigger
	}

	url, _ := command.Flags().GetString("url")
	if url != "" {
		modifiedCommand.URL = url
	}

	responseUsername, _ := command.Flags().GetString("response-username")
	if responseUsername != "" {
		modifiedCommand.Username = responseUsername
	}

	icon, _ := command.Flags().GetString("icon")
	if icon != "" {
		modifiedCommand.IconURL = icon
	}

	autocomplete, _ := command.Flags().GetBool("autocomplete")
	modifiedCommand.AutoComplete = autocomplete

	autocompleteDesc, _ := command.Flags().GetString("autocompleteDesc")
	if autocompleteDesc != "" {
		modifiedCommand.AutoCompleteDesc = autocompleteDesc
	}

	autocompleteHint, _ := command.Flags().GetString("autocompleteHint")
	if autocompleteHint != "" {
		modifiedCommand.AutoCompleteHint = autocompleteHint
	}

	post, err := command.Flags().GetBool("post")
	method := "P"
	if err != nil || !post {
		method = "G"
	}
	modifiedCommand.Method = method

	updatedCommand, errUpdated := a.UpdateCommand(oldCommand, modifiedCommand)
	if errUpdated != nil {
		return errors.New("unable to modify command '" + modifiedCommand.DisplayName + "'. " + errUpdated.Error())
	}
	CommandPrettyPrintln("modified command '" + modifiedCommand.DisplayName + "'")

	auditRec.Success()
	auditRec.AddMeta("update", updatedCommand)

	return nil
}
