// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

var usage = `Mattermost testing commands to help configure the system

	COMMANDS:

	Setup - Creates a testing environment in current team.
		/test setup [teams] [fuzz] <Num Channels> <Num Users> <NumPosts>

		Example:
		/test setup teams fuzz 10 20 50

	Users - Add a specified number of random users with fuzz text to current team.
		/test users [fuzz] <Min Users> <Max Users>

		Example:
			/test users fuzz 5 10

	Channels - Add a specified number of random channels with fuzz text to current team.
		/test channels [fuzz] <Min Channels> <Max Channels>

		Example:
			/test channels fuzz 5 10

	ThreadedPost - create a large threaded post
        /test threaded_post

	Posts - Add some random posts with fuzz text to current channel.
		/test posts [fuzz] <Min Posts> <Max Posts> <Max Images>

		Example:
			/test posts fuzz 5 10 3

	Post - Add post to a channel as another user.
		/test post u=@username p=passwd c=~channelname t=teamname "message"

		Example:
			/test post u=@user-1 p=user-1 c=~town-square t=ad-1 "message"

	Url - Add a post containing the text from a given url to current channel.
		/test url

		Example:
			/test http://www.example.com/sample_file.md

	Json - Add a post using the JSON file as payload to the current channel.
	        /test json url

		Example
		/test json http://www.example.com/sample_body.json

`

const (
	CmdTest = "test"
)

var (
	userRE    = regexp.MustCompile(`u=@([^\s]+)`)
	passwdRE  = regexp.MustCompile(`p=([^\s]+)`)
	teamRE    = regexp.MustCompile(`t=([^\s]+)`)
	channelRE = regexp.MustCompile(`c=~([^\s]+)`)
	messageRE = regexp.MustCompile(`"(.*)"`)
)

type LoadTestProvider struct {
}

func init() {
	app.RegisterCommandProvider(&LoadTestProvider{})
}

func (*LoadTestProvider) GetTrigger() string {
	return CmdTest
}

func (*LoadTestProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	if !*a.Config().ServiceSettings.EnableTesting {
		return nil
	}
	return &model.Command{
		Trigger:          CmdTest,
		AutoComplete:     false,
		AutoCompleteDesc: "Debug Load Testing",
		AutoCompleteHint: "help",
		DisplayName:      "test",
	}
}

func (lt *LoadTestProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	commandResponse, err := lt.doCommand(a, c, args, message)
	if err != nil {
		mlog.Error("failed command /"+CmdTest, mlog.Err(err))
	}

	return commandResponse
}

func (lt *LoadTestProvider) doCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	//This command is only available when EnableTesting is true
	if !*a.Config().ServiceSettings.EnableTesting {
		return &model.CommandResponse{}, nil
	}

	if strings.HasPrefix(message, "setup") {
		return lt.SetupCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "users") {
		return lt.UsersCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "activate_user") {
		return lt.ActivateUserCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "deactivate_user") {
		return lt.DeActivateUserCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "channels") {
		return lt.ChannelsCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "posts") {
		return lt.PostsCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "post") {
		return lt.PostCommand(a, args, message)
	}

	if strings.HasPrefix(message, "threaded_post") {
		return lt.ThreadedPostCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "url") {
		return lt.URLCommand(a, c, args, message)
	}

	if strings.HasPrefix(message, "json") {
		return lt.JSONCommand(a, c, args, message)
	}

	return lt.HelpCommand(args, message), nil
}

func (*LoadTestProvider) HelpCommand(args *model.CommandArgs, message string) *model.CommandResponse {
	return &model.CommandResponse{Text: usage, ResponseType: model.CommandResponseTypeEphemeral}
}

