// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type AutoChannelCreator struct {
	a                  *app.App
	userID             string
	team               *model.Team
	Fuzzy              bool
	DisplayNameLen     utils.Range
	DisplayNameCharset string
	NameLen            utils.Range
	NameCharset        string
	ChannelType        string
}

func NewAutoChannelCreator(a *app.App, team *model.Team, userID string) *AutoChannelCreator {
	return &AutoChannelCreator{
		a:                  a,
		team:               team,
		userID:             userID,
		Fuzzy:              false,
		DisplayNameLen:     ChannelDisplayNameLen,
		DisplayNameCharset: utils.ALPHANUMERIC,
		NameLen:            ChannelNameLen,
		NameCharset:        utils.LOWERCASE,
		ChannelType:        ChannelType,
	}
}

func (cfg *AutoChannelCreator) createRandomChannel(c *request.Context) (*model.Channel, error) {
	var displayName string
	if cfg.Fuzzy {
		displayName = utils.FuzzName()
	} else {
		displayName = utils.RandomName(cfg.NameLen, cfg.NameCharset)
	}
	name := utils.RandomName(cfg.NameLen, cfg.NameCharset)

	channel := &model.Channel{
		TeamID:      cfg.team.ID,
		DisplayName: displayName,
		Name:        name,
		Type:        cfg.ChannelType,
		CreatorID:   cfg.userID,
	}

	channel, err := cfg.a.CreateChannel(c, channel, true)
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (cfg *AutoChannelCreator) CreateTestChannels(c *request.Context, num utils.Range) ([]*model.Channel, error) {
	numChannels := utils.RandIntFromRange(num)
	channels := make([]*model.Channel, numChannels)

	for i := 0; i < numChannels; i++ {
		var err error
		channels[i], err = cfg.createRandomChannel(c)
		if err != nil {
			return nil, err
		}
	}

	return channels, nil
}
