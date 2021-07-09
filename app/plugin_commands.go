// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
)

type PluginCommand struct {
	Command  *model.Command
	PluginID string
}

func (a *App) RegisterPluginCommand(pluginID string, command *model.Command) error {
	if command.Trigger == "" {
		return errors.New("invalid command")
	}
	if command.AutocompleteData != nil {
		if err := command.AutocompleteData.IsValid(); err != nil {
			return errors.Wrap(err, "invalid autocomplete data in command")
		}
	}

	if command.AutocompleteData == nil {
		command.AutocompleteData = model.NewAutocompleteData(command.Trigger, command.AutoCompleteHint, command.AutoCompleteDesc)
	} else {
		baseURL, err := url.Parse("/plugins/" + pluginID)
		if err != nil {
			return errors.Wrapf(err, "Can't parse url %s", "/plugins/"+pluginID)
		}
		err = command.AutocompleteData.UpdateRelativeURLsForPluginCommands(baseURL)
		if err != nil {
			return errors.Wrap(err, "Can't update relative urls for plugin commands")
		}
	}

	command = &model.Command{
		Trigger:              strings.ToLower(command.Trigger),
		TeamID:               command.TeamID,
		AutoComplete:         command.AutoComplete,
		AutoCompleteDesc:     command.AutoCompleteDesc,
		AutoCompleteHint:     command.AutoCompleteHint,
		DisplayName:          command.DisplayName,
		AutocompleteData:     command.AutocompleteData,
		AutocompleteIconData: command.AutocompleteIconData,
	}

	a.Srv().pluginCommandsLock.Lock()
	defer a.Srv().pluginCommandsLock.Unlock()

	for _, pc := range a.Srv().pluginCommands {
		if pc.Command.Trigger == command.Trigger && pc.Command.TeamID == command.TeamID {
			if pc.PluginID == pluginID {
				pc.Command = command
				return nil
			}
		}
	}

	a.Srv().pluginCommands = append(a.Srv().pluginCommands, &PluginCommand{
		Command:  command,
		PluginID: pluginID,
	})
	return nil
}

func (a *App) UnregisterPluginCommand(pluginID, teamID, trigger string) {
	trigger = strings.ToLower(trigger)

	a.Srv().pluginCommandsLock.Lock()
	defer a.Srv().pluginCommandsLock.Unlock()

	var remaining []*PluginCommand
	for _, pc := range a.Srv().pluginCommands {
		if pc.Command.TeamID != teamID || pc.Command.Trigger != trigger {
			remaining = append(remaining, pc)
		}
	}
	a.Srv().pluginCommands = remaining
}

func (a *App) UnregisterPluginCommands(pluginID string) {
	a.Srv().unregisterPluginCommands(pluginID)
}

func (s *Server) unregisterPluginCommands(pluginID string) {
	s.pluginCommandsLock.Lock()
	defer s.pluginCommandsLock.Unlock()

	var remaining []*PluginCommand
	for _, pc := range s.pluginCommands {
		if pc.PluginID != pluginID {
			remaining = append(remaining, pc)
		}
	}
	s.pluginCommands = remaining
}

func (a *App) PluginCommandsForTeam(teamID string) []*model.Command {
	a.Srv().pluginCommandsLock.RLock()
	defer a.Srv().pluginCommandsLock.RUnlock()

	var commands []*model.Command
	for _, pc := range a.Srv().pluginCommands {
		if pc.Command.TeamID == "" || pc.Command.TeamID == teamID {
			commands = append(commands, pc.Command)
		}
	}
	return commands
}

// tryExecutePluginCommand attempts to run a command provided by a plugin based on the given arguments. If no such
// command can be found, returns nil for all arguments.
func (a *App) tryExecutePluginCommand(c *request.Context, args *model.CommandArgs) (*model.Command, *model.CommandResponse, *model.AppError) {
	parts := strings.Split(args.Command, " ")
	trigger := parts[0][1:]
	trigger = strings.ToLower(trigger)

	var matched *PluginCommand
	a.Srv().pluginCommandsLock.RLock()
	for _, pc := range a.Srv().pluginCommands {
		if (pc.Command.TeamID == "" || pc.Command.TeamID == args.TeamID) && pc.Command.Trigger == trigger {
			matched = pc
			break
		}
	}
	a.Srv().pluginCommandsLock.RUnlock()
	if matched == nil {
		return nil, nil, nil
	}

	pluginsEnvironment := a.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		return nil, nil, nil
	}

	// Checking if plugin is working or not
	if err := pluginsEnvironment.PerformHealthCheck(matched.PluginID); err != nil {
		return matched.Command, nil, model.NewAppError("ExecutePluginCommand", "model.plugin_command_error.error.app_error", map[string]interface{}{"Command": trigger}, "err= Plugin has recently crashed: "+matched.PluginID, http.StatusInternalServerError)
	}

	pluginHooks, err := pluginsEnvironment.HooksForPlugin(matched.PluginID)
	if err != nil {
		return matched.Command, nil, model.NewAppError("ExecutePluginCommand", "model.plugin_command.error.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
	}

	for username, userID := range a.MentionsToTeamMembers(args.Command, args.TeamID) {
		args.AddUserMention(username, userID)
	}

	for channelName, channelID := range a.MentionsToPublicChannels(args.Command, args.TeamID) {
		args.AddChannelMention(channelName, channelID)
	}

	response, appErr := pluginHooks.ExecuteCommand(pluginContext(c), args)

	// Checking if plugin crashed after running the command
	if err := pluginsEnvironment.PerformHealthCheck(matched.PluginID); err != nil {
		errMessage := fmt.Sprintf("err= Plugin %s crashed due to /%s command", matched.PluginID, trigger)
		return matched.Command, nil, model.NewAppError("ExecutePluginCommand", "model.plugin_command_crash.error.app_error", map[string]interface{}{"Command": trigger, "PluginId": matched.PluginID}, errMessage, http.StatusInternalServerError)
	}
	return matched.Command, response, appErr
}
