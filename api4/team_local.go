// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/app/email"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (api *API) InitTeamLocal() {
	api.BaseRoutes.Teams.Handle("", api.ApiLocal(localCreateTeam)).Methods("POST")
	api.BaseRoutes.Teams.Handle("", api.ApiLocal(getAllTeams)).Methods("GET")
	api.BaseRoutes.Teams.Handle("/search", api.ApiLocal(searchTeams)).Methods("POST")

	api.BaseRoutes.Team.Handle("", api.ApiLocal(getTeam)).Methods("GET")
	api.BaseRoutes.Team.Handle("", api.ApiLocal(updateTeam)).Methods("PUT")
	api.BaseRoutes.Team.Handle("", api.ApiLocal(localDeleteTeam)).Methods("DELETE")
	api.BaseRoutes.Team.Handle("/invite/email", api.ApiLocal(localInviteUsersToTeam)).Methods("POST")
	api.BaseRoutes.Team.Handle("/patch", api.ApiLocal(patchTeam)).Methods("PUT")
	api.BaseRoutes.Team.Handle("/privacy", api.ApiLocal(updateTeamPrivacy)).Methods("PUT")
	api.BaseRoutes.Team.Handle("/restore", api.ApiLocal(restoreTeam)).Methods("POST")

	api.BaseRoutes.TeamByName.Handle("", api.ApiLocal(getTeamByName)).Methods("GET")
	api.BaseRoutes.TeamMembers.Handle("", api.ApiLocal(addTeamMember)).Methods("POST")
	api.BaseRoutes.TeamMember.Handle("", api.ApiLocal(removeTeamMember)).Methods("DELETE")
}

func localDeleteTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("localDeleteTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if team, err := c.App.GetTeam(c.Params.TeamId); err == nil {
		auditRec.AddMeta("team", team)
	}

	var err *model.AppError
	if c.Params.Permanent {
		err = c.App.PermanentDeleteTeamId(c.Params.TeamId)
	} else {
		err = c.App.SoftDeleteTeam(c.Params.TeamId)
	}

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func localInviteUsersToTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().ServiceSettings.EnableEmailInvitations {
		c.Err = model.NewAppError("localInviteUsersToTeam", "api.team.invite_members.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	emailList := model.ArrayFromJson(r.Body)
	if len(emailList) == 0 {
		c.SetInvalidParam("user_email")
		return
	}
	for i := range emailList {
		email := strings.ToLower(emailList[i])
		if !model.IsValidEmail(email) {
			c.Err = model.NewAppError("localInviteUsersToTeam", "api.team.invite_members.invalid_email.app_error", map[string]interface{}{"Address": email}, "", http.StatusBadRequest)
			return
		}
		emailList[i] = email
	}

	auditRec := c.MakeAuditRecord("localInviteUsersToTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamId)
	auditRec.AddMeta("count", len(emailList))
	auditRec.AddMeta("emails", emailList)

	team, nErr := c.App.Srv().Store.Team().Get(c.Params.TeamId)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			c.Err = model.NewAppError("localInviteUsersToTeam", "app.team.get.find.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			c.Err = model.NewAppError("localInviteUsersToTeam", "app.team.get.finding.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
		return
	}

	allowedDomains := []string{team.AllowedDomains, *c.App.Config().TeamSettings.RestrictCreationToDomains}

	if r.URL.Query().Get("graceful") != "" {
		var invitesWithErrors []*model.EmailInviteWithError
		var goodEmails, errList []string
		for _, email := range emailList {
			invite := &model.EmailInviteWithError{
				Email: email,
				Error: nil,
			}
			if !isEmailAddressAllowed(email, allowedDomains) {
				invite.Error = model.NewAppError("localInviteUsersToTeam", "api.team.invite_members.invalid_email.app_error", map[string]interface{}{"Addresses": email}, "", http.StatusBadRequest)
				errList = append(errList, model.EmailInviteWithErrorToString(invite))
			} else {
				goodEmails = append(goodEmails, email)
			}
			invitesWithErrors = append(invitesWithErrors, invite)
		}
		auditRec.AddMeta("errors", errList)
		if len(goodEmails) > 0 {
			err := c.App.Srv().EmailService.SendInviteEmails(team, "Administrator", "mmctl "+model.NewId(), goodEmails, *c.App.Config().ServiceSettings.SiteURL)
			if err != nil {
				switch {
				case errors.Is(err, email.NoRateLimiterError):
					c.Err = model.NewAppError("SendInviteEmails", "app.email.no_rate_limiter.app_error", nil, fmt.Sprintf("team_id=%s", team.Id), http.StatusInternalServerError)
				case errors.Is(err, email.SetupRateLimiterError):
					c.Err = model.NewAppError("SendInviteEmails", "app.email.setup_rate_limiter.app_error", nil, fmt.Sprintf("team_id=%s, error=%v", team.Id, err), http.StatusInternalServerError)
				default:
					c.Err = model.NewAppError("SendInviteEmails", "app.email.rate_limit_exceeded.app_error", nil, fmt.Sprintf("team_id=%s, error=%v", team.Id, err), http.StatusRequestEntityTooLarge)
				}
				return
			}
		}
		// in graceful mode we return both the successful ones and the failed ones
		w.Write([]byte(model.EmailInviteWithErrorToJson(invitesWithErrors)))
	} else {
		var invalidEmailList []string

		for _, email := range emailList {
			if !isEmailAddressAllowed(email, allowedDomains) {
				invalidEmailList = append(invalidEmailList, email)
			}
		}
		if len(invalidEmailList) > 0 {
			s := strings.Join(invalidEmailList, ", ")
			c.Err = model.NewAppError("localInviteUsersToTeam", "api.team.invite_members.invalid_email.app_error", map[string]interface{}{"Addresses": s}, "", http.StatusBadRequest)
			return
		}
		err := c.App.Srv().EmailService.SendInviteEmails(team, "Administrator", "mmctl "+model.NewId(), emailList, *c.App.Config().ServiceSettings.SiteURL)
		if err != nil {
			switch {
			case errors.Is(err, email.NoRateLimiterError):
				c.Err = model.NewAppError("SendInviteEmails", "app.email.no_rate_limiter.app_error", nil, fmt.Sprintf("team_id=%s", team.Id), http.StatusInternalServerError)
			case errors.Is(err, email.SetupRateLimiterError):
				c.Err = model.NewAppError("SendInviteEmails", "app.email.setup_rate_limiter.app_error", nil, fmt.Sprintf("team_id=%s, error=%v", team.Id, err), http.StatusInternalServerError)
			default:
				c.Err = model.NewAppError("SendInviteEmails", "app.email.rate_limit_exceeded.app_error", nil, fmt.Sprintf("team_id=%s, error=%v", team.Id, err), http.StatusRequestEntityTooLarge)
			}
			return
		}
		ReturnStatusOK(w)
	}
	auditRec.Success()
}

func isEmailAddressAllowed(email string, allowedDomains []string) bool {
	for _, restriction := range allowedDomains {
		domains := normalizeDomains(restriction)
		if len(domains) <= 0 {
			continue
		}
		matched := false
		for _, d := range domains {
			if strings.HasSuffix(email, "@"+d) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func normalizeDomains(domains string) []string {
	// commas and @ signs are optional
	// can be in the form of "@corp.mattermost.com, mattermost.com mattermost.org" -> corp.mattermost.com mattermost.com mattermost.org
	return strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(strings.Replace(domains, "@", " ", -1), ",", " ", -1))))
}

func localCreateTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	team := model.TeamFromJson(r.Body)
	if team == nil {
		c.SetInvalidParam("team")
		return
	}
	team.Email = strings.ToLower(team.Email)

	auditRec := c.MakeAuditRecord("localCreateTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team", team)

	rteam, err := c.App.CreateTeam(c.AppContext, team)
	if err != nil {
		c.Err = err
		return
	}
	// Don't sanitize the team here since the user will be a team admin and their session won't reflect that yet

	auditRec.Success()
	auditRec.AddMeta("team", team) // overwrite meta

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rteam.ToJson()))
}