func (*LoadTestProvider) SetupCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	tokens := strings.Fields(strings.TrimPrefix(message, "setup"))
	doTeams := contains(tokens, "teams")
	doFuzz := contains(tokens, "fuzz")

	numArgs := 0
	if doTeams {
		numArgs++
	}
	if doFuzz {
		numArgs++
	}

	var numTeams int
	var numChannels int
	var numUsers int
	var numPosts int

	// Defaults
	numTeams = 10
	numChannels = 10
	numUsers = 10
	numPosts = 10

	if doTeams {
		if (len(tokens) - numArgs) >= 4 {
			numTeams, _ = strconv.Atoi(tokens[numArgs+0])
			numChannels, _ = strconv.Atoi(tokens[numArgs+1])
			numUsers, _ = strconv.Atoi(tokens[numArgs+2])
			numPosts, _ = strconv.Atoi(tokens[numArgs+3])
		}
	} else {
		if (len(tokens) - numArgs) >= 3 {
			numChannels, _ = strconv.Atoi(tokens[numArgs+0])
			numUsers, _ = strconv.Atoi(tokens[numArgs+1])
			numPosts, _ = strconv.Atoi(tokens[numArgs+2])
		}
	}
	client := model.NewAPIv4Client(args.SiteURL)

	if doTeams {
		if err := CreateBasicUser(a, client); err != nil {
			return &model.CommandResponse{Text: "Failed to create testing environment", ResponseType: model.CommandResponseTypeEphemeral}, err
		}
		_, resp := client.Login(BTestUserEmail, BTestUserPassword)
		if resp.Error != nil {
			return &model.CommandResponse{Text: "Failed to create testing environment", ResponseType: model.CommandResponseTypeEphemeral}, resp.Error
		}
		environment, err := CreateTestEnvironmentWithTeams(
			a,
			c,
			client,
			utils.Range{Begin: numTeams, End: numTeams},
			utils.Range{Begin: numChannels, End: numChannels},
			utils.Range{Begin: numUsers, End: numUsers},
			utils.Range{Begin: numPosts, End: numPosts},
			doFuzz)
		if err != nil {
			return &model.CommandResponse{Text: "Failed to create testing environment", ResponseType: model.CommandResponseTypeEphemeral}, err
		}

		mlog.Info("Testing environment created")
		for i := 0; i < len(environment.Teams); i++ {
			mlog.Info("Team Created: " + environment.Teams[i].Name)
			mlog.Info("\t User to login: " + environment.Environments[i].Users[0].Email + ", " + UserPassword)
		}
	} else {
		team, err := a.Srv().Store.Team().Get(args.TeamID)
		if err != nil {
			return &model.CommandResponse{Text: "Failed to create testing environment", ResponseType: model.CommandResponseTypeEphemeral}, err
		}

		CreateTestEnvironmentInTeam(
			a,
			c,
			client,
			team,
			utils.Range{Begin: numChannels, End: numChannels},
			utils.Range{Begin: numUsers, End: numUsers},
			utils.Range{Begin: numPosts, End: numPosts},
			doFuzz)
	}

	return &model.CommandResponse{Text: "Created environment", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) ActivateUserCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	user_id := strings.TrimSpace(strings.TrimPrefix(message, "activate_user"))
	if err := a.UpdateUserActive(c, user_id, true); err != nil {
		return &model.CommandResponse{Text: "Failed to activate user", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	return &model.CommandResponse{Text: "Activated user", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) DeActivateUserCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	user_id := strings.TrimSpace(strings.TrimPrefix(message, "deactivate_user"))
	if err := a.UpdateUserActive(c, user_id, false); err != nil {
		return &model.CommandResponse{Text: "Failed to deactivate user", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	return &model.CommandResponse{Text: "DeActivated user", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) UsersCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	cmd := strings.TrimSpace(strings.TrimPrefix(message, "users"))

	doFuzz := false
	if strings.Index(cmd, "fuzz") == 0 {
		doFuzz = true
		cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "fuzz"))
	}

	usersr, ok := parseRange(cmd, "")
	if !ok {
		usersr = utils.Range{Begin: 2, End: 5}
	}

	team, err := a.Srv().Store.Team().Get(args.TeamID)
	if err != nil {
		return &model.CommandResponse{Text: "Failed to add users", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	client := model.NewAPIv4Client(args.SiteURL)
	userCreator := NewAutoUserCreator(a, client, team)
	userCreator.Fuzzy = doFuzz
	if _, err := userCreator.CreateTestUsers(c, usersr); err != nil {
		return &model.CommandResponse{Text: "Failed to add users: " + err.Error(), ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	return &model.CommandResponse{Text: "Added users", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) ChannelsCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	cmd := strings.TrimSpace(strings.TrimPrefix(message, "channels"))

	doFuzz := false
	if strings.Index(cmd, "fuzz") == 0 {
		doFuzz = true
		cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "fuzz"))
	}

	channelsr, ok := parseRange(cmd, "")
	if !ok {
		channelsr = utils.Range{Begin: 2, End: 5}
	}

	team, err := a.Srv().Store.Team().Get(args.TeamID)
	if err != nil {
		return &model.CommandResponse{Text: "Failed to add channels", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	channelCreator := NewAutoChannelCreator(a, team, args.UserID)
	channelCreator.Fuzzy = doFuzz
	if _, err := channelCreator.CreateTestChannels(c, channelsr); err != nil {
		return &model.CommandResponse{Text: "Failed to create test channels: " + err.Error(), ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	return &model.CommandResponse{Text: "Added channels", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) ThreadedPostCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	var usernames []string
	options := &model.UserGetOptions{InTeamID: args.TeamID, Page: 0, PerPage: 1000}
	if profileUsers, err := a.Srv().Store.User().GetProfiles(options); err == nil {
		usernames = make([]string, len(profileUsers))
		i := 0
		for _, userprof := range profileUsers {
			usernames[i] = userprof.Username
			i++
		}
	}

	testPoster := NewAutoPostCreator(a, args.ChannelID, args.UserID)
	testPoster.Fuzzy = true
	testPoster.Users = usernames
	rpost, err2 := testPoster.CreateRandomPost(c)
	if err2 != nil {
		return &model.CommandResponse{Text: "Failed to create a post", ResponseType: model.CommandResponseTypeEphemeral}, err2
	}
	for i := 0; i < 1000; i++ {
		testPoster.CreateRandomPostNested(c, rpost.ID, rpost.ID)
	}

	return &model.CommandResponse{Text: "Added threaded post", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) PostsCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	cmd := strings.TrimSpace(strings.TrimPrefix(message, "posts"))

	doFuzz := false
	if strings.Index(cmd, "fuzz") == 0 {
		doFuzz = true
		cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "fuzz"))
	}

	postsr, ok := parseRange(cmd, "")
	if !ok {
		postsr = utils.Range{Begin: 20, End: 30}
	}

	tokens := strings.Fields(cmd)
	rimages := utils.Range{Begin: 0, End: 0}
	if len(tokens) >= 3 {
		if numImages, err := strconv.Atoi(tokens[2]); err == nil {
			rimages = utils.Range{Begin: numImages, End: numImages}
		}
	}

	var usernames []string
	options := &model.UserGetOptions{InTeamID: args.TeamID, Page: 0, PerPage: 1000}
	if profileUsers, err := a.Srv().Store.User().GetProfiles(options); err == nil {
		usernames = make([]string, len(profileUsers))
		i := 0
		for _, userprof := range profileUsers {
			usernames[i] = userprof.Username
			i++
		}
	}

	testPoster := NewAutoPostCreator(a, args.ChannelID, args.UserID)
	testPoster.Fuzzy = doFuzz
	testPoster.Users = usernames

	numImages := utils.RandIntFromRange(rimages)
	numPosts := utils.RandIntFromRange(postsr)
	for i := 0; i < numPosts; i++ {
		testPoster.HasImage = (i < numImages)
		_, err := testPoster.CreateRandomPost(c)
		if err != nil {
			return &model.CommandResponse{Text: "Failed to add posts", ResponseType: model.CommandResponseTypeEphemeral}, err
		}

	}

	return &model.CommandResponse{Text: "Added posts", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func getMatch(re *regexp.Regexp, text string) string {
	if match := re.FindStringSubmatch(text); match != nil {
		return match[1]
	}

	return ""
}

func (*LoadTestProvider) PostCommand(a *app.App, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	textMessage := getMatch(messageRE, message)
	if textMessage == "" {
		return &model.CommandResponse{Text: "No message to post", ResponseType: model.CommandResponseTypeEphemeral}, nil
	}

	teamName := getMatch(teamRE, message)
	team, err := a.GetTeamByName(teamName)
	if err != nil {
		return &model.CommandResponse{Text: "Failed to get a team", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	channelName := getMatch(channelRE, message)
	channel, err := a.GetChannelByName(channelName, team.ID, true)
	if err != nil {
		return &model.CommandResponse{Text: "Failed to get a channel", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	passwd := getMatch(passwdRE, message)
	username := getMatch(userRE, message)
	user, err := a.GetUserByUsername(username)
	if err != nil {
		return &model.CommandResponse{Text: "Failed to get a user", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	client := model.NewAPIv4Client(args.SiteURL)
	_, resp := client.LoginByID(user.ID, passwd)
	if resp.Error != nil {
		return &model.CommandResponse{Text: "Failed to login a user", ResponseType: model.CommandResponseTypeEphemeral}, resp.Error
	}

	post := &model.Post{
		ChannelID: channel.ID,
		Message:   textMessage,
	}
	_, resp = client.CreatePost(post)
	if resp.Error != nil {
		return &model.CommandResponse{Text: "Failed to create a post", ResponseType: model.CommandResponseTypeEphemeral}, resp.Error
	}

	return &model.CommandResponse{Text: "Added a post to " + channel.DisplayName, ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) URLCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	url := strings.TrimSpace(strings.TrimPrefix(message, "url"))
	if url == "" {
		return &model.CommandResponse{Text: "Command must contain a url", ResponseType: model.CommandResponseTypeEphemeral}, nil
	}

	// provide a shortcut to easily access tests stored in doc/developer/tests
	if !strings.HasPrefix(url, "http") {
		url = "https://raw.githubusercontent.com/mattermost/mattermost-server/master/tests/" + url

		if path.Ext(url) == "" {
			url += ".md"
		}
	}

	r, err := http.Get(url)
	if err != nil {
		return &model.CommandResponse{Text: "Unable to get file", ResponseType: model.CommandResponseTypeEphemeral}, err
	}
	defer func() {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()
	}()

	if r.StatusCode > 400 {
		return &model.CommandResponse{Text: "Unable to get file", ResponseType: model.CommandResponseTypeEphemeral}, errors.Errorf("unexpected status code %d", r.StatusCode)
	}

	bytes := make([]byte, 4000)

	// break contents into 4000 byte posts
	for {
		length, err := r.Body.Read(bytes)
		if err != nil && err != io.EOF {
			return &model.CommandResponse{Text: "Encountered error reading file", ResponseType: model.CommandResponseTypeEphemeral}, err
		}

		if length == 0 {
			break
		}

		post := &model.Post{}
		post.Message = string(bytes[:length])
		post.ChannelID = args.ChannelID
		post.UserID = args.UserID

		if _, err := a.CreatePostMissingChannel(c, post, false); err != nil {
			return &model.CommandResponse{Text: "Unable to create post", ResponseType: model.CommandResponseTypeEphemeral}, err
		}
	}

	return &model.CommandResponse{Text: "Loaded data", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func (*LoadTestProvider) JSONCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) (*model.CommandResponse, error) {
	url := strings.TrimSpace(strings.TrimPrefix(message, "json"))
	if url == "" {
		return &model.CommandResponse{Text: "Command must contain a url", ResponseType: model.CommandResponseTypeEphemeral}, nil
	}

	// provide a shortcut to easily access tests stored in doc/developer/tests
	if !strings.HasPrefix(url, "http") {
		url = "https://raw.githubusercontent.com/mattermost/mattermost-server/master/tests/" + url

		if path.Ext(url) == "" {
			url += ".json"
		}
	}

	r, err := http.Get(url)
	if err != nil {
		return &model.CommandResponse{Text: "Unable to get file", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	if r.StatusCode > 400 {
		return &model.CommandResponse{Text: "Unable to get file", ResponseType: model.CommandResponseTypeEphemeral}, errors.Errorf("unexpected status code %d", r.StatusCode)
	}
	defer func() {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()
	}()

	post := model.PostFromJSON(r.Body)
	if post == nil {
		return &model.CommandResponse{Text: "Unable to decode post", ResponseType: model.CommandResponseTypeEphemeral}, errors.Errorf("could not decode post from json")
	}
	post.ChannelID = args.ChannelID
	post.UserID = args.UserID
	if post.Message == "" {
		post.Message = message
	}

	if _, err := a.CreatePostMissingChannel(c, post, false); err != nil {
		return &model.CommandResponse{Text: "Unable to create post", ResponseType: model.CommandResponseTypeEphemeral}, err
	}

	return &model.CommandResponse{Text: "Loaded data", ResponseType: model.CommandResponseTypeEphemeral}, nil
}

func parseRange(command string, cmd string) (utils.Range, bool) {
	tokens := strings.Fields(strings.TrimPrefix(command, cmd))
	var begin int
	var end int
	var err1 error
	var err2 error
	switch {
	case len(tokens) == 1:
		begin, err1 = strconv.Atoi(tokens[0])
		end = begin
		if err1 != nil {
			return utils.Range{Begin: 0, End: 0}, false
		}
	case len(tokens) >= 2:
		begin, err1 = strconv.Atoi(tokens[0])
		end, err2 = strconv.Atoi(tokens[1])
		if err1 != nil || err2 != nil {
			return utils.Range{Begin: 0, End: 0}, false
		}
	default:
		return utils.Range{Begin: 0, End: 0}, false
	}
	return utils.Range{Begin: begin, End: end}, true
}

func contains(items []string, token string) bool {
	for _, elem := range items {
		if elem == token {
			return true
		}
	}
	return false
}
