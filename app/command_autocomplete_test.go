// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

func TestParseStaticListArgument(t *testing.T) {
	items := []model.AutocompleteListItem{
		{
			Hint:     "[hint]",
			Item:     "on",
			HelpText: "help",
		},
	}
	fixedArgs := &model.AutocompleteStaticListArg{PossibleArguments: items}

	argument := &model.AutocompleteArg{
		Name:     "", //positional
		HelpText: "some_help",
		Type:     model.AutocompleteArgTypeStaticList,
		Data:     fixedArgs,
	}
	found, _, _, suggestions := parseStaticListArgument(argument, "", "") //TODO understand this!
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "on", Suggestion: "on", Hint: "[hint]", Description: "help"}}, suggestions)

	found, _, _, suggestions = parseStaticListArgument(argument, "", "o")
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "on", Suggestion: "on", Hint: "[hint]", Description: "help"}}, suggestions)

	found, parsed, toBeParsed, _ := parseStaticListArgument(argument, "", "on ")
	assert.False(t, found)
	assert.Equal(t, "on ", parsed)
	assert.Equal(t, "", toBeParsed)

	found, parsed, toBeParsed, _ = parseStaticListArgument(argument, "", "on some")
	assert.False(t, found)
	assert.Equal(t, "on ", parsed)
	assert.Equal(t, "some", toBeParsed)

	fixedArgs.PossibleArguments = append(fixedArgs.PossibleArguments,
		model.AutocompleteListItem{Hint: "[hint]", Item: "off", HelpText: "help"})

	found, _, _, suggestions = parseStaticListArgument(argument, "", "o")
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "on", Suggestion: "on", Hint: "[hint]", Description: "help"}, {Complete: "off", Suggestion: "off", Hint: "[hint]", Description: "help"}}, suggestions)

	found, _, _, suggestions = parseStaticListArgument(argument, "", "of")
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "off", Suggestion: "off", Hint: "[hint]", Description: "help"}}, suggestions)

	found, _, _, suggestions = parseStaticListArgument(argument, "", "o some")
	assert.True(t, found)
	assert.Len(t, suggestions, 0)

	found, parsed, toBeParsed, _ = parseStaticListArgument(argument, "", "off some")
	assert.False(t, found)
	assert.Equal(t, "off ", parsed)
	assert.Equal(t, "some", toBeParsed)

	fixedArgs.PossibleArguments = append(fixedArgs.PossibleArguments,
		model.AutocompleteListItem{Hint: "[hint]", Item: "onon", HelpText: "help"})

	found, _, _, suggestions = parseStaticListArgument(argument, "", "on")
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "on", Suggestion: "on", Hint: "[hint]", Description: "help"}, {Complete: "onon", Suggestion: "onon", Hint: "[hint]", Description: "help"}}, suggestions)

	found, _, _, suggestions = parseStaticListArgument(argument, "bla ", "ono")
	assert.True(t, found)
	assert.Equal(t, []model.AutocompleteSuggestion{{Complete: "bla onon", Suggestion: "onon", Hint: "[hint]", Description: "help"}}, suggestions)

	found, parsed, toBeParsed, _ = parseStaticListArgument(argument, "", "on some")
	assert.False(t, found)
	assert.Equal(t, "on ", parsed)
	assert.Equal(t, "some", toBeParsed)

	found, parsed, toBeParsed, _ = parseStaticListArgument(argument, "", "onon some")
	assert.False(t, found)
	assert.Equal(t, "onon ", parsed)
	assert.Equal(t, "some", toBeParsed)
}

