// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// AutocompleteDynamicArgProvider dynamically provides auto-completion args for built-in commands.
type AutocompleteDynamicArgProvider interface {
	GetAutoCompleteListItems(a *App, commandArgs *model.CommandArgs, arg *model.AutocompleteArg, parsed, toBeParsed string) ([]model.AutocompleteListItem, error)
}

// GetSuggestions returns suggestions for user input.
func (a *App) GetSuggestions(c *request.Context, commandArgs *model.CommandArgs, commands []*model.Command, roleID string) []model.AutocompleteSuggestion {
	sort.Slice(commands, func(i, j int) bool {
		return strings.Compare(strings.ToLower(commands[i].Trigger), strings.ToLower(commands[j].Trigger)) < 0
	})

	autocompleteData := []*model.AutocompleteData{}
	for _, command := range commands {
		if command.AutocompleteData == nil {
			command.AutocompleteData = model.NewAutocompleteData(command.Trigger, command.AutoCompleteHint, command.AutoCompleteDesc)
		}
		autocompleteData = append(autocompleteData, command.AutocompleteData)
	}

	userInput := commandArgs.Command
	suggestions := a.getSuggestions(c, commandArgs, autocompleteData, "", userInput, roleID)
	for i, suggestion := range suggestions {
		for _, command := range commands {
			if strings.HasPrefix(suggestion.Complete, command.Trigger) {
				suggestions[i].IconData = command.AutocompleteIconData
				break
			}
		}
	}

	return suggestions
}

func (a *App) getSuggestions(c *request.Context, commandArgs *model.CommandArgs, commands []*model.AutocompleteData, inputParsed, inputToBeParsed, roleID string) []model.AutocompleteSuggestion {
	suggestions := []model.AutocompleteSuggestion{}
	index := strings.Index(inputToBeParsed, " ")

	if index == -1 { // no space in input
		for _, command := range commands {
			if strings.HasPrefix(command.Trigger, strings.ToLower(inputToBeParsed)) && (command.RoleID == roleID || roleID == model.SystemAdminRoleID || roleID == "") {
				s := model.AutocompleteSuggestion{
					Complete:    inputParsed + command.Trigger,
					Suggestion:  command.Trigger,
					Description: command.HelpText,
					Hint:        command.Hint,
				}
				suggestions = append(suggestions, s)
			}
		}
		return suggestions
	}

	for _, command := range commands {
		if command.Trigger != strings.ToLower(inputToBeParsed[:index]) {
			continue
		}
		if roleID != "" && roleID != model.SystemAdminRoleID && roleID != command.RoleID {
			continue
		}
		toBeParsed := inputToBeParsed[index+1:]
		parsed := inputParsed + inputToBeParsed[:index+1]

		if len(command.Arguments) == 0 {
			// Seek recursively in subcommands
			subSuggestions := a.getSuggestions(c, commandArgs, command.SubCommands, parsed, toBeParsed, roleID)
			suggestions = append(suggestions, subSuggestions...)
			continue
		}

		found, _, _, suggestion := a.parseArguments(c, commandArgs, command.Arguments, parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, suggestion...)
		}
	}

	return suggestions
}

func (a *App) parseArguments(c *request.Context, commandArgs *model.CommandArgs, args []*model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestions []model.AutocompleteSuggestion) {
	if len(args) == 0 {
		return false, parsed, toBeParsed, suggestions
	}

	if args[0].Required {
		found, changedParsed, changedToBeParsed, suggestion := a.parseArgument(c, commandArgs, args[0], parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, suggestion...)
			return true, changedParsed, changedToBeParsed, suggestions
		}
		return a.parseArguments(c, commandArgs, args[1:], changedParsed, changedToBeParsed)
	}

	// Handling optional arguments. Optional argument can be inputted or not,
	// so we have to pase both cases recursively and output combined suggestions.
	foundWithOptional, changedParsedWithOptional, changedToBeParsedWithOptional, suggestionsWithOptional := a.parseArgument(c, commandArgs, args[0], parsed, toBeParsed)
	if foundWithOptional {
		suggestions = append(suggestions, suggestionsWithOptional...)
	} else {
		foundWithOptionalRest, changedParsedWithOptionalRest, changedToBeParsedWithOptionalRest, suggestionsWithOptionalRest := a.parseArguments(c, commandArgs, args[1:], changedParsedWithOptional, changedToBeParsedWithOptional)
		if foundWithOptionalRest {
			suggestions = append(suggestions, suggestionsWithOptionalRest...)
		}
		foundWithOptional = foundWithOptionalRest
		changedParsedWithOptional = changedParsedWithOptionalRest
		changedToBeParsedWithOptional = changedToBeParsedWithOptionalRest
	}

	foundWithoutOptional, changedParsedWithoutOptional, changedToBeParsedWithoutOptional, suggestionsWithoutOptional := a.parseArguments(c, commandArgs, args[1:], parsed, toBeParsed)
	if foundWithoutOptional {
		suggestions = append(suggestions, suggestionsWithoutOptional...)
	}

	// if suggestions were found we can return them
	if foundWithOptional || foundWithoutOptional {
		return true, parsed + toBeParsed, "", suggestions
	}

	// no suggestions found yet, check if optional argument was inputted
	if changedParsedWithOptional != parsed && changedToBeParsedWithOptional != toBeParsed {
		return false, changedParsedWithOptional, changedToBeParsedWithOptional, suggestions
	}

	// no suggestions and optional argument was not inputted
	return foundWithoutOptional, changedParsedWithoutOptional, changedToBeParsedWithoutOptional, suggestions
}

