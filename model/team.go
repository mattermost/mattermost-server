// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	TeamOpen                    = "O"
	TeamInvite                  = "I"
	TeamAllowedDomainsMaxLength = 500
	TeamCompanyNameMaxLength    = 64
	TeamDescriptionMaxLength    = 255
	TeamDisplayNameMaxRunes     = 64
	TeamEmailMaxLength          = 128
	TeamNameMaxLength           = 64
	TeamNameMinLength           = 2
)

type Team struct {
	ID                 string  `json:"id" db:"id"`
	CreateAt           int64   `json:"create_at"`
	UpdateAt           int64   `json:"update_at"`
	DeleteAt           int64   `json:"delete_at"`
	DisplayName        string  `json:"display_name"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Email              string  `json:"email"`
	Type               string  `json:"type"`
	CompanyName        string  `json:"company_name"`
	AllowedDomains     string  `json:"allowed_domains"`
	InviteID           string  `json:"invite_id"`
	AllowOpenInvite    bool    `json:"allow_open_invite"`
	LastTeamIconUpdate int64   `json:"last_team_icon_update,omitempty"`
	SchemeID           *string `json:"scheme_id"`
	GroupConstrained   *bool   `json:"group_constrained"`
	PolicyID           *string `json:"policy_id" db:"-"`
}

type TeamPatch struct {
	DisplayName      *string `json:"display_name"`
	Description      *string `json:"description"`
	CompanyName      *string `json:"company_name"`
	AllowedDomains   *string `json:"allowed_domains"`
	AllowOpenInvite  *bool   `json:"allow_open_invite"`
	GroupConstrained *bool   `json:"group_constrained"`
}

type TeamForExport struct {
	Team
	SchemeName *string
}

type Invites struct {
	Invites []map[string]string `json:"invites"`
}

type TeamsWithCount struct {
	Teams      []*Team `json:"teams"`
	TotalCount int64   `json:"total_count"`
}

func InvitesFromJSON(data io.Reader) *Invites {
	var o *Invites
	json.NewDecoder(data).Decode(&o)
	return o
}

func (o *Invites) ToEmailList() []string {
	emailList := make([]string, len(o.Invites))
	for _, invite := range o.Invites {
		emailList = append(emailList, invite["email"])
	}
	return emailList
}

func (o *Invites) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *Team) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func TeamFromJSON(data io.Reader) *Team {
	var o *Team
	json.NewDecoder(data).Decode(&o)
	return o
}

func TeamMapToJSON(u map[string]*Team) string {
	b, _ := json.Marshal(u)
	return string(b)
}

func TeamMapFromJSON(data io.Reader) map[string]*Team {
	var teams map[string]*Team
	json.NewDecoder(data).Decode(&teams)
	return teams
}

func TeamListToJSON(t []*Team) string {
	b, _ := json.Marshal(t)
	return string(b)
}

func TeamsWithCountToJSON(tlc *TeamsWithCount) []byte {
	b, _ := json.Marshal(tlc)
	return b
}

func TeamsWithCountFromJSON(data io.Reader) *TeamsWithCount {
	var twc *TeamsWithCount
	json.NewDecoder(data).Decode(&twc)
	return twc
}

func TeamListFromJSON(data io.Reader) []*Team {
	var teams []*Team
	json.NewDecoder(data).Decode(&teams)
	return teams
}

func (o *Team) Etag() string {
	return Etag(o.ID, o.UpdateAt)
}

func (o *Team) IsValid() *AppError {

	if !IsValidID(o.ID) {
		return NewAppError("Team.IsValid", "model.team.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreateAt == 0 {
		return NewAppError("Team.IsValid", "model.team.is_valid.create_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("Team.IsValid", "model.team.is_valid.update_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.Email) > TeamEmailMaxLength {
		return NewAppError("Team.IsValid", "model.team.is_valid.email.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if o.Email != "" && !IsValidEmail(o.Email) {
		return NewAppError("Team.IsValid", "model.team.is_valid.email.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(o.DisplayName) == 0 || utf8.RuneCountInString(o.DisplayName) > TeamDisplayNameMaxRunes {
		return NewAppError("Team.IsValid", "model.team.is_valid.name.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.Name) > TeamNameMaxLength {
		return NewAppError("Team.IsValid", "model.team.is_valid.url.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.Description) > TeamDescriptionMaxLength {
		return NewAppError("Team.IsValid", "model.team.is_valid.description.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if o.InviteID == "" {
		return NewAppError("Team.IsValid", "model.team.is_valid.invite_id.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if IsReservedTeamName(o.Name) {
		return NewAppError("Team.IsValid", "model.team.is_valid.reserved.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if !IsValidTeamName(o.Name) {
		return NewAppError("Team.IsValid", "model.team.is_valid.characters.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if !(o.Type == TeamOpen || o.Type == TeamInvite) {
		return NewAppError("Team.IsValid", "model.team.is_valid.type.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.CompanyName) > TeamCompanyNameMaxLength {
		return NewAppError("Team.IsValid", "model.team.is_valid.company.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.AllowedDomains) > TeamAllowedDomainsMaxLength {
		return NewAppError("Team.IsValid", "model.team.is_valid.domains.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	return nil
}

func (o *Team) PreSave() {
	if o.ID == "" {
		o.ID = NewID()
	}

	o.CreateAt = GetMillis()
	o.UpdateAt = o.CreateAt

	o.Name = SanitizeUnicode(o.Name)
	o.DisplayName = SanitizeUnicode(o.DisplayName)
	o.Description = SanitizeUnicode(o.Description)
	o.CompanyName = SanitizeUnicode(o.CompanyName)

	if o.InviteID == "" {
		o.InviteID = NewID()
	}
}

func (o *Team) PreUpdate() {
	o.UpdateAt = GetMillis()
	o.Name = SanitizeUnicode(o.Name)
	o.DisplayName = SanitizeUnicode(o.DisplayName)
	o.Description = SanitizeUnicode(o.Description)
	o.CompanyName = SanitizeUnicode(o.CompanyName)
}

func IsReservedTeamName(s string) bool {
	s = strings.ToLower(s)

	for _, value := range reservedName {
		if strings.Index(s, value) == 0 {
			return true
		}
	}

	return false
}

func IsValidTeamName(s string) bool {
	if !IsValidAlphaNum(s) {
		return false
	}

	if len(s) < TeamNameMinLength {
		return false
	}

	return true
}

var validTeamNameCharacter = regexp.MustCompile(`^[a-z0-9-]$`)

func CleanTeamName(s string) string {
	s = strings.ToLower(strings.Replace(s, " ", "-", -1))

	for _, value := range reservedName {
		if strings.Index(s, value) == 0 {
			s = strings.Replace(s, value, "", -1)
		}
	}

	s = strings.TrimSpace(s)

	for _, c := range s {
		char := fmt.Sprintf("%c", c)
		if !validTeamNameCharacter.MatchString(char) {
			s = strings.Replace(s, char, "", -1)
		}
	}

	s = strings.Trim(s, "-")

	if !IsValidTeamName(s) {
		s = NewID()
	}

	return s
}

func (o *Team) Sanitize() {
	o.Email = ""
	o.InviteID = ""
}

func (o *Team) Patch(patch *TeamPatch) {
	if patch.DisplayName != nil {
		o.DisplayName = *patch.DisplayName
	}

	if patch.Description != nil {
		o.Description = *patch.Description
	}

	if patch.CompanyName != nil {
		o.CompanyName = *patch.CompanyName
	}

	if patch.AllowedDomains != nil {
		o.AllowedDomains = *patch.AllowedDomains
	}

	if patch.AllowOpenInvite != nil {
		o.AllowOpenInvite = *patch.AllowOpenInvite
	}

	if patch.GroupConstrained != nil {
		o.GroupConstrained = patch.GroupConstrained
	}
}

func (o *Team) IsGroupConstrained() bool {
	return o.GroupConstrained != nil && *o.GroupConstrained
}

func (t *TeamPatch) ToJSON() string {
	b, err := json.Marshal(t)
	if err != nil {
		return ""
	}

	return string(b)
}

func TeamPatchFromJSON(data io.Reader) *TeamPatch {
	decoder := json.NewDecoder(data)
	var team TeamPatch
	err := decoder.Decode(&team)
	if err != nil {
		return nil
	}

	return &team
}