func TestParseInputTextArgument(t *testing.T) {
	argument := &model.AutocompleteArg{
		Name:     "", //positional
		HelpText: "some_help",
		Type:     model.AutocompleteArgTypeText,
		Data:     &model.AutocompleteTextArg{Hint: "hint", Pattern: "pat"},
	}

	found, _, _, suggestion := parseInputTextArgument(argument, "", "")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "", Suggestion: "", Hint: "hint", Description: "some_help"}, suggestion)

	found, _, _, suggestion = parseInputTextArgument(argument, "", " ")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: " ", Suggestion: "", Hint: "hint", Description: "some_help"}, suggestion)

	found, _, _, suggestion = parseInputTextArgument(argument, "", "abc")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "abc", Suggestion: "", Hint: "hint", Description: "some_help"}, suggestion)

	found, _, _, suggestion = parseInputTextArgument(argument, "", "\"abc dfd df ")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "\"abc dfd df ", Suggestion: "", Hint: "hint", Description: "some_help"}, suggestion)

	found, parsed, toBeParsed, _ := parseInputTextArgument(argument, "", "abc efg ")
	assert.False(t, found)
	assert.Equal(t, "abc ", parsed)
	assert.Equal(t, "efg ", toBeParsed)

	found, parsed, toBeParsed, _ = parseInputTextArgument(argument, "", "abc ")
	assert.False(t, found)
	assert.Equal(t, "abc ", parsed)
	assert.Equal(t, "", toBeParsed)

	found, parsed, toBeParsed, _ = parseInputTextArgument(argument, "", "\"abc def\" abc")
	assert.False(t, found)
	assert.Equal(t, "\"abc def\" ", parsed)
	assert.Equal(t, "abc", toBeParsed)

	found, parsed, toBeParsed, _ = parseInputTextArgument(argument, "", "\"abc def\"")
	assert.False(t, found)
	assert.Equal(t, "\"abc def\"", parsed)
	assert.Equal(t, "", toBeParsed)
}

func TestParseNamedArguments(t *testing.T) {
	argument := &model.AutocompleteArg{
		Name:     "name", //named
		HelpText: "some_help",
		Type:     model.AutocompleteArgTypeText,
		Data:     &model.AutocompleteTextArg{Hint: "hint", Pattern: "pat"},
	}

	found, _, _, suggestion := parseNamedArgument(argument, "", "")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "--name ", Suggestion: "--name", Hint: "", Description: "some_help"}, suggestion)

	found, _, _, suggestion = parseNamedArgument(argument, "", " ")
	assert.True(t, found)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: " --name ", Suggestion: "--name", Hint: "", Description: "some_help"}, suggestion)

	found, parsed, toBeParsed, _ := parseNamedArgument(argument, "", "abc")
	assert.False(t, found)
	assert.Equal(t, "abc", parsed)
	assert.Equal(t, "", toBeParsed)

	found, parsed, toBeParsed, suggestion = parseNamedArgument(argument, "", "-")
	assert.True(t, found)
	assert.Equal(t, "-", parsed)
	assert.Equal(t, "", toBeParsed)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "--name ", Suggestion: "--name", Hint: "", Description: "some_help"}, suggestion)

	found, parsed, toBeParsed, suggestion = parseNamedArgument(argument, "", " -")
	assert.True(t, found)
	assert.Equal(t, " -", parsed)
	assert.Equal(t, "", toBeParsed)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: " --name ", Suggestion: "--name", Hint: "", Description: "some_help"}, suggestion)

	found, parsed, toBeParsed, suggestion = parseNamedArgument(argument, "", "--name")
	assert.True(t, found)
	assert.Equal(t, "--name", parsed)
	assert.Equal(t, "", toBeParsed)
	assert.Equal(t, model.AutocompleteSuggestion{Complete: "--name ", Suggestion: "--name", Hint: "", Description: "some_help"}, suggestion)

	found, parsed, toBeParsed, _ = parseNamedArgument(argument, "", "--name bla")
	assert.False(t, found)
	assert.Equal(t, "--name ", parsed)
	assert.Equal(t, "bla", toBeParsed)

	found, parsed, toBeParsed, _ = parseNamedArgument(argument, "", "--name bla gla")
	assert.False(t, found)
	assert.Equal(t, "--name ", parsed)
	assert.Equal(t, "bla gla", toBeParsed)

	found, parsed, toBeParsed, _ = parseNamedArgument(argument, "", "--name \"bla gla\"")
	assert.False(t, found)
	assert.Equal(t, "--name ", parsed)
	assert.Equal(t, "\"bla gla\"", toBeParsed)

	found, parsed, toBeParsed, _ = parseNamedArgument(argument, "", "--name \"bla gla\" ")
	assert.False(t, found)
	assert.Equal(t, "--name ", parsed)
	assert.Equal(t, "\"bla gla\" ", toBeParsed)

	found, parsed, toBeParsed, _ = parseNamedArgument(argument, "", "bla")
	assert.False(t, found)
	assert.Equal(t, "bla", parsed)
	assert.Equal(t, "", toBeParsed)

}

