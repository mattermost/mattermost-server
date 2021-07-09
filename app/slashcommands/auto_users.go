// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type AutoUserCreator struct {
	app          *app.App
	client       *model.Client4
	team         *model.Team
	EmailLength  utils.Range
	EmailCharset string
	NameLength   utils.Range
	NameCharset  string
	Fuzzy        bool
}

func NewAutoUserCreator(a *app.App, client *model.Client4, team *model.Team) *AutoUserCreator {
	return &AutoUserCreator{
		app:          a,
		client:       client,
		team:         team,
		EmailLength:  UserEmailLen,
		EmailCharset: utils.LOWERCASE,
		NameLength:   UserNameLen,
		NameCharset:  utils.LOWERCASE,
		Fuzzy:        false,
	}
}

// Basic test team and user so you always know one
func CreateBasicUser(a *app.App, client *model.Client4) *model.AppError {
	found, _ := client.TeamExists(BTestTeamName, "")
	if found {
		return nil
	}

	newteam := &model.Team{DisplayName: BTestTeamDisplayName, Name: BTestTeamName, Email: BTestTeamEmail, Type: BTestTeamType}
	basicteam, resp := client.CreateTeam(newteam)
	if resp.Error != nil {
		return resp.Error
	}
	newuser := &model.User{Email: BTestUserEmail, Nickname: BTestUserName, Password: BTestUserPassword}
	ruser, resp := client.CreateUser(newuser)
	if resp.Error != nil {
		return resp.Error
	}
	_, err := a.Srv().Store.User().VerifyEmail(ruser.ID, ruser.Email)
	if err != nil {
		return model.NewAppError("CreateBasicUser", "app.user.verify_email.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if _, nErr := a.Srv().Store.Team().SaveMember(&model.TeamMember{TeamID: basicteam.ID, UserID: ruser.ID}, *a.Config().TeamSettings.MaxUsersPerTeam); nErr != nil {
		var appErr *model.AppError
		var conflictErr *store.ErrConflict
		var limitExceededErr *store.ErrLimitExceeded
		switch {
		case errors.As(nErr, &appErr): // in case we haven't converted to plain error.
			return appErr
		case errors.As(nErr, &conflictErr):
			return model.NewAppError("CreateBasicUser", "app.create_basic_user.save_member.conflict.app_error", nil, nErr.Error(), http.StatusBadRequest)
		case errors.As(nErr, &limitExceededErr):
			return model.NewAppError("CreateBasicUser", "app.create_basic_user.save_member.max_accounts.app_error", nil, nErr.Error(), http.StatusBadRequest)
		default: // last fallback in case it doesn't map to an existing app error.
			return model.NewAppError("CreateBasicUser", "app.create_basic_user.save_member.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	return nil
}

func (cfg *AutoUserCreator) createRandomUser(c *request.Context) (*model.User, error) {
	var userEmail string
	var userName string
	if cfg.Fuzzy {
		userEmail = "success+" + model.NewID() + "@simulator.amazonses.com"
		userName = utils.FuzzName()
	} else {
		userEmail = "success+" + model.NewID() + "@simulator.amazonses.com"
		userName = utils.RandomName(cfg.NameLength, cfg.NameCharset)
	}

	user := &model.User{
		Email:    userEmail,
		Nickname: userName,
		Password: UserPassword}

	ruser, appErr := cfg.app.CreateUserWithInviteID(c, user, cfg.team.InviteID, "")
	if appErr != nil {
		return nil, appErr
	}

	status := &model.Status{UserID: ruser.ID, Status: model.StatusOnline, Manual: false, LastActivityAt: model.GetMillis(), ActiveChannel: ""}
	if err := cfg.app.Srv().Store.Status().SaveOrUpdate(status); err != nil {
		return nil, err
	}

	// We need to cheat to verify the user's email
	_, err := cfg.app.Srv().Store.User().VerifyEmail(ruser.ID, ruser.Email)
	if err != nil {
		return nil, err
	}

	return ruser, nil
}

func (cfg *AutoUserCreator) CreateTestUsers(c *request.Context, num utils.Range) ([]*model.User, error) {
	numUsers := utils.RandIntFromRange(num)
	users := make([]*model.User, numUsers)

	for i := 0; i < numUsers; i++ {
		var err error
		users[i], err = cfg.createRandomUser(c)
		if err != nil {
			return nil, err
		}
	}

	return users, nil
}
