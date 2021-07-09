// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

var WebhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Management of webhooks",
}

var WebhookListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List webhooks",
	Long:    "list all webhooks",
	Example: "  webhook list myteam",
	RunE:    listWebhookCmdF,
}

var WebhookShowCmd = &cobra.Command{
	Use:     "show [webhookId]",
	Short:   "Show a webhook",
	Long:    "Show the webhook specified by [webhookId]",
	Args:    cobra.ExactArgs(1),
	Example: "  webhook show w16zb5tu3n1zkqo18goqry1je",
	RunE:    showWebhookCmdF,
}

var WebhookCreateIncomingCmd = &cobra.Command{
	Use:     "create-incoming",
	Short:   "Create incoming webhook",
	Long:    "create incoming webhook which allows external posting of messages to specific channel",
	Example: "  webhook create-incoming --channel [channelID] --user [userID] --display-name [displayName] --description [webhookDescription] --lock-to-channel --icon [iconURL]",
	RunE:    createIncomingWebhookCmdF,
}

var WebhookModifyIncomingCmd = &cobra.Command{
	Use:     "modify-incoming",
	Short:   "Modify incoming webhook",
	Long:    "Modify existing incoming webhook by changing its title, description, channel or icon url",
	Example: "  webhook modify-incoming [webhookID] --channel [channelID] --display-name [displayName] --description [webhookDescription] --lock-to-channel --icon [iconURL]",
	RunE:    modifyIncomingWebhookCmdF,
}

var WebhookCreateOutgoingCmd = &cobra.Command{
	Use:   "create-outgoing",
	Short: "Create outgoing webhook",
	Long:  "create outgoing webhook which allows external posting of messages from a specific channel",
	Example: `  webhook create-outgoing --team myteam --user myusername --display-name mywebhook --trigger-word "build" --trigger-word "test" --url http://localhost:8000/my-webhook-handler
	webhook create-outgoing --team myteam --channel mychannel --user myusername --display-name mywebhook --description "My cool webhook" --trigger-when start --trigger-word build --trigger-word test --icon http://localhost:8000/my-slash-handler-bot-icon.png --url http://localhost:8000/my-webhook-handler --content-type "application/json"`,
	RunE: createOutgoingWebhookCmdF,
}

var WebhookModifyOutgoingCmd = &cobra.Command{
	Use:     "modify-outgoing",
	Short:   "Modify outgoing webhook",
	Long:    "Modify existing outgoing webhook by changing its title, description, channel, icon, url, content-type, and triggers",
	Example: `  webhook modify-outgoing [webhookId] --channel [channelId] --display-name [displayName] --description "New webhook description" --icon http://localhost:8000/my-slash-handler-bot-icon.png --url http://localhost:8000/my-webhook-handler --content-type "application/json" --trigger-word test --trigger-when start`,
	RunE:    modifyOutgoingWebhookCmdF,
}

var WebhookDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete webhooks",
	Long:    "Delete webhook with given id",
	Example: "  webhook delete [webhookID]",
	RunE:    deleteWebhookCmdF,
}

var WebhookMoveOutgoingCmd = &cobra.Command{
	Use:     "move-outgoing",
	Short:   "Move outgoing webhook",
	Long:    "Move outgoing webhook with an id",
	Example: "  webhook move-outgoing newteam oldteam:webhook-id --channel new-default-channel",
	Args:    cobra.ExactArgs(2),
	RunE:    moveOutgoingWebhookCmd,
}

func listWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	var teams []*model.Team
	if len(args) < 1 {
		var getErr *model.AppError
		// If no team is specified, list all teams
		teams, getErr = app.GetAllTeams()
		if getErr != nil {
			return getErr
		}
	} else {
		teams = getTeamsFromTeamArgs(app, args)
	}

	for i, team := range teams {
		if team == nil {
			CommandPrintErrorln("Unable to find team '" + args[i] + "'")
			continue
		}

		// Fetch all hooks with a very large limit so we get them all.
		incomingResult := make(chan store.StoreResult, 1)
		go func() {
			incomingHooks, err := app.Srv().Store.Webhook().GetIncomingByTeam(team.ID, 0, 100000000)
			incomingResult <- store.StoreResult{Data: incomingHooks, NErr: err}
			close(incomingResult)
		}()
		outgoingResult := make(chan store.StoreResult, 1)
		go func() {
			outgoingHooks, err := app.Srv().Store.Webhook().GetOutgoingByTeam(team.ID, 0, 100000000)
			outgoingResult <- store.StoreResult{Data: outgoingHooks, NErr: err}
			close(outgoingResult)
		}()

		if result := <-incomingResult; result.NErr == nil {
			CommandPrettyPrintln(fmt.Sprintf("Incoming webhooks for %s (%s):", team.DisplayName, team.Name))
			hooks := result.Data.([]*model.IncomingWebhook)
			for _, hook := range hooks {
				CommandPrettyPrintln("\t" + hook.DisplayName + " (" + hook.ID + ")")
			}
		} else {
			CommandPrintErrorln("Unable to list incoming webhooks for '" + args[i] + "'")
		}

		if result := <-outgoingResult; result.NErr == nil {
			hooks := result.Data.([]*model.OutgoingWebhook)
			CommandPrettyPrintln(fmt.Sprintf("Outgoing webhooks for %s (%s):", team.DisplayName, team.Name))
			for _, hook := range hooks {
				CommandPrettyPrintln("\t" + hook.DisplayName + " (" + hook.ID + ")")
			}
		} else {
			CommandPrintErrorln("Unable to list outgoing webhooks for '" + args[i] + "'")
		}
	}
	return nil
}

func createIncomingWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	channelArg, errChannel := command.Flags().GetString("channel")
	if errChannel != nil || channelArg == "" {
		return errors.New("Channel is required")
	}
	channel := getChannelFromChannelArg(app, channelArg)
	if channel == nil {
		return errors.New("Unable to find channel '" + channelArg + "'")
	}

	userArg, errUser := command.Flags().GetString("user")
	if errUser != nil || userArg == "" {
		return errors.New("User is required")
	}
	user := getUserFromUserArg(app, userArg)
	if user == nil {
		return errors.New("Unable to find user '" + userArg + "'")
	}

	displayName, _ := command.Flags().GetString("display-name")
	description, _ := command.Flags().GetString("description")
	iconURL, _ := command.Flags().GetString("icon")
	channelLocked, _ := command.Flags().GetBool("lock-to-channel")

	incomingWebhook := &model.IncomingWebhook{
		ChannelID:     channel.ID,
		DisplayName:   displayName,
		Description:   description,
		IconURL:       iconURL,
		ChannelLocked: channelLocked,
	}

	createdIncoming, errIncomingWebhook := app.CreateIncomingWebhookForChannel(user.ID, channel, incomingWebhook)
	if errIncomingWebhook != nil {
		return errIncomingWebhook
	}

	CommandPrettyPrintln("Id: " + createdIncoming.ID)
	CommandPrettyPrintln("Display Name: " + createdIncoming.DisplayName)

	auditRec := app.MakeAuditRecord("createIncomingWebhook", audit.Success)
	auditRec.AddMeta("user", user)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("hook", createdIncoming)
	app.LogAuditRec(auditRec, nil)

	return nil
}

func modifyIncomingWebhookCmdF(command *cobra.Command, args []string) (cmdError error) {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	if len(args) < 1 {
		return errors.New("WebhookID is not specified")
	}

	webhookArg := args[0]
	oldHook, getErr := app.GetIncomingWebhook(webhookArg)
	if getErr != nil {
		return errors.New("Unable to find webhook '" + webhookArg + "'")
	}

	updatedHook := oldHook

	auditRec := app.MakeAuditRecord("createIncomingWebhook", audit.Fail)
	defer func() { app.LogAuditRec(auditRec, cmdError) }()
	auditRec.AddMeta("hook", oldHook)

	channelArg, _ := command.Flags().GetString("channel")
	if channelArg != "" {
		channel := getChannelFromChannelArg(app, channelArg)
		if channel == nil {
			return errors.New("Unable to find channel '" + channelArg + "'")
		}
		updatedHook.ChannelID = channel.ID
	}

	displayName, _ := command.Flags().GetString("display-name")
	if displayName != "" {
		updatedHook.DisplayName = displayName
	}
	description, _ := command.Flags().GetString("description")
	if description != "" {
		updatedHook.Description = description
	}
	iconUrl, _ := command.Flags().GetString("icon")
	if iconUrl != "" {
		updatedHook.IconURL = iconUrl
	}
	channelLocked, _ := command.Flags().GetBool("lock-to-channel")
	updatedHook.ChannelLocked = channelLocked

	updatedIncomingHook, errUpdated := app.UpdateIncomingWebhook(oldHook, updatedHook)
	if errUpdated != nil {
		return errUpdated
	}

	auditRec.Success()
	auditRec.AddMeta("update", updatedIncomingHook)

	return nil
}

func createOutgoingWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	teamArg, errTeam := command.Flags().GetString("team")
	if errTeam != nil || teamArg == "" {
		return errors.New("Team is required")
	}
	team := getTeamFromTeamArg(app, teamArg)
	if team == nil {
		return errors.New("Unable to find team: " + teamArg)
	}

	userArg, errUser := command.Flags().GetString("user")
	if errUser != nil || userArg == "" {
		return errors.New("User is required")
	}
	user := getUserFromUserArg(app, userArg)
	if user == nil {
		return errors.New("Unable to find user: " + userArg)
	}

	displayName, errName := command.Flags().GetString("display-name")
	if errName != nil || displayName == "" {
		return errors.New("Display name is required")
	}

	triggerWords, errWords := command.Flags().GetStringArray("trigger-word")
	if errWords != nil || len(triggerWords) == 0 {
		return errors.New("Trigger word or words required")
	}

	callbackURLs, errURL := command.Flags().GetStringArray("url")
	if errURL != nil || len(callbackURLs) == 0 {
		return errors.New("Callback URL or URLs required")
	}

	triggerWhenString, _ := command.Flags().GetString("trigger-when")
	var triggerWhen int
	if triggerWhenString == "exact" {
		triggerWhen = 0
	} else if triggerWhenString == "start" {
		triggerWhen = 1
	} else {
		return errors.New("Invalid trigger when parameter")
	}
	description, _ := command.Flags().GetString("description")
	contentType, _ := command.Flags().GetString("content-type")
	iconURL, _ := command.Flags().GetString("icon")

	outgoingWebhook := &model.OutgoingWebhook{
		CreatorID:    user.ID,
		Username:     user.Username,
		TeamID:       team.ID,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: callbackURLs,
		DisplayName:  displayName,
		Description:  description,
		ContentType:  contentType,
		IconURL:      iconURL,
	}

	var channel *model.Channel
	channelArg, _ := command.Flags().GetString("channel")
	if channelArg != "" {
		channel = getChannelFromChannelArg(app, channelArg)
		if channel != nil {
			outgoingWebhook.ChannelID = channel.ID
		}
	}

	createdOutgoing, errOutgoing := app.CreateOutgoingWebhook(outgoingWebhook)
	if errOutgoing != nil {
		return errOutgoing
	}

	CommandPrettyPrintln("Id: " + createdOutgoing.ID)
	CommandPrettyPrintln("Display Name: " + createdOutgoing.DisplayName)

	auditRec := app.MakeAuditRecord("createOutgoingWebhook", audit.Success)
	auditRec.AddMeta("user", user)
	auditRec.AddMeta("hook", createdOutgoing)
	if channel != nil {
		auditRec.AddMeta("channel", channel)
	}
	app.LogAuditRec(auditRec, nil)

	return nil
}

func modifyOutgoingWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	if len(args) < 1 {
		return errors.New("WebhookID is not specified")
	}

	webhookArg := args[0]
	oldHook, appErr := app.GetOutgoingWebhook(webhookArg)
	if appErr != nil {
		return fmt.Errorf("unable to find webhook '%s'", webhookArg)
	}

	updatedHook := model.OutgoingWebhookFromJSON(strings.NewReader(oldHook.ToJSON()))

	channelArg, _ := command.Flags().GetString("channel")
	if channelArg != "" {
		channel := getChannelFromChannelArg(app, channelArg)
		if channel == nil {
			return fmt.Errorf("unable to find channel '%s'", channelArg)
		}
		updatedHook.ChannelID = channel.ID
	}

	displayName, _ := command.Flags().GetString("display-name")
	if displayName != "" {
		updatedHook.DisplayName = displayName
	}

	description, _ := command.Flags().GetString("description")
	if description != "" {
		updatedHook.Description = description
	}

	triggerWords, err := command.Flags().GetStringArray("trigger-word")
	if err != nil {
		return errors.Wrap(err, "invalid trigger-word parameter")
	}
	if len(triggerWords) > 0 {
		updatedHook.TriggerWords = triggerWords
	}

	triggerWhenString, _ := command.Flags().GetString("trigger-when")
	if triggerWhenString != "" {
		var triggerWhen int
		if triggerWhenString == "exact" {
			triggerWhen = 0
		} else if triggerWhenString == "start" {
			triggerWhen = 1
		} else {
			return errors.New("invalid trigger-when parameter")
		}
		updatedHook.TriggerWhen = triggerWhen
	}

	iconURL, _ := command.Flags().GetString("icon")
	if iconURL != "" {
		updatedHook.IconURL = iconURL
	}

	contentType, _ := command.Flags().GetString("content-type")
	if contentType != "" {
		updatedHook.ContentType = contentType
	}

	callbackURLs, err := command.Flags().GetStringArray("url")
	if err != nil {
		return errors.Wrap(err, "invalid URL parameter")
	}
	if len(callbackURLs) > 0 {
		updatedHook.CallbackURLs = callbackURLs
	}

	updatedWebhook, appErr := app.UpdateOutgoingWebhook(oldHook, updatedHook)
	if appErr != nil {
		return appErr
	}

	auditRec := app.MakeAuditRecord("modifyOutgoingWebhook", audit.Success)
	auditRec.AddMeta("hook", oldHook)
	auditRec.AddMeta("update", updatedWebhook)
	app.LogAuditRec(auditRec, nil)

	return nil
}

func deleteWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	if len(args) < 1 {
		return errors.New("WebhookID is not specified")
	}

	webhookID := args[0]
	errIncomingWebhook := app.DeleteIncomingWebhook(webhookID)
	errOutgoingWebhook := app.DeleteOutgoingWebhook(webhookID)

	if errIncomingWebhook != nil && errOutgoingWebhook != nil {
		return errors.New("Unable to delete webhook '" + webhookID + "'")
	}

	auditRec := app.MakeAuditRecord("deleteWebhook", audit.Success)
	auditRec.AddMeta("hook_id", webhookID)
	app.LogAuditRec(auditRec, nil)

	return nil
}

func showWebhookCmdF(command *cobra.Command, args []string) error {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	webhookID := args[0]
	if incomingWebhook, err := app.GetIncomingWebhook(webhookID); err == nil {
		fmt.Printf("%s", prettyPrintStruct(*incomingWebhook))
		return nil
	}
	if outgoingWebhook, err := app.GetOutgoingWebhook(webhookID); err == nil {
		fmt.Printf("%s", prettyPrintStruct(*outgoingWebhook))
		return nil
	}

	return errors.New("Webhook with id " + webhookID + " not found")
}

func moveOutgoingWebhookCmd(command *cobra.Command, args []string) (cmdError error) {
	app, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer app.Srv().Shutdown()

	newTeamID := args[0]
	_, teamError := app.GetTeam(newTeamID)
	if teamError != nil {
		return teamError
	}

	webhookInformation := strings.Split(args[1], ":")
	sourceTeam := webhookInformation[0]
	_, teamErr := app.GetTeam(sourceTeam)
	if teamErr != nil {
		return teamErr
	}

	webhookID := webhookInformation[1]
	webhook, appError := app.GetOutgoingWebhook(webhookID)
	if appError != nil {
		return appError
	}

	auditRec := app.MakeAuditRecord("moveOutgoingWebhook", audit.Fail)
	defer func() { app.LogAuditRec(auditRec, cmdError) }()
	auditRec.AddMeta("hook", webhook)

	channelName, channelErr := command.Flags().GetString("channel")
	if channelErr != nil {
		return channelErr
	}
	channel, getChannelErr := app.GetChannelByName(channelName, newTeamID, false)

	if webhook.ChannelID != "" {
		if getChannelErr != nil {
			return getChannelErr
		}
		webhook.ChannelID = channel.ID
	} else if channelName != "" {
		webhook.ChannelID = channel.ID
	}

	deleteErr := app.DeleteOutgoingWebhook(webhook.ID)
	if deleteErr != nil {
		return deleteErr
	}

	webhook.ID = ""
	webhook.TeamID = newTeamID

	updatedWebHook, createErr := app.CreateOutgoingWebhook(webhook)
	if createErr != nil {
		return model.NewAppError("moveOutgoingWebhookCmd", "cli.outgoing_webhook.inconsistent_state.app_error", nil, createErr.Error(), http.StatusInternalServerError)
	}

	auditRec.Success()
	auditRec.AddMeta("update", updatedWebHook)

	return nil
}