func (a *App) parseArgument(c *request.Context, commandArgs *model.CommandArgs, arg *model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestions []model.AutocompleteSuggestion) {
	if arg.Name != "" { //Parse the --name first
		found, changedParsed, changedToBeParsed, suggestion := parseNamedArgument(arg, parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, suggestion)
			return true, changedParsed, changedToBeParsed, suggestions
		}
		if changedToBeParsed == "" {
			return true, changedParsed, changedToBeParsed, suggestions
		}
		if changedToBeParsed == " " {
			changedToBeParsed = ""
		}
		parsed = changedParsed
		toBeParsed = changedToBeParsed
	}

	if arg.Type == model.AutocompleteArgTypeText {
		found, changedParsed, changedToBeParsed, suggestion := parseInputTextArgument(arg, parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, suggestion)
			return true, changedParsed, changedToBeParsed, suggestions
		}
		parsed = changedParsed
		toBeParsed = changedToBeParsed
	} else if arg.Type == model.AutocompleteArgTypeStaticList {
		found, changedParsed, changedToBeParsed, staticListSuggestions := parseStaticListArgument(arg, parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, staticListSuggestions...)
			return true, changedParsed, changedToBeParsed, suggestions
		}
		parsed = changedParsed
		toBeParsed = changedToBeParsed
	} else if arg.Type == model.AutocompleteArgTypeDynamicList {
		found, changedParsed, changedToBeParsed, dynamicListSuggestions := a.getDynamicListArgument(c, commandArgs, arg, parsed, toBeParsed)
		if found {
			suggestions = append(suggestions, dynamicListSuggestions...)
			return true, changedParsed, changedToBeParsed, suggestions
		}
		parsed = changedParsed
		toBeParsed = changedToBeParsed
	}

	return false, parsed, toBeParsed, suggestions
}

func parseNamedArgument(arg *model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestion model.AutocompleteSuggestion) {
	in := strings.TrimPrefix(toBeParsed, " ")
	namedArg := "--" + arg.Name
	if in == "" { //The user has not started typing the argument.
		return true, parsed + toBeParsed, "", model.AutocompleteSuggestion{Complete: parsed + toBeParsed + namedArg + " ", Suggestion: namedArg, Hint: "", Description: arg.HelpText}
	}
	if strings.HasPrefix(strings.ToLower(namedArg), strings.ToLower(in)) {
		return true, parsed + toBeParsed, "", model.AutocompleteSuggestion{Complete: parsed + toBeParsed + namedArg[len(in):] + " ", Suggestion: namedArg, Hint: "", Description: arg.HelpText}
	}

	if !strings.HasPrefix(strings.ToLower(in), strings.ToLower(namedArg)+" ") {
		return false, parsed + toBeParsed, "", model.AutocompleteSuggestion{}
	}
	if strings.ToLower(in) == strings.ToLower(namedArg)+" " {
		return false, parsed + namedArg + " ", " ", model.AutocompleteSuggestion{}
	}
	return false, parsed + namedArg + " ", in[len(namedArg)+1:], model.AutocompleteSuggestion{}
}

func parseInputTextArgument(arg *model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestion model.AutocompleteSuggestion) {
	in := strings.TrimPrefix(toBeParsed, " ")
	a := arg.Data.(*model.AutocompleteTextArg)
	if in == "" { //The user has not started typing the argument.
		return true, parsed + toBeParsed, "", model.AutocompleteSuggestion{Complete: parsed + toBeParsed, Suggestion: "", Hint: a.Hint, Description: arg.HelpText}
	}
	if in[0] == '"' { //input with multiple words
		indexOfSecondQuote := strings.Index(in[1:], `"`)
		if indexOfSecondQuote == -1 { //typing of the multiple word argument is not finished
			return true, parsed + toBeParsed, "", model.AutocompleteSuggestion{Complete: parsed + toBeParsed, Suggestion: "", Hint: a.Hint, Description: arg.HelpText}
		}
		// this argument is typed already
		offset := 2
		if len(in) > indexOfSecondQuote+2 && in[indexOfSecondQuote+2] == ' ' {
			offset++
		}
		return false, parsed + in[:indexOfSecondQuote+offset], in[indexOfSecondQuote+offset:], model.AutocompleteSuggestion{}
	}
	// input with a single word
	index := strings.Index(in, " ")
	if index == -1 { // typing of the single word argument is not finished
		return true, parsed + toBeParsed, "", model.AutocompleteSuggestion{Complete: parsed + toBeParsed, Suggestion: "", Hint: a.Hint, Description: arg.HelpText}
	}
	// single word argument already typed
	return false, parsed + in[:index+1], in[index+1:], model.AutocompleteSuggestion{}
}

