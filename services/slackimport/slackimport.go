// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slackimport

import (
	"archive/zip"
	"bytes"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type slackChannel struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Creator string          `json:"creator"`
	Members []string        `json:"members"`
	Purpose slackChannelSub `json:"purpose"`
	Topic   slackChannelSub `json:"topic"`
	Type    string
}

type slackChannelSub struct {
	Value string `json:"value"`
}

type slackProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type slackUser struct {
	ID       string       `json:"id"`
	Username string       `json:"name"`
	Profile  slackProfile `json:"profile"`
}

type slackFile struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type slackPost struct {
	User        string                   `json:"user"`
	BotID       string                   `json:"bot_id"`
	BotUsername string                   `json:"username"`
	Text        string                   `json:"text"`
	TimeStamp   string                   `json:"ts"`
	ThreadTS    string                   `json:"thread_ts"`
	Type        string                   `json:"type"`
	SubType     string                   `json:"subtype"`
	Comment     *slackComment            `json:"comment"`
	Upload      bool                     `json:"upload"`
	File        *slackFile               `json:"file"`
	Files       []*slackFile             `json:"files"`
	Attachments []*model.SlackAttachment `json:"attachments"`
}

var isValidChannelNameCharacters = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`).MatchString

const slackImportMaxFileSize = 1024 * 1024 * 70

type slackComment struct {
	User    string `json:"user"`
	Comment string `json:"comment"`
}

// Actions provides the actions that needs to be used for import slack data
type Actions struct {
	UpdateActive           func(*model.User, bool) (*model.User, *model.AppError)
	AddUserToChannel       func(*model.User, *model.Channel, bool) (*model.ChannelMember, *model.AppError)
	JoinUserToTeam         func(*model.Team, *model.User, string) (*model.TeamMember, *model.AppError)
	CreateDirectChannel    func(string, string, ...model.ChannelOption) (*model.Channel, *model.AppError)
	CreateGroupChannel     func([]string) (*model.Channel, *model.AppError)
	CreateChannel          func(*model.Channel, bool) (*model.Channel, *model.AppError)
	DoUploadFile           func(time.Time, string, string, string, string, []byte) (*model.FileInfo, *model.AppError)
	GenerateThumbnailImage func(image.Image, string)
	GeneratePreviewImage   func(image.Image, string)
	InvalidateAllCaches    func()
	MaxPostSize            func() int
	PrepareImage           func(fileData []byte) (image.Image, func(), error)
}

// SlackImporter is a service that allows to import slack dumps into mattermost
type SlackImporter struct {
	store   store.Store
	actions Actions
	config  *model.Config
}

// New creates a new SlackImporter service instance. It receive a store, a set of actions and the current config.
// It is expected to be used right away and discarded after that
func New(store store.Store, actions Actions, config *model.Config) *SlackImporter {
	return &SlackImporter{
		store:   store,
		actions: actions,
		config:  config,
	}
}

func (si *SlackImporter) SlackImport(fileData multipart.File, fileSize int64, teamID string) (*model.AppError, *bytes.Buffer) {
	// Create log file
	log := bytes.NewBufferString(i18n.T("api.slackimport.slack_import.log"))

	zipreader, err := zip.NewReader(fileData, fileSize)
	if err != nil || zipreader.File == nil {
		log.WriteString(i18n.T("api.slackimport.slack_import.zip.app_error"))
		return model.NewAppError("SlackImport", "api.slackimport.slack_import.zip.app_error", nil, err.Error(), http.StatusBadRequest), log
	}

	var channels []slackChannel
	var publicChannels []slackChannel
	var privateChannels []slackChannel
	var groupChannels []slackChannel
	var directChannels []slackChannel

	var users []slackUser
	posts := make(map[string][]slackPost)
	uploads := make(map[string]*zip.File)
	for _, file := range zipreader.File {
		if file.UncompressedSize64 > slackImportMaxFileSize {
			log.WriteString(i18n.T("api.slackimport.slack_import.zip.file_too_large", map[string]interface{}{"Filename": file.Name}))
			continue
		}
		reader, err := file.Open()
		if err != nil {
			log.WriteString(i18n.T("api.slackimport.slack_import.open.app_error", map[string]interface{}{"Filename": file.Name}))
			return model.NewAppError("SlackImport", "api.slackimport.slack_import.open.app_error", map[string]interface{}{"Filename": file.Name}, err.Error(), http.StatusInternalServerError), log
		}
		if file.Name == "channels.json" {
			publicChannels, _ = slackParseChannels(reader, model.ChannelTypeOpen)
			channels = append(channels, publicChannels...)
		} else if file.Name == "dms.json" {
			directChannels, _ = slackParseChannels(reader, model.ChannelTypeDirect)
			channels = append(channels, directChannels...)
		} else if file.Name == "groups.json" {
			privateChannels, _ = slackParseChannels(reader, model.ChannelTypePrivate)
			channels = append(channels, privateChannels...)
		} else if file.Name == "mpims.json" {
			groupChannels, _ = slackParseChannels(reader, model.ChannelTypeGroup)
			channels = append(channels, groupChannels...)
		} else if file.Name == "users.json" {
			users, _ = slackParseUsers(reader)
		} else {
			spl := strings.Split(file.Name, "/")
			if len(spl) == 2 && strings.HasSuffix(spl[1], ".json") {
				newposts, _ := slackParsePosts(reader)
				channel := spl[0]
				if _, ok := posts[channel]; !ok {
					posts[channel] = newposts
				} else {
					posts[channel] = append(posts[channel], newposts...)
				}
			} else if len(spl) == 3 && spl[0] == "__uploads" {
				uploads[spl[1]] = file
			}
		}
	}

	posts = slackConvertUserMentions(users, posts)
	posts = slackConvertChannelMentions(channels, posts)
	posts = slackConvertPostsMarkup(posts)

	addedUsers := si.slackAddUsers(teamID, users, log)
	botUser := si.slackAddBotUser(teamID, log)

	si.slackAddChannels(teamID, channels, posts, addedUsers, uploads, botUser, log)

	if botUser != nil {
		si.deactivateSlackBotUser(botUser)
	}

	si.actions.InvalidateAllCaches()

	log.WriteString(i18n.T("api.slackimport.slack_import.notes"))
	log.WriteString("=======\r\n\r\n")

	log.WriteString(i18n.T("api.slackimport.slack_import.note1"))
	log.WriteString(i18n.T("api.slackimport.slack_import.note2"))
	log.WriteString(i18n.T("api.slackimport.slack_import.note3"))

	return nil, log
}

func truncateRunes(s string, i int) string {
	runes := []rune(s)
	if len(runes) > i {
		return string(runes[:i])
	}
	return s
}

func (si *SlackImporter) slackAddUsers(teamID string, slackusers []slackUser, importerLog *bytes.Buffer) map[string]*model.User {
	// Log header
	importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.created"))
	importerLog.WriteString("===============\r\n\r\n")

	addedUsers := make(map[string]*model.User)

	// Need the team
	team, err := si.store.Team().Get(teamID)
	if err != nil {
		importerLog.WriteString(i18n.T("api.slackimport.slack_import.team_fail"))
		return addedUsers
	}

	for _, sUser := range slackusers {
		firstName := sUser.Profile.FirstName
		lastName := sUser.Profile.LastName
		email := sUser.Profile.Email
		if email == "" {
			email = sUser.Username + "@example.com"
			importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.missing_email_address", map[string]interface{}{"Email": email, "Username": sUser.Username}))
			mlog.Warn("Slack Import: User does not have an email address in the Slack export. Used username as a placeholder. The user should update their email address once logged in to the system.", mlog.String("user_email", email), mlog.String("user_name", sUser.Username))
		}

		password := model.NewID()

		// Check for email conflict and use existing user if found
		if existingUser, err := si.store.User().GetByEmail(email); err == nil {
			addedUsers[sUser.ID] = existingUser
			if _, err := si.actions.JoinUserToTeam(team, addedUsers[sUser.ID], ""); err != nil {
				importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.merge_existing_failed", map[string]interface{}{"Email": existingUser.Email, "Username": existingUser.Username}))
			} else {
				importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.merge_existing", map[string]interface{}{"Email": existingUser.Email, "Username": existingUser.Username}))
			}
			continue
		}

		email = strings.ToLower(email)
		newUser := model.User{
			Username:  sUser.Username,
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			Password:  password,
		}

		mUser := si.oldImportUser(team, &newUser)
		if mUser == nil {
			importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.unable_import", map[string]interface{}{"Username": sUser.Username}))
			continue
		}
		addedUsers[sUser.ID] = mUser
		importerLog.WriteString(i18n.T("api.slackimport.slack_add_users.email_pwd", map[string]interface{}{"Email": newUser.Email, "Password": password}))
	}

	return addedUsers
}

func (si *SlackImporter) slackAddBotUser(teamID string, log *bytes.Buffer) *model.User {
	team, err := si.store.Team().Get(teamID)
	if err != nil {
		log.WriteString(i18n.T("api.slackimport.slack_import.team_fail"))
		return nil
	}

	password := model.NewID()
	username := "slackimportuser_" + model.NewID()
	email := username + "@localhost"

	botUser := model.User{
		Username:  username,
		FirstName: "",
		LastName:  "",
		Email:     email,
		Password:  password,
	}

	mUser := si.oldImportUser(team, &botUser)
	if mUser == nil {
		log.WriteString(i18n.T("api.slackimport.slack_add_bot_user.unable_import", map[string]interface{}{"Username": username}))
		return nil
	}

	log.WriteString(i18n.T("api.slackimport.slack_add_bot_user.email_pwd", map[string]interface{}{"Email": botUser.Email, "Password": password}))
	return mUser
}

func (si *SlackImporter) slackAddPosts(teamID string, channel *model.Channel, posts []slackPost, users map[string]*model.User, uploads map[string]*zip.File, botUser *model.User) {
	sort.Slice(posts, func(i, j int) bool {
		return slackConvertTimeStamp(posts[i].TimeStamp) < slackConvertTimeStamp(posts[j].TimeStamp)
	})
	threads := make(map[string]string)
	for _, sPost := range posts {
		switch {
		case sPost.Type == "message" && (sPost.SubType == "" || sPost.SubType == "file_share"):
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Text,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
			}
			if sPost.Upload {
				if sPost.File != nil {
					if fileInfo, ok := si.slackUploadFile(sPost.File, uploads, teamID, newPost.ChannelID, newPost.UserID, sPost.TimeStamp); ok {
						newPost.FileIDs = append(newPost.FileIDs, fileInfo.ID)
					}
				} else if sPost.Files != nil {
					for _, file := range sPost.Files {
						if fileInfo, ok := si.slackUploadFile(file, uploads, teamID, newPost.ChannelID, newPost.UserID, sPost.TimeStamp); ok {
							newPost.FileIDs = append(newPost.FileIDs, fileInfo.ID)
						}
					}
				}
			}
			// If post in thread
			if sPost.ThreadTS != "" && sPost.ThreadTS != sPost.TimeStamp {
				newPost.RootID = threads[sPost.ThreadTS]
				newPost.ParentID = threads[sPost.ThreadTS]
			}
			postID := si.oldImportPost(&newPost)
			// If post is thread starter
			if sPost.ThreadTS == sPost.TimeStamp {
				threads[sPost.ThreadTS] = postID
			}
		case sPost.Type == "message" && sPost.SubType == "file_comment":
			if sPost.Comment == nil {
				mlog.Debug("Slack Import: Unable to import the message as it has no comments.")
				continue
			}
			if sPost.Comment.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.Comment.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.Comment.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Comment.Comment,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
			}
			si.oldImportPost(&newPost)
		case sPost.Type == "message" && sPost.SubType == "bot_message":
			if botUser == nil {
				mlog.Warn("Slack Import: Unable to import the bot message as the bot user does not exist.")
				continue
			}
			if sPost.BotID == "" {
				mlog.Warn("Slack Import: Unable to import bot message as the BotId field is missing.")
				continue
			}

			props := make(model.StringInterface)
			props["override_username"] = sPost.BotUsername
			if len(sPost.Attachments) > 0 {
				props["attachments"] = sPost.Attachments
			}

			post := &model.Post{
				UserID:    botUser.ID,
				ChannelID: channel.ID,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
				Message:   sPost.Text,
				Type:      model.PostTypeSlackAttachment,
			}

			postID := si.oldImportIncomingWebhookPost(post, props)
			// If post is thread starter
			if sPost.ThreadTS == sPost.TimeStamp {
				threads[sPost.ThreadTS] = postID
			}
		case sPost.Type == "message" && (sPost.SubType == "channel_join" || sPost.SubType == "channel_leave"):
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}

			var postType string
			if sPost.SubType == "channel_join" {
				postType = model.PostTypeJoinChannel
			} else {
				postType = model.PostTypeLeaveChannel
			}

			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Text,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
				Type:      postType,
				Props: model.StringInterface{
					"username": users[sPost.User].Username,
				},
			}
			si.oldImportPost(&newPost)
		case sPost.Type == "message" && sPost.SubType == "me_message":
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   "*" + sPost.Text + "*",
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
			}
			postID := si.oldImportPost(&newPost)
			// If post is thread starter
			if sPost.ThreadTS == sPost.TimeStamp {
				threads[sPost.ThreadTS] = postID
			}
		case sPost.Type == "message" && sPost.SubType == "channel_topic":
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Text,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
				Type:      model.PostTypeHeaderChange,
			}
			si.oldImportPost(&newPost)
		case sPost.Type == "message" && sPost.SubType == "channel_purpose":
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Text,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
				Type:      model.PostTypePurposeChange,
			}
			si.oldImportPost(&newPost)
		case sPost.Type == "message" && sPost.SubType == "channel_name":
			if sPost.User == "" {
				mlog.Debug("Slack Import: Unable to import the message as the user field is missing.")
				continue
			}
			if users[sPost.User] == nil {
				mlog.Debug("Slack Import: Unable to add the message as the Slack user does not exist in Mattermost.", mlog.String("user", sPost.User))
				continue
			}
			newPost := model.Post{
				UserID:    users[sPost.User].ID,
				ChannelID: channel.ID,
				Message:   sPost.Text,
				CreateAt:  slackConvertTimeStamp(sPost.TimeStamp),
				Type:      model.PostTypeDisplaynameChange,
			}
			si.oldImportPost(&newPost)
		default:
			mlog.Warn(
				"Slack Import: Unable to import the message as its type is not supported",
				mlog.String("post_type", sPost.Type),
				mlog.String("post_subtype", sPost.SubType),
			)
		}
	}
}

func (si *SlackImporter) slackUploadFile(slackPostFile *slackFile, uploads map[string]*zip.File, teamID string, channelID string, userID string, slackTimestamp string) (*model.FileInfo, bool) {
	if slackPostFile == nil {
		mlog.Warn("Slack Import: Unable to attach the file to the post as the latter has no file section present in Slack export.")
		return nil, false
	}
	file, ok := uploads[slackPostFile.ID]
	if !ok {
		mlog.Warn("Slack Import: Unable to import file as the file is missing from the Slack export zip file.", mlog.String("file_id", slackPostFile.ID))
		return nil, false
	}
	openFile, err := file.Open()
	if err != nil {
		mlog.Warn("Slack Import: Unable to open the file from the Slack export.", mlog.String("file_id", slackPostFile.ID), mlog.Err(err))
		return nil, false
	}
	defer openFile.Close()

	timestamp := utils.TimeFromMillis(slackConvertTimeStamp(slackTimestamp))
	uploadedFile, err := si.oldImportFile(timestamp, openFile, teamID, channelID, userID, filepath.Base(file.Name))
	if err != nil {
		mlog.Warn("Slack Import: An error occurred when uploading file.", mlog.String("file_id", slackPostFile.ID), mlog.Err(err))
		return nil, false
	}

	return uploadedFile, true
}

func (si *SlackImporter) deactivateSlackBotUser(user *model.User) {
	if _, err := si.actions.UpdateActive(user, false); err != nil {
		mlog.Warn("Slack Import: Unable to deactivate the user account used for the bot.")
	}
}

func (si *SlackImporter) addSlackUsersToChannel(members []string, users map[string]*model.User, channel *model.Channel, log *bytes.Buffer) {
	for _, member := range members {
		user, ok := users[member]
		if !ok {
			log.WriteString(i18n.T("api.slackimport.slack_add_channels.failed_to_add_user", map[string]interface{}{"Username": "?"}))
			continue
		}
		if _, err := si.actions.AddUserToChannel(user, channel, false); err != nil {
			log.WriteString(i18n.T("api.slackimport.slack_add_channels.failed_to_add_user", map[string]interface{}{"Username": user.Username}))
		}
	}
}

func slackSanitiseChannelProperties(channel model.Channel) model.Channel {
	if utf8.RuneCountInString(channel.DisplayName) > model.ChannelDisplayNameMaxRunes {
		mlog.Warn("Slack Import: Channel display name exceeds the maximum length. It will be truncated when imported.", mlog.String("channel_display_name", channel.DisplayName))
		channel.DisplayName = truncateRunes(channel.DisplayName, model.ChannelDisplayNameMaxRunes)
	}

	if len(channel.Name) > model.ChannelNameMaxLength {
		mlog.Warn("Slack Import: Channel handle exceeds the maximum length. It will be truncated when imported.", mlog.String("channel_display_name", channel.DisplayName))
		channel.Name = channel.Name[0:model.ChannelNameMaxLength]
	}

	if utf8.RuneCountInString(channel.Purpose) > model.ChannelPurposeMaxRunes {
		mlog.Warn("Slack Import: Channel purpose exceeds the maximum length. It will be truncated when imported.", mlog.String("channel_display_name", channel.DisplayName))
		channel.Purpose = truncateRunes(channel.Purpose, model.ChannelPurposeMaxRunes)
	}

	if utf8.RuneCountInString(channel.Header) > model.ChannelHeaderMaxRunes {
		mlog.Warn("Slack Import: Channel header exceeds the maximum length. It will be truncated when imported.", mlog.String("channel_display_name", channel.DisplayName))
		channel.Header = truncateRunes(channel.Header, model.ChannelHeaderMaxRunes)
	}

	return channel
}

func (si *SlackImporter) slackAddChannels(teamID string, slackchannels []slackChannel, posts map[string][]slackPost, users map[string]*model.User, uploads map[string]*zip.File, botUser *model.User, importerLog *bytes.Buffer) map[string]*model.Channel {
	// Write Header
	importerLog.WriteString(i18n.T("api.slackimport.slack_add_channels.added"))
	importerLog.WriteString("=================\r\n\r\n")

	addedChannels := make(map[string]*model.Channel)
	for _, sChannel := range slackchannels {
		newChannel := model.Channel{
			TeamID:      teamID,
			Type:        sChannel.Type,
			DisplayName: sChannel.Name,
			Name:        slackConvertChannelName(sChannel.Name, sChannel.ID),
			Purpose:     sChannel.Purpose.Value,
			Header:      sChannel.Topic.Value,
		}

		// Direct message channels in Slack don't have a name so we set the id as name or else the messages won't get imported.
		if newChannel.Type == model.ChannelTypeDirect {
			sChannel.Name = sChannel.ID
		}

		newChannel = slackSanitiseChannelProperties(newChannel)

		var mChannel *model.Channel
		var err error
		if mChannel, err = si.store.Channel().GetByName(teamID, sChannel.Name, true); err == nil {
			// The channel already exists as an active channel. Merge with the existing one.
			importerLog.WriteString(i18n.T("api.slackimport.slack_add_channels.merge", map[string]interface{}{"DisplayName": newChannel.DisplayName}))
		} else if _, nErr := si.store.Channel().GetDeletedByName(teamID, sChannel.Name); nErr == nil {
			// The channel already exists but has been deleted. Generate a random string for the handle instead.
			newChannel.Name = model.NewID()
			newChannel = slackSanitiseChannelProperties(newChannel)
		}

		if mChannel == nil {
			// Haven't found an existing channel to merge with. Try importing it as a new one.
			mChannel = si.oldImportChannel(&newChannel, sChannel, users)
			if mChannel == nil {
				mlog.Warn("Slack Import: Unable to import Slack channel.", mlog.String("channel_display_name", newChannel.DisplayName))
				importerLog.WriteString(i18n.T("api.slackimport.slack_add_channels.import_failed", map[string]interface{}{"DisplayName": newChannel.DisplayName}))
				continue
			}
		}

		// Members for direct and group channels are added during the creation of the channel in the oldImportChannel function
		if sChannel.Type == model.ChannelTypeOpen || sChannel.Type == model.ChannelTypePrivate {
			si.addSlackUsersToChannel(sChannel.Members, users, mChannel, importerLog)
		}
		importerLog.WriteString(newChannel.DisplayName + "\r\n")
		addedChannels[sChannel.ID] = mChannel
		si.slackAddPosts(teamID, mChannel, posts[sChannel.Name], users, uploads, botUser)
	}

	return addedChannels
}

//
// -- Old SlackImport Functions --
// Import functions are suitable for entering posts and users into the database without
// some of the usual checks. (IsValid is still run)
//

func (si *SlackImporter) oldImportPost(post *model.Post) string {
	// Workaround for empty messages, which may be the case if they are webhook posts.
	firstIteration := true
	firstPostID := ""
	if post.ParentID != "" {
		firstPostID = post.ParentID
	}
	maxPostSize := si.actions.MaxPostSize()
	for messageRuneCount := utf8.RuneCountInString(post.Message); messageRuneCount > 0 || firstIteration; messageRuneCount = utf8.RuneCountInString(post.Message) {
		var remainder string
		if messageRuneCount > maxPostSize {
			remainder = string(([]rune(post.Message))[maxPostSize:])
			post.Message = truncateRunes(post.Message, maxPostSize)
		} else {
			remainder = ""
		}

		post.Hashtags, _ = model.ParseHashtags(post.Message)

		post.RootID = firstPostID
		post.ParentID = firstPostID

		_, err := si.store.Post().Save(post)
		if err != nil {
			mlog.Debug("Error saving post.", mlog.String("user_id", post.UserID), mlog.String("message", post.Message))
		}

		if firstIteration {
			if firstPostID == "" {
				firstPostID = post.ID
			}
			for _, fileID := range post.FileIDs {
				if err := si.store.FileInfo().AttachToPost(fileID, post.ID, post.UserID); err != nil {
					mlog.Error(
						"Error attaching files to post.",
						mlog.String("post_id", post.ID),
						mlog.String("file_ids", strings.Join(post.FileIDs, ",")),
						mlog.String("user_id", post.UserID),
						mlog.Err(err),
					)
				}
			}
			post.FileIDs = nil
		}

		post.ID = ""
		post.CreateAt++
		post.Message = remainder
		firstIteration = false
	}
	return firstPostID
}

func (si *SlackImporter) oldImportUser(team *model.Team, user *model.User) *model.User {
	user.MakeNonNil()

	user.Roles = model.SystemUserRoleID

	ruser, nErr := si.store.User().Save(user)
	if nErr != nil {
		mlog.Debug("Error saving user.", mlog.Err(nErr))
		return nil
	}

	if _, err := si.store.User().VerifyEmail(ruser.ID, ruser.Email); err != nil {
		mlog.Warn("Failed to set email verified.", mlog.Err(err))
	}

	if _, err := si.actions.JoinUserToTeam(team, user, ""); err != nil {
		mlog.Warn("Failed to join team when importing.", mlog.Err(err))
	}

	return ruser
}

func (si *SlackImporter) oldImportChannel(channel *model.Channel, sChannel slackChannel, users map[string]*model.User) *model.Channel {
	switch {
	case channel.Type == model.ChannelTypeDirect:
		if len(sChannel.Members) < 2 {
			return nil
		}
		u1 := users[sChannel.Members[0]]
		u2 := users[sChannel.Members[1]]
		if u1 == nil || u2 == nil {
			mlog.Warn("Either or both of user ids not found in users.json. Ignoring.", mlog.String("id1", sChannel.Members[0]), mlog.String("id2", sChannel.Members[1]))
			return nil
		}
		sc, err := si.actions.CreateDirectChannel(u1.ID, u2.ID)
		if err != nil {
			return nil
		}

		return sc
	// check if direct channel has less than 8 members and if not import as private channel instead
	case channel.Type == model.ChannelTypeGroup && len(sChannel.Members) < 8:
		members := make([]string, len(sChannel.Members))

		for i := range sChannel.Members {
			u := users[sChannel.Members[i]]
			if u == nil {
				mlog.Warn("User not found in users.json. Ignoring.", mlog.String("id", sChannel.Members[i]))
				continue
			}
			members[i] = u.ID
		}

		creator := users[sChannel.Creator]
		if creator == nil {
			return nil
		}
		sc, err := si.actions.CreateGroupChannel(members)
		if err != nil {
			return nil
		}

		return sc
	case channel.Type == model.ChannelTypeGroup:
		channel.Type = model.ChannelTypePrivate
		sc, err := si.actions.CreateChannel(channel, false)
		if err != nil {
			return nil
		}

		return sc
	}

	sc, err := si.store.Channel().Save(channel, *si.config.TeamSettings.MaxChannelsPerTeam)
	if err != nil {
		return nil
	}

	return sc
}

func (si *SlackImporter) oldImportFile(timestamp time.Time, file io.Reader, teamID string, channelID string, userID string, fileName string) (*model.FileInfo, error) {
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, file)
	data := buf.Bytes()

	fileInfo, err := si.actions.DoUploadFile(timestamp, teamID, channelID, userID, fileName, data)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsImage() && fileInfo.MimeType != "image/svg+xml" {
		img, release, err := si.actions.PrepareImage(data)
		if err != nil {
			return nil, err
		}
		defer release()
		si.actions.GenerateThumbnailImage(img, fileInfo.ThumbnailPath)
		si.actions.GeneratePreviewImage(img, fileInfo.PreviewPath)
	}

	return fileInfo, nil
}

func (si *SlackImporter) oldImportIncomingWebhookPost(post *model.Post, props model.StringInterface) string {
	linkWithTextRegex := regexp.MustCompile(`<([^<\|]+)\|([^>]+)>`)
	post.Message = linkWithTextRegex.ReplaceAllString(post.Message, "[${2}](${1})")

	post.AddProp("from_webhook", "true")

	if _, ok := props["override_username"]; !ok {
		post.AddProp("override_username", model.DefaultWebhookUsername)
	}

	if len(props) > 0 {
		for key, val := range props {
			if key == "attachments" {
				if attachments, success := val.([]*model.SlackAttachment); success {
					model.ParseSlackAttachment(post, attachments)
				}
			} else if key != "from_webhook" {
				post.AddProp(key, val)
			}
		}
	}

	return si.oldImportPost(post)
}
