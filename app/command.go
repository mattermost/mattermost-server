// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"unicode"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	CmdCustomStatusTrigger = "status"
)

type CommandProvider interface {
	GetTrigger() string
	GetCommand(a *App, T i18n.TranslateFunc) *model.Command
	DoCommand(a *App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse
}

var commandProviders = make(map[string]CommandProvider)

func RegisterCommandProvider(newProvider CommandProvider) {
	commandProviders[newProvider.GetTrigger()] = newProvider
}

func GetCommandProvider(name string) CommandProvider {
	provider, ok := commandProviders[name]
	if ok {
		return provider
	}

	return nil
}

// @openTracingParams teamID, skipSlackParsing
func (a *App) CreateCommandPost(c *request.Context, post *model.Post, teamID string, response *model.CommandResponse, skipSlackParsing bool) (*model.Post, *model.AppError) {
	if skipSlackParsing {
		post.Message = response.Text
	} else {
		post.Message = model.ParseSlackLinksToMarkdown(response.Text)
	}

	post.CreateAt = model.GetMillis()

	if strings.HasPrefix(post.Type, model.PostSystemMessagePrefix) {
		err := model.NewAppError("CreateCommandPost", "api.context.invalid_param.app_error", map[string]interface{}{"Name": "post.type"}, "", http.StatusBadRequest)
		return nil, err
	}

	if response.Attachments != nil {
		model.ParseSlackAttachment(post, response.Attachments)
	}

	if response.ResponseType == model.CommandResponseTypeInChannel {
		return a.CreatePostMissingChannel(c, post, true)
	}

	if (response.ResponseType == "" || response.ResponseType == model.CommandResponseTypeEphemeral) && (response.Text != "" || response.Attachments != nil) {
		post.ParentID = ""
		a.SendEphemeralPost(post.UserID, post)
	}

	return post, nil
}

// @openTracingParams teamID
// previous ListCommands now ListAutocompleteCommands
func (a *App) ListAutocompleteCommands(teamID string, T i18n.TranslateFunc) ([]*model.Command, *model.AppError) {
	commands := make([]*model.Command, 0, 32)
	seen := make(map[string]bool)

	// Disable custom status slash command if the feature or the setting is off
	if !*a.Config().TeamSettings.EnableCustomUserStatuses {
		seen[CmdCustomStatusTrigger] = true
	}

	for _, cmd := range a.PluginCommandsForTeam(teamID) {
		if cmd.AutoComplete && !seen[cmd.Trigger] {
			seen[cmd.Trigger] = true
			commands = append(commands, cmd)
		}
	}

	if *a.Config().ServiceSettings.EnableCommands {
		teamCmds, err := a.Srv().Store.Command().GetByTeam(teamID)
		if err != nil {
			return nil, model.NewAppError("ListAutocompleteCommands", "app.command.listautocompletecommands.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}

		for _, cmd := range teamCmds {
			if cmd.AutoComplete && !seen[cmd.Trigger] {
				cmd.Sanitize()
				seen[cmd.Trigger] = true
				commands = append(commands, cmd)
			}
		}
	}

	for _, value := range commandProviders {
		if cmd := value.GetCommand(a, T); cmd != nil {
			cpy := *cmd
			if cpy.AutoComplete && !seen[cpy.Trigger] {
				cpy.Sanitize()
				seen[cpy.Trigger] = true
				commands = append(commands, &cpy)
			}
		}
	}

	return commands, nil
}

func (a *App) ListTeamCommands(teamID string) ([]*model.Command, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, model.NewAppError("ListTeamCommands", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	teamCmds, err := a.Srv().Store.Command().GetByTeam(teamID)
	if err != nil {
		return nil, model.NewAppError("ListTeamCommands", "app.command.listteamcommands.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return teamCmds, nil
}

func (a *App) ListAllCommands(teamID string, T i18n.TranslateFunc) ([]*model.Command, *model.AppError) {
	commands := make([]*model.Command, 0, 32)
	seen := make(map[string]bool)
	for _, value := range commandProviders {
		if cmd := value.GetCommand(a, T); cmd != nil {
			cpy := *cmd
			if cpy.AutoComplete && !seen[cpy.Trigger] {
				cpy.Sanitize()
				seen[cpy.Trigger] = true
				commands = append(commands, &cpy)
			}
		}
	}

	for _, cmd := range a.PluginCommandsForTeam(teamID) {
		if !seen[cmd.Trigger] {
			seen[cmd.Trigger] = true
			commands = append(commands, cmd)
		}
	}

	if *a.Config().ServiceSettings.EnableCommands {
		teamCmds, err := a.Srv().Store.Command().GetByTeam(teamID)
		if err != nil {
			return nil, model.NewAppError("ListAllCommands", "app.command.listallcommands.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
		for _, cmd := range teamCmds {
			if !seen[cmd.Trigger] {
				cmd.Sanitize()
				seen[cmd.Trigger] = true
				commands = append(commands, cmd)
			}
		}
	}

	return commands, nil
}

// @openTracingParams args
func (a *App) ExecuteCommand(c *request.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := ""
	message := ""
	index := strings.IndexFunc(args.Command, unicode.IsSpace)
	if index != -1 {
		trigger = args.Command[:index]
		message = args.Command[index+1:]
	} else {
		trigger = args.Command
	}
	trigger = strings.ToLower(trigger)
	if !strings.HasPrefix(trigger, "/") {
		return nil, model.NewAppError("command", "api.command.execute_command.format.app_error", map[string]interface{}{"Trigger": trigger}, "", http.StatusBadRequest)
	}
	trigger = strings.TrimPrefix(trigger, "/")

	clientTriggerID, triggerID, appErr := model.GenerateTriggerID(args.UserID, a.AsymmetricSigningKey())
	if appErr != nil {
		mlog.Warn("error occurred in generating trigger Id for a user ", mlog.Err(appErr))
	}

	args.TriggerID = triggerID

	// Plugins can override built in and custom commands
	cmd, response, appErr := a.tryExecutePluginCommand(c, args)
	if appErr != nil {
		return nil, appErr
	} else if cmd != nil && response != nil {
		response.TriggerID = clientTriggerID
		return a.HandleCommandResponse(c, cmd, args, response, true)
	}

	// Custom commands can override built ins
	cmd, response, appErr = a.tryExecuteCustomCommand(args, trigger, message)
	if appErr != nil {
		return nil, appErr
	} else if cmd != nil && response != nil {
		response.TriggerID = clientTriggerID
		return a.HandleCommandResponse(c, cmd, args, response, false)
	}

	cmd, response = a.tryExecuteBuiltInCommand(c, args, trigger, message)
	if cmd != nil && response != nil {
		return a.HandleCommandResponse(c, cmd, args, response, true)
	}

	return nil, model.NewAppError("command", "api.command.execute_command.not_found.app_error", map[string]interface{}{"Trigger": trigger}, "", http.StatusNotFound)
}

// MentionsToTeamMembers returns all the @ mentions found in message that
// belong to users in the specified team, linking them to their users
func (a *App) MentionsToTeamMembers(message, teamID string) model.UserMentionMap {
	type mentionMapItem struct {
		Name string
		ID   string
	}

	possibleMentions := model.PossibleAtMentions(message)
	mentionChan := make(chan *mentionMapItem, len(possibleMentions))

	var wg sync.WaitGroup
	for _, mention := range possibleMentions {
		wg.Add(1)
		go func(mention string) {
			defer wg.Done()
			user, nErr := a.Srv().Store.User().GetByUsername(mention)

			var nfErr *store.ErrNotFound
			if nErr != nil && !errors.As(nErr, &nfErr) {
				mlog.Warn("Failed to retrieve user @"+mention, mlog.Err(nErr))
				return
			}

			// If it's a http.StatusNotFound error, check for usernames in substrings
			// without trailing punctuation
			if nErr != nil {
				trimmed, ok := model.TrimUsernameSpecialChar(mention)
				for ; ok; trimmed, ok = model.TrimUsernameSpecialChar(trimmed) {
					userFromTrimmed, nErr := a.Srv().Store.User().GetByUsername(trimmed)
					if nErr != nil && !errors.As(nErr, &nfErr) {
						return
					}

					if nErr != nil {
						continue
					}

					_, err := a.GetTeamMember(teamID, userFromTrimmed.ID)
					if err != nil {
						// The user is not in the team, so we should ignore it
						return
					}

					mentionChan <- &mentionMapItem{trimmed, userFromTrimmed.ID}
					return
				}

				return
			}

			_, err := a.GetTeamMember(teamID, user.ID)
			if err != nil {
				// The user is not in the team, so we should ignore it
				return
			}

			mentionChan <- &mentionMapItem{mention, user.ID}
		}(mention)
	}

	wg.Wait()
	close(mentionChan)

	atMentionMap := make(model.UserMentionMap)
	for mention := range mentionChan {
		atMentionMap[mention.Name] = mention.ID
	}

	return atMentionMap
}

// MentionsToPublicChannels returns all the mentions to public channels,
// linking them to their channels
func (a *App) MentionsToPublicChannels(message, teamID string) model.ChannelMentionMap {
	type mentionMapItem struct {
		Name string
		ID   string
	}

	channelMentions := model.ChannelMentions(message)
	mentionChan := make(chan *mentionMapItem, len(channelMentions))

	var wg sync.WaitGroup
	for _, channelName := range channelMentions {
		wg.Add(1)
		go func(channelName string) {
			defer wg.Done()
			channel, err := a.GetChannelByName(channelName, teamID, false)
			if err != nil {
				return
			}

			if !channel.IsOpen() {
				return
			}

			mentionChan <- &mentionMapItem{channelName, channel.ID}
		}(channelName)
	}

	wg.Wait()
	close(mentionChan)

	channelMentionMap := make(model.ChannelMentionMap)
	for mention := range mentionChan {
		channelMentionMap[mention.Name] = mention.ID
	}

	return channelMentionMap
}

// tryExecuteBuiltInCommand attempts to run a built in command based on the given arguments. If no such command can be
// found, returns nil for all arguments.
func (a *App) tryExecuteBuiltInCommand(c *request.Context, args *model.CommandArgs, trigger string, message string) (*model.Command, *model.CommandResponse) {
	provider := GetCommandProvider(trigger)
	if provider == nil {
		return nil, nil
	}

	cmd := provider.GetCommand(a, args.T)
	if cmd == nil {
		return nil, nil
	}

	return cmd, provider.DoCommand(a, c, args, message)
}

// tryExecuteCustomCommand attempts to run a custom command based on the given arguments. If no such command can be
// found, returns nil for all arguments.
func (a *App) tryExecuteCustomCommand(args *model.CommandArgs, trigger string, message string) (*model.Command, *model.CommandResponse, *model.AppError) {
	// Handle custom commands
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, nil, model.NewAppError("ExecuteCommand", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	chanChan := make(chan store.StoreResult, 1)
	go func() {
		channel, err := a.Srv().Store.Channel().Get(args.ChannelID, true)
		chanChan <- store.StoreResult{Data: channel, NErr: err}
		close(chanChan)
	}()

	teamChan := make(chan store.StoreResult, 1)
	go func() {
		team, err := a.Srv().Store.Team().Get(args.TeamID)
		teamChan <- store.StoreResult{Data: team, NErr: err}
		close(teamChan)
	}()

	userChan := make(chan store.StoreResult, 1)
	go func() {
		user, err := a.Srv().Store.User().Get(context.Background(), args.UserID)
		userChan <- store.StoreResult{Data: user, NErr: err}
		close(userChan)
	}()

	teamCmds, err := a.Srv().Store.Command().GetByTeam(args.TeamID)
	if err != nil {
		return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.command.tryexecutecustomcommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}

	tr := <-teamChan
	if tr.NErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(tr.NErr, &nfErr):
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.team.get.find.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.team.get.finding.app_error", nil, tr.NErr.Error(), http.StatusInternalServerError)
		}
	}
	team := tr.Data.(*model.Team)

	ur := <-userChan
	if ur.NErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(ur.NErr, &nfErr):
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.user.get.app_error", nil, ur.NErr.Error(), http.StatusInternalServerError)
		}
	}
	user := ur.Data.(*model.User)

	cr := <-chanChan
	if cr.NErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(cr.NErr, &nfErr):
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.channel.get.existing.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, nil, model.NewAppError("tryExecuteCustomCommand", "app.channel.get.find.app_error", nil, cr.NErr.Error(), http.StatusInternalServerError)
		}
	}
	channel := cr.Data.(*model.Channel)

	var cmd *model.Command

	for _, teamCmd := range teamCmds {
		if trigger == teamCmd.Trigger {
			cmd = teamCmd
		}
	}

	if cmd == nil {
		return nil, nil, nil
	}

	mlog.Debug("Executing command", mlog.String("command", trigger), mlog.String("user_id", args.UserID))

	p := url.Values{}
	p.Set("token", cmd.Token)

	p.Set("team_id", cmd.TeamID)
	p.Set("team_domain", team.Name)

	p.Set("channel_id", args.ChannelID)
	p.Set("channel_name", channel.Name)

	p.Set("user_id", args.UserID)
	p.Set("user_name", user.Username)

	p.Set("command", "/"+trigger)
	p.Set("text", message)

	p.Set("trigger_id", args.TriggerID)

	userMentionMap := a.MentionsToTeamMembers(message, team.ID)
	for key, values := range userMentionMap.ToURLValues() {
		p[key] = values
	}

	channelMentionMap := a.MentionsToPublicChannels(message, team.ID)
	for key, values := range channelMentionMap.ToURLValues() {
		p[key] = values
	}

	hook, appErr := a.CreateCommandWebhook(cmd.ID, args)
	if appErr != nil {
		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed.app_error", map[string]interface{}{"Trigger": trigger}, appErr.Error(), http.StatusInternalServerError)
	}
	p.Set("response_url", args.SiteURL+"/hooks/commands/"+hook.ID)

	return a.DoCommandRequest(cmd, p)
}

func (a *App) DoCommandRequest(cmd *model.Command, p url.Values) (*model.Command, *model.CommandResponse, *model.AppError) {
	// Prepare the request
	var req *http.Request
	var err error
	if cmd.Method == model.CommandMethodGet {
		req, err = http.NewRequest(http.MethodGet, cmd.URL, nil)
	} else {
		req, err = http.NewRequest(http.MethodPost, cmd.URL, strings.NewReader(p.Encode()))
	}

	if err != nil {
		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed.app_error", map[string]interface{}{"Trigger": cmd.Trigger}, err.Error(), http.StatusInternalServerError)
	}

	if cmd.Method == model.CommandMethodGet {
		if req.URL.RawQuery != "" {
			req.URL.RawQuery += "&"
		}
		req.URL.RawQuery += p.Encode()
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+cmd.Token)
	if cmd.Method == model.CommandMethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Send the request
	resp, err := a.HTTPService().MakeClient(false).Do(req)
	if err != nil {
		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed.app_error", map[string]interface{}{"Trigger": cmd.Trigger}, err.Error(), http.StatusInternalServerError)
	}

	defer resp.Body.Close()

	// Handle the response
	body := io.LimitReader(resp.Body, MaxIntegrationResponseSize)

	if resp.StatusCode != http.StatusOK {
		// Ignore the error below because the resulting string will just be the empty string if bodyBytes is nil
		bodyBytes, _ := ioutil.ReadAll(body)

		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed_resp.app_error", map[string]interface{}{"Trigger": cmd.Trigger, "Status": resp.Status}, string(bodyBytes), http.StatusInternalServerError)
	}

	response, err := model.CommandResponseFromHTTPBody(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed.app_error", map[string]interface{}{"Trigger": cmd.Trigger}, err.Error(), http.StatusInternalServerError)
	} else if response == nil {
		return cmd, nil, model.NewAppError("command", "api.command.execute_command.failed_empty.app_error", map[string]interface{}{"Trigger": cmd.Trigger}, "", http.StatusInternalServerError)
	}

	return cmd, response, nil
}

func (a *App) HandleCommandResponse(c *request.Context, command *model.Command, args *model.CommandArgs, response *model.CommandResponse, builtIn bool) (*model.CommandResponse, *model.AppError) {
	trigger := ""
	if args.Command != "" {
		parts := strings.Split(args.Command, " ")
		trigger = parts[0][1:]
		trigger = strings.ToLower(trigger)
	}

	var lastError *model.AppError
	_, err := a.HandleCommandResponsePost(c, command, args, response, builtIn)

	if err != nil {
		mlog.Debug("Error occurred in handling command response post", mlog.Err(err))
		lastError = err
	}

	if response.ExtraResponses != nil {
		for _, resp := range response.ExtraResponses {
			_, err := a.HandleCommandResponsePost(c, command, args, resp, builtIn)

			if err != nil {
				mlog.Debug("Error occurred in handling command response post", mlog.Err(err))
				lastError = err
			}
		}
	}

	if lastError != nil {
		return response, model.NewAppError("command", "api.command.execute_command.create_post_failed.app_error", map[string]interface{}{"Trigger": trigger}, "", http.StatusInternalServerError)
	}

	return response, nil
}

func (a *App) HandleCommandResponsePost(c *request.Context, command *model.Command, args *model.CommandArgs, response *model.CommandResponse, builtIn bool) (*model.Post, *model.AppError) {
	post := &model.Post{}
	post.ChannelID = args.ChannelID
	post.RootID = args.RootID
	post.ParentID = args.ParentID
	post.UserID = args.UserID
	post.Type = response.Type
	post.SetProps(response.Props)

	if response.ChannelID != "" {
		_, err := a.GetChannelMember(context.Background(), response.ChannelID, args.UserID)
		if err != nil {
			err = model.NewAppError("HandleCommandResponsePost", "api.command.command_post.forbidden.app_error", nil, err.Error(), http.StatusForbidden)
			return nil, err
		}
		post.ChannelID = response.ChannelID
	}

	isBotPost := !builtIn

	if *a.Config().ServiceSettings.EnablePostUsernameOverride {
		if command.Username != "" {
			post.AddProp("override_username", command.Username)
			isBotPost = true
		} else if response.Username != "" {
			post.AddProp("override_username", response.Username)
			isBotPost = true
		}
	}

	if *a.Config().ServiceSettings.EnablePostIconOverride {
		if command.IconURL != "" {
			post.AddProp("override_icon_url", command.IconURL)
			isBotPost = true
		} else if response.IconURL != "" {
			post.AddProp("override_icon_url", response.IconURL)
			isBotPost = true
		} else {
			post.AddProp("override_icon_url", "")
		}
	}

	if isBotPost {
		post.AddProp("from_webhook", "true")
	}

	// Process Slack text replacements if the response does not contain "skip_slack_parsing": true.
	if !response.SkipSlackParsing {
		response.Text = a.ProcessSlackText(response.Text)
		response.Attachments = a.ProcessSlackAttachments(response.Attachments)
	}

	if _, err := a.CreateCommandPost(c, post, args.TeamID, response, response.SkipSlackParsing); err != nil {
		return post, err
	}

	return post, nil
}

func (a *App) CreateCommand(cmd *model.Command) (*model.Command, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, model.NewAppError("CreateCommand", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	return a.createCommand(cmd)
}

func (a *App) createCommand(cmd *model.Command) (*model.Command, *model.AppError) {
	cmd.Trigger = strings.ToLower(cmd.Trigger)

	teamCmds, err := a.Srv().Store.Command().GetByTeam(cmd.TeamID)
	if err != nil {
		return nil, model.NewAppError("CreateCommand", "app.command.createcommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, existingCommand := range teamCmds {
		if cmd.Trigger == existingCommand.Trigger {
			return nil, model.NewAppError("CreateCommand", "api.command.duplicate_trigger.app_error", nil, "", http.StatusBadRequest)
		}
	}

	for _, builtInProvider := range commandProviders {
		builtInCommand := builtInProvider.GetCommand(a, i18n.T)
		if builtInCommand != nil && cmd.Trigger == builtInCommand.Trigger {
			return nil, model.NewAppError("CreateCommand", "api.command.duplicate_trigger.app_error", nil, "", http.StatusBadRequest)
		}
	}

	command, nErr := a.Srv().Store.Command().Save(cmd)
	if nErr != nil {
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("CreateCommand", "app.command.createcommand.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	return command, nil
}

func (a *App) GetCommand(commandID string) (*model.Command, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, model.NewAppError("GetCommand", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	command, err := a.Srv().Store.Command().Get(commandID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("SqlCommandStore.Get", "store.sql_command.get.missing.app_error", map[string]interface{}{"command_id": commandID}, "", http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetCommand", "app.command.getcommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return command, nil
}

func (a *App) UpdateCommand(oldCmd, updatedCmd *model.Command) (*model.Command, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, model.NewAppError("UpdateCommand", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	updatedCmd.Trigger = strings.ToLower(updatedCmd.Trigger)
	updatedCmd.ID = oldCmd.ID
	updatedCmd.Token = oldCmd.Token
	updatedCmd.CreateAt = oldCmd.CreateAt
	updatedCmd.UpdateAt = model.GetMillis()
	updatedCmd.DeleteAt = oldCmd.DeleteAt
	updatedCmd.CreatorID = oldCmd.CreatorID
	updatedCmd.PluginID = oldCmd.PluginID
	updatedCmd.TeamID = oldCmd.TeamID

	command, err := a.Srv().Store.Command().Update(updatedCmd)
	if err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("SqlCommandStore.Update", "store.sql_command.update.missing.app_error", map[string]interface{}{"command_id": updatedCmd.ID}, "", http.StatusNotFound)
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("UpdateCommand", "app.command.updatecommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return command, nil
}

func (a *App) MoveCommand(team *model.Team, command *model.Command) *model.AppError {
	command.TeamID = team.ID

	_, err := a.Srv().Store.Command().Update(command)
	if err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return model.NewAppError("SqlCommandStore.Update", "store.sql_command.update.missing.app_error", map[string]interface{}{"command_id": command.ID}, "", http.StatusNotFound)
		case errors.As(err, &appErr):
			return appErr
		default:
			return model.NewAppError("MoveCommand", "app.command.movecommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return nil
}

func (a *App) RegenCommandToken(cmd *model.Command) (*model.Command, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCommands {
		return nil, model.NewAppError("RegenCommandToken", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	cmd.Token = model.NewID()

	command, err := a.Srv().Store.Command().Update(cmd)
	if err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("SqlCommandStore.Update", "store.sql_command.update.missing.app_error", map[string]interface{}{"command_id": cmd.ID}, "", http.StatusNotFound)
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("RegenCommandToken", "app.command.regencommandtoken.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return command, nil
}

func (a *App) DeleteCommand(commandID string) *model.AppError {
	if !*a.Config().ServiceSettings.EnableCommands {
		return model.NewAppError("DeleteCommand", "api.command.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	err := a.Srv().Store.Command().Delete(commandID, model.GetMillis())
	if err != nil {
		return model.NewAppError("DeleteCommand", "app.command.deletecommand.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}