func parseStaticListArgument(arg *model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestions []model.AutocompleteSuggestion) {
	a := arg.Data.(*model.AutocompleteStaticListArg)
	return parseListItems(a.PossibleArguments, parsed, toBeParsed)
}

func (a *App) getDynamicListArgument(c *request.Context, commandArgs *model.CommandArgs, arg *model.AutocompleteArg, parsed, toBeParsed string) (found bool, alreadyParsed string, yetToBeParsed string, suggestions []model.AutocompleteSuggestion) {
	dynamicArg := arg.Data.(*model.AutocompleteDynamicListArg)

	if strings.HasPrefix(dynamicArg.FetchURL, "builtin:") {
		listItems, err := a.getBuiltinDynamicListArgument(commandArgs, arg, parsed, toBeParsed)
		if err != nil {
			a.Log().Error("Can't fetch dynamic list arguments for", mlog.String("url", dynamicArg.FetchURL), mlog.Err(err))
			return false, parsed, toBeParsed, []model.AutocompleteSuggestion{}
		}
		return parseListItems(listItems, parsed, toBeParsed)
	}

	params := url.Values{}
	params.Add("user_input", parsed+toBeParsed)
	params.Add("parsed", parsed)

	// Encode the information normally provided to a plugin slash command handler into the request parameters
	// Encode PluginContext:
	pluginContext := pluginContext(c)
	params.Add("request_id", pluginContext.RequestID)
	params.Add("session_id", pluginContext.SessionID)
	params.Add("ip_address", pluginContext.IDAddress)
	params.Add("accept_language", pluginContext.AcceptLanguage)
	params.Add("user_agent", pluginContext.UserAgent)

	// Encode CommandArgs:
	params.Add("channel_id", commandArgs.ChannelID)
	params.Add("team_id", commandArgs.TeamID)
	params.Add("root_id", commandArgs.RootID)
	params.Add("parent_id", commandArgs.ParentID)
	params.Add("user_id", commandArgs.UserID)
	params.Add("site_url", commandArgs.SiteURL)

	resp, err := a.doPluginRequest(c, "GET", dynamicArg.FetchURL, params, nil)

	if err != nil {
		a.Log().Error("Can't fetch dynamic list arguments for", mlog.String("url", dynamicArg.FetchURL), mlog.Err(err))
		return false, parsed, toBeParsed, []model.AutocompleteSuggestion{}
	}

	listItems := model.AutocompleteStaticListItemsFromJSON(resp.Body)

	return parseListItems(listItems, parsed, toBeParsed)
}

func parseListItems(items []model.AutocompleteListItem, parsed, toBeParsed string) (bool, string, string, []model.AutocompleteSuggestion) {
	in := strings.TrimPrefix(toBeParsed, " ")
	suggestions := []model.AutocompleteSuggestion{}
	maxPrefix := ""
	for _, arg := range items {
		if strings.HasPrefix(strings.ToLower(in), strings.ToLower(arg.Item)+" ") && len(maxPrefix) < len(arg.Item)+1 {
			maxPrefix = arg.Item + " "
		}
	}
	if maxPrefix != "" { //typing of an argument finished
		return false, parsed + in[:len(maxPrefix)], in[len(maxPrefix):], []model.AutocompleteSuggestion{}
	}
	// user has not finished typing static argument
	for _, arg := range items {
		if strings.HasPrefix(strings.ToLower(arg.Item), strings.ToLower(in)) {
			suggestions = append(suggestions, model.AutocompleteSuggestion{Complete: parsed + arg.Item, Suggestion: arg.Item, Hint: arg.Hint, Description: arg.HelpText})
		}
	}
	return true, parsed + toBeParsed, "", suggestions
}

func (a *App) getBuiltinDynamicListArgument(commandArgs *model.CommandArgs, arg *model.AutocompleteArg, parsed, toBeParsed string) ([]model.AutocompleteListItem, error) {
	dynamicArg := arg.Data.(*model.AutocompleteDynamicListArg)
	arr := strings.Split(dynamicArg.FetchURL, ":")
	if len(arr) < 2 {
		return nil, errors.New("dynamic list URL missing built-in command name")
	}
	cmdName := arr[1]

	provider := GetCommandProvider(cmdName)
	if provider == nil {
		return nil, fmt.Errorf("no command provider for %s", cmdName)
	}

	dp, ok := provider.(AutocompleteDynamicArgProvider)
	if !ok {
		return nil, fmt.Errorf("auto-completion not available for built-in command %s", cmdName)
	}

	return dp.GetAutoCompleteListItems(a, commandArgs, arg, parsed, toBeParsed)
}
