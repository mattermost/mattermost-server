// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type TeamEnvironment struct {
	Users    []*model.User
	Channels []*model.Channel
}

type AutoTeamCreator struct {
	client        *model.Client4
	Fuzzy         bool
	NameLength    utils.Range
	NameCharset   string
	DomainLength  utils.Range
	DomainCharset string
	EmailLength   utils.Range
	EmailCharset  string
}

func NewAutoTeamCreator(client *model.Client4) *AutoTeamCreator {
	return &AutoTeamCreator{
		client:        client,
		Fuzzy:         false,
		NameLength:    TeamNameLen,
		NameCharset:   utils.LOWERCASE,
		DomainLength:  TeamDomainNameLen,
		DomainCharset: utils.LOWERCASE,
		EmailLength:   TeamEmailLen,
		EmailCharset:  utils.LOWERCASE,
	}
}

func (cfg *AutoTeamCreator) createRandomTeam() (*model.Team, error) {
	var teamEmail string
	var teamDisplayName string
	var teamName string
	if cfg.Fuzzy {
		teamEmail = "success+" + model.NewID() + "simulator.amazonses.com"
		teamDisplayName = utils.FuzzName()
		teamName = model.NewRandomTeamName()
	} else {
		teamEmail = "success+" + model.NewID() + "simulator.amazonses.com"
		teamDisplayName = utils.RandomName(cfg.NameLength, cfg.NameCharset)
		teamName = utils.RandomName(cfg.NameLength, cfg.NameCharset) + model.NewID()
	}
	team := &model.Team{
		DisplayName: teamDisplayName,
		Name:        teamName,
		Email:       teamEmail,
		Type:        model.TeamOpen,
	}

	createdTeam, resp := cfg.client.CreateTeam(team)
	if resp.Error != nil {
		return nil, resp.Error
	}
	return createdTeam, nil
}

func (cfg *AutoTeamCreator) CreateTestTeams(num utils.Range) ([]*model.Team, error) {
	numTeams := utils.RandIntFromRange(num)
	teams := make([]*model.Team, numTeams)

	for i := 0; i < numTeams; i++ {
		var err error
		teams[i], err = cfg.createRandomTeam()
		if err != nil {
			return nil, err
		}
	}

	return teams, nil
}