func TestSuggestions(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	jira := createJiraAutocompleteData()
	emptyCmdArgs := &model.CommandArgs{}

	suggestions := th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "ji", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, jira.Trigger, suggestions[0].Complete)
	assert.Equal(t, jira.Trigger, suggestions[0].Suggestion)
	assert.Equal(t, "[command]", suggestions[0].Hint)
	assert.Equal(t, jira.HelpText, suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira crea", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira create", suggestions[0].Complete)
	assert.Equal(t, "create", suggestions[0].Suggestion)
	assert.Equal(t, "[issue text]", suggestions[0].Hint)
	assert.Equal(t, "Create a new Issue", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira c", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "jira create", suggestions[1].Complete)
	assert.Equal(t, "create", suggestions[1].Suggestion)
	assert.Equal(t, "[issue text]", suggestions[1].Hint)
	assert.Equal(t, "Create a new Issue", suggestions[1].Description)
	assert.Equal(t, "jira connect", suggestions[0].Complete)
	assert.Equal(t, "connect", suggestions[0].Suggestion)
	assert.Equal(t, "[url]", suggestions[0].Hint)
	assert.Equal(t, "Connect your Mattermost account to your Jira account", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira create ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira create ", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "[text]", suggestions[0].Hint)
	assert.Equal(t, "This text is optional, will be inserted into the description field", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira create some", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira create some", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "[text]", suggestions[0].Hint)
	assert.Equal(t, "This text is optional, will be inserted into the description field", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira create some text ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 0)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "invalid command", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 0)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira settings notifications o", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "jira settings notifications On", suggestions[0].Complete)
	assert.Equal(t, "On", suggestions[0].Suggestion)
	assert.Equal(t, "Turn notifications on", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "jira settings notifications Off", suggestions[1].Complete)
	assert.Equal(t, "Off", suggestions[1].Suggestion)
	assert.Equal(t, "Turn notifications off", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 11)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira ", model.SystemUserRoleID)
	assert.Len(t, suggestions, 9)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira create \"some issue text", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira create \"some issue text", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "[text]", suggestions[0].Hint)
	assert.Equal(t, "This text is optional, will be inserted into the description field", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira timezone ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira timezone --zone ", suggestions[0].Complete)
	assert.Equal(t, "--zone", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "Set timezone", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira timezone --", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira timezone --zone ", suggestions[0].Complete)
	assert.Equal(t, "--zone", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "Set timezone", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira timezone --zone ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira timezone --zone ", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "[UTC+07:00]", suggestions[0].Hint)
	assert.Equal(t, "Set timezone", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira timezone --zone bla", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "jira timezone --zone bla", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "[UTC+07:00]", suggestions[0].Hint)
	assert.Equal(t, "Set timezone", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{jira}, "", "jira timezone bla", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 0)

	commandA := &model.Command{
		Trigger:          "alice",
		AutocompleteData: model.NewAutocompleteData("alice", "", ""),
	}
	commandB := &model.Command{
		Trigger:          "bob",
		AutocompleteData: model.NewAutocompleteData("bob", "", ""),
	}
	commandC := &model.Command{
		Trigger:          "charles",
		AutocompleteData: model.NewAutocompleteData("charles", "", ""),
	}
	suggestions = th.App.GetSuggestions(th.Context, emptyCmdArgs, []*model.Command{commandB, commandC, commandA}, model.SystemAdminRoleID)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "alice", suggestions[0].Complete)
	assert.Equal(t, "bob", suggestions[1].Complete)
	assert.Equal(t, "charles", suggestions[2].Complete)
}

func TestCommandWithOptionalArgs(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	command := createCommandWithOptionalArgs()
	emptyCmdArgs := &model.CommandArgs{}

	suggestions := th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "comm", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, command.Trigger, suggestions[0].Complete)
	assert.Equal(t, command.Trigger, suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, command.HelpText, suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 4)
	assert.Equal(t, "command subcommand1", suggestions[0].Complete)
	assert.Equal(t, "subcommand1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "command subcommand2", suggestions[1].Complete)
	assert.Equal(t, "subcommand2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)
	assert.Equal(t, "command subcommand3", suggestions[2].Complete)
	assert.Equal(t, "subcommand3", suggestions[2].Suggestion)
	assert.Equal(t, "", suggestions[2].Hint)
	assert.Equal(t, "", suggestions[2].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "command subcommand1 item1", suggestions[0].Complete)
	assert.Equal(t, "item1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "command subcommand1 item2", suggestions[1].Complete)
	assert.Equal(t, "item2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand1 item1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "command subcommand1 item1 --name2 ", suggestions[0].Complete)
	assert.Equal(t, "--name2", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg2", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand1 item1 --name2 bla", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "command subcommand1 item1 --name2 bla", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg2", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "command subcommand2 --name1 ", suggestions[0].Complete)
	assert.Equal(t, "--name1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg1", suggestions[0].Description)
	assert.Equal(t, "command subcommand2 ", suggestions[1].Complete)
	assert.Equal(t, "", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "arg2", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 -", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "command subcommand2 --name1 ", suggestions[0].Complete)
	assert.Equal(t, "--name1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg1", suggestions[0].Description)
	assert.Equal(t, "command subcommand2 -", suggestions[1].Complete)
	assert.Equal(t, "", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "arg2", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 --name1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "command subcommand2 --name1 item1", suggestions[0].Complete)
	assert.Equal(t, "item1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "command subcommand2 --name1 item2", suggestions[1].Complete)
	assert.Equal(t, "item2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)
	assert.Equal(t, "command subcommand2 --name1 ", suggestions[2].Complete)
	assert.Equal(t, "", suggestions[2].Suggestion)
	assert.Equal(t, "", suggestions[2].Hint)
	assert.Equal(t, "arg3", suggestions[2].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 --name1 item", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "command subcommand2 --name1 item1", suggestions[0].Complete)
	assert.Equal(t, "item1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "command subcommand2 --name1 item2", suggestions[1].Complete)
	assert.Equal(t, "item2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)
	assert.Equal(t, "command subcommand2 --name1 item", suggestions[2].Complete)
	assert.Equal(t, "", suggestions[2].Suggestion)
	assert.Equal(t, "", suggestions[2].Hint)
	assert.Equal(t, "arg3", suggestions[2].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 --name1 item1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "command subcommand2 --name1 item1 ", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg2", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 --name1 item1 bla ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "command subcommand2 --name1 item1 bla ", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg3", suggestions[0].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand2 --name1 item1 bla bla ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 0)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand3 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "command subcommand3 --name1 ", suggestions[0].Complete)
	assert.Equal(t, "--name1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg1", suggestions[0].Description)
	assert.Equal(t, "command subcommand3 --name2 ", suggestions[1].Complete)
	assert.Equal(t, "--name2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "arg2", suggestions[1].Description)
	assert.Equal(t, "command subcommand3 --name3 ", suggestions[2].Complete)
	assert.Equal(t, "--name3", suggestions[2].Suggestion)
	assert.Equal(t, "", suggestions[2].Hint)
	assert.Equal(t, "arg3", suggestions[2].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand3 --name", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, "command subcommand3 --name1 ", suggestions[0].Complete)
	assert.Equal(t, "--name1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "arg1", suggestions[0].Description)
	assert.Equal(t, "command subcommand3 --name2 ", suggestions[1].Complete)
	assert.Equal(t, "--name2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "arg2", suggestions[1].Description)
	assert.Equal(t, "command subcommand3 --name3 ", suggestions[2].Complete)
	assert.Equal(t, "--name3", suggestions[2].Suggestion)
	assert.Equal(t, "", suggestions[2].Hint)
	assert.Equal(t, "arg3", suggestions[2].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand3 --name1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "command subcommand3 --name1 item1", suggestions[0].Complete)
	assert.Equal(t, "item1", suggestions[0].Suggestion)
	assert.Equal(t, "", suggestions[0].Hint)
	assert.Equal(t, "", suggestions[0].Description)
	assert.Equal(t, "command subcommand3 --name1 item2", suggestions[1].Complete)
	assert.Equal(t, "item2", suggestions[1].Suggestion)
	assert.Equal(t, "", suggestions[1].Hint)
	assert.Equal(t, "", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand4 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "command subcommand4 item1", suggestions[0].Complete)
	assert.Equal(t, "item1", suggestions[0].Suggestion)
	assert.Equal(t, "(optional)", suggestions[0].Hint)
	assert.Equal(t, "help3", suggestions[0].Description)
	assert.Equal(t, "command subcommand4 ", suggestions[1].Complete)
	assert.Equal(t, "", suggestions[1].Suggestion)
	assert.Equal(t, "message", suggestions[1].Hint)
	assert.Equal(t, "help4", suggestions[1].Description)

	suggestions = th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command}, "", "command subcommand4 item1 ", model.SystemAdminRoleID)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "command subcommand4 item1 ", suggestions[0].Complete)
	assert.Equal(t, "", suggestions[0].Suggestion)
	assert.Equal(t, "message", suggestions[0].Hint)
	assert.Equal(t, "help4", suggestions[0].Description)
}

func createCommandWithOptionalArgs() *model.AutocompleteData {
	command := model.NewAutocompleteData("command", "", "")
	subCommand1 := model.NewAutocompleteData("subcommand1", "", "")
	subCommand1.AddStaticListArgument("arg1", true, []model.AutocompleteListItem{{Item: "item1"}, {Item: "item2"}})
	subCommand1.AddNamedTextArgument("name2", "arg2", "", "", false)
	command.AddCommand(subCommand1)
	subCommand2 := model.NewAutocompleteData("subcommand2", "", "")
	subCommand2.AddNamedStaticListArgument("name1", "arg1", false, []model.AutocompleteListItem{{Item: "item1"}, {Item: "item2"}})
	subCommand2.AddTextArgument("arg2", "", "")
	subCommand2.AddTextArgument("arg3", "", "")
	command.AddCommand(subCommand2)
	subCommand3 := model.NewAutocompleteData("subcommand3", "", "")
	subCommand3.AddNamedStaticListArgument("name1", "arg1", false, []model.AutocompleteListItem{{Item: "item1"}, {Item: "item2"}})
	subCommand3.AddNamedTextArgument("name2", "arg2", "", "", false)
	subCommand3.AddNamedTextArgument("name3", "arg3", "", "", false)
	command.AddCommand(subCommand3)
	subcommand4 := model.NewAutocompleteData("subcommand4", "", "help1")
	subcommand4.AddStaticListArgument("help2", false, []model.AutocompleteListItem{{
		HelpText: "help3",
		Hint:     "(optional)",
		Item:     "item1",
	}})
	subcommand4.AddTextArgument("help4", "message", "")
	command.AddCommand(subcommand4)

	return command
}

// createJiraAutocompleteData will create autocomplete data for jira plugin. For testing purposes only.
func createJiraAutocompleteData() *model.AutocompleteData {
	jira := model.NewAutocompleteData("jira", "[command]", "Available commands: connect, assign, disconnect, create, transition, view, subscribe, settings, install cloud/server, uninstall cloud/server, help")

	connect := model.NewAutocompleteData("connect", "[url]", "Connect your Mattermost account to your Jira account")
	jira.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your Jira account")
	jira.AddCommand(disconnect)

	assign := model.NewAutocompleteData("assign", "[issue]", "Change the assignee of a Jira issue")
	assign.AddDynamicListArgument("List of issues is downloading from your Jira account", "/url/issue-key", true)
	assign.AddDynamicListArgument("List of assignees is downloading from your Jira account", "/url/assignee", true)
	jira.AddCommand(assign)

	create := model.NewAutocompleteData("create", "[issue text]", "Create a new Issue")
	create.AddTextArgument("This text is optional, will be inserted into the description field", "[text]", "")
	jira.AddCommand(create)

	transition := model.NewAutocompleteData("transition", "[issue]", "Change the state of a Jira issue")
	assign.AddDynamicListArgument("List of issues is downloading from your Jira account", "/url/issue-key", true)
	assign.AddDynamicListArgument("List of states is downloading from your Jira account", "/url/states", true)
	jira.AddCommand(transition)

	subscribe := model.NewAutocompleteData("subscribe", "", "Configure the Jira notifications sent to this channel")
	jira.AddCommand(subscribe)

	view := model.NewAutocompleteData("view", "[issue]", "View the details of a specific Jira issue")
	assign.AddDynamicListArgument("List of issues is downloading from your Jira account", "/url/issue-key", true)
	jira.AddCommand(view)

	settings := model.NewAutocompleteData("settings", "", "Update your user settings")
	notifications := model.NewAutocompleteData("notifications", "[on/off]", "Turn notifications on or off")

	items := []model.AutocompleteListItem{
		{
			Hint: "Turn notifications on",
			Item: "On",
		},
		{
			Hint: "Turn notifications off",
			Item: "Off",
		},
	}
	notifications.AddStaticListArgument("Turn notifications on or off", true, items)
	settings.AddCommand(notifications)
	jira.AddCommand(settings)

	timezone := model.NewAutocompleteData("timezone", "", "Update your timezone")
	timezone.AddNamedTextArgument("zone", "Set timezone", "[UTC+07:00]", "", true)
	jira.AddCommand(timezone)

	install := model.NewAutocompleteData("install", "", "Connect Mattermost to a Jira instance")
	install.RoleID = model.SystemAdminRoleID
	cloud := model.NewAutocompleteData("cloud", "", "Connect to a Jira Cloud instance")
	urlPattern := "https?:\\/\\/(www\\.)?[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\\+.~#?&//=]*)"
	cloud.AddTextArgument("input URL of the Jira Cloud instance", "[URL]", urlPattern)
	install.AddCommand(cloud)
	server := model.NewAutocompleteData("server", "", "Connect to a Jira Server or Data Center instance")
	server.AddTextArgument("input URL of the Jira Server or Data Center instance", "[URL]", urlPattern)
	install.AddCommand(server)
	jira.AddCommand(install)

	uninstall := model.NewAutocompleteData("uninstall", "", "Disconnect Mattermost from a Jira instance")
	uninstall.RoleID = model.SystemAdminRoleID
	cloud = model.NewAutocompleteData("cloud", "", "Disconnect from a Jira Cloud instance")
	cloud.AddTextArgument("input URL of the Jira Cloud instance", "[URL]", urlPattern)
	uninstall.AddCommand(cloud)
	server = model.NewAutocompleteData("server", "", "Disconnect from a Jira Server or Data Center instance")
	server.AddTextArgument("input URL of the Jira Server or Data Center instance", "[URL]", urlPattern)
	uninstall.AddCommand(server)
	jira.AddCommand(uninstall)

	return jira
}

func TestDynamicListArgsForBuiltin(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	provider := &testCommandProvider{}
	RegisterCommandProvider(provider)

	command := provider.GetCommand(th.App, nil)
	emptyCmdArgs := &model.CommandArgs{}

	t.Run("GetAutoCompleteListItems", func(t *testing.T) {
		suggestions := th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command.AutocompleteData}, "", "bogus --dynaArg ", model.SystemAdminRoleID)
		assert.Len(t, suggestions, 3)
		assert.Equal(t, "this is hint 1", suggestions[0].Hint)
		assert.Equal(t, "this is hint 2", suggestions[1].Hint)
		assert.Equal(t, "this is hint 3", suggestions[2].Hint)
	})

	t.Run("GetAutoCompleteListItems bad arg", func(t *testing.T) {
		suggestions := th.App.getSuggestions(th.Context, emptyCmdArgs, []*model.AutocompleteData{command.AutocompleteData}, "", "bogus --badArg ", model.SystemAdminRoleID)
		assert.Empty(t, suggestions)
	})
}

type testCommandProvider struct {
}

func (p *testCommandProvider) GetTrigger() string {
	return "bogus"
}

func (p *testCommandProvider) GetCommand(a *App, T i18n.TranslateFunc) *model.Command {
	top := model.NewAutocompleteData(p.GetTrigger(), "[command]", "Just a test.")
	top.AddNamedDynamicListArgument("dynaArg", "A dynamic list", "builtin:bogus", true)

	return &model.Command{
		Trigger:          p.GetTrigger(),
		AutoComplete:     true,
		AutoCompleteDesc: "Test description",
		AutoCompleteHint: "Test hint.",
		DisplayName:      "test display name",
		AutocompleteData: top,
	}
}

func (p *testCommandProvider) DoCommand(a *App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	return &model.CommandResponse{
		Text:         "I do nothing!",
		ResponseType: model.CommandResponseTypeEphemeral,
	}
}

func (p *testCommandProvider) GetAutoCompleteListItems(a *App, commandArgs *model.CommandArgs, arg *model.AutocompleteArg, parsed, toBeParsed string) ([]model.AutocompleteListItem, error) {
	if arg.Name == "dynaArg" {
		return []model.AutocompleteListItem{
			{Item: "item1", Hint: "this is hint 1", HelpText: "This is help text 1."},
			{Item: "item2", Hint: "this is hint 2", HelpText: "This is help text 2."},
			{Item: "item3", Hint: "this is hint 3", HelpText: "This is help text 3."},
		}, nil
	}
	return nil, fmt.Errorf("%s not a dynamic argument", arg.Name)
}