func init() {
	WebhookCreateIncomingCmd.Flags().String("channel", "", "Channel ID (required)")
	WebhookCreateIncomingCmd.Flags().String("user", "", "User ID (required)")
	WebhookCreateIncomingCmd.Flags().String("display-name", "", "Incoming webhook display name")
	WebhookCreateIncomingCmd.Flags().String("description", "", "Incoming webhook description")
	WebhookCreateIncomingCmd.Flags().String("icon", "", "Icon URL")
	WebhookCreateIncomingCmd.Flags().Bool("lock-to-channel", false, "Lock to channel")

	WebhookModifyIncomingCmd.Flags().String("channel", "", "Channel ID")
	WebhookModifyIncomingCmd.Flags().String("display-name", "", "Incoming webhook display name")
	WebhookModifyIncomingCmd.Flags().String("description", "", "Incoming webhook description")
	WebhookModifyIncomingCmd.Flags().String("icon", "", "Icon URL")
	WebhookModifyIncomingCmd.Flags().Bool("lock-to-channel", false, "Lock to channel")

	WebhookCreateOutgoingCmd.Flags().String("team", "", "Team name or ID (required)")
	WebhookCreateOutgoingCmd.Flags().String("channel", "", "Channel name or ID")
	WebhookCreateOutgoingCmd.Flags().String("user", "", "User username, email, or ID (required)")
	WebhookCreateOutgoingCmd.Flags().String("display-name", "", "Outgoing webhook display name (required)")
	WebhookCreateOutgoingCmd.Flags().String("description", "", "Outgoing webhook description")
	WebhookCreateOutgoingCmd.Flags().StringArray("trigger-word", []string{}, "Word to trigger webhook (required)")
	WebhookCreateOutgoingCmd.Flags().String("trigger-when", "exact", "When to trigger webhook (exact: for first word matches a trigger word exactly, start: for first word starts with a trigger word)")
	WebhookCreateOutgoingCmd.Flags().String("icon", "", "Icon URL")
	WebhookCreateOutgoingCmd.Flags().StringArray("url", []string{}, "Callback URL (required)")
	WebhookCreateOutgoingCmd.Flags().String("content-type", "", "Content-type")

	WebhookModifyOutgoingCmd.Flags().String("channel", "", "Channel name or ID")
	WebhookModifyOutgoingCmd.Flags().String("display-name", "", "Outgoing webhook display name")
	WebhookModifyOutgoingCmd.Flags().String("description", "", "Outgoing webhook description")
	WebhookModifyOutgoingCmd.Flags().StringArray("trigger-word", []string{}, "Word to trigger webhook")
	WebhookModifyOutgoingCmd.Flags().String("trigger-when", "", "When to trigger webhook (exact: for first word matches a trigger word exactly, start: for first word starts with a trigger word)")
	WebhookModifyOutgoingCmd.Flags().String("icon", "", "Icon URL")
	WebhookModifyOutgoingCmd.Flags().StringArray("url", []string{}, "Callback URL")
	WebhookModifyOutgoingCmd.Flags().String("content-type", "", "Content-type")

	WebhookMoveOutgoingCmd.Flags().String("channel", "", "Channel name or ID")

	WebhookCmd.AddCommand(
		WebhookListCmd,
		WebhookCreateIncomingCmd,
		WebhookModifyIncomingCmd,
		WebhookCreateOutgoingCmd,
		WebhookModifyOutgoingCmd,
		WebhookDeleteCmd,
		WebhookShowCmd,
		WebhookMoveOutgoingCmd,
	)

	RootCmd.AddCommand(WebhookCmd)
}
