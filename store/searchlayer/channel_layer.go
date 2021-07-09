// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	"context"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchChannelStore struct {
	store.ChannelStore
	rootStore *SearchStore
}

func (c *SearchChannelStore) deleteChannelIndex(channel *model.Channel) {
	if channel.Type == model.ChannelTypeOpen {
		for _, engine := range c.rootStore.searchEngine.GetActiveEngines() {
			if engine.IsIndexingEnabled() {
				runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
					if err := engineCopy.DeleteChannel(channel); err != nil {
						mlog.Warn("Encountered error deleting channel", mlog.String("channel_id", channel.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
						return
					}
					mlog.Debug("Removed channel from index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("channel_id", channel.ID))
				})
			}
		}
	}
}

func (c *SearchChannelStore) indexChannel(channel *model.Channel) {
	if channel.Type == model.ChannelTypeOpen {
		for _, engine := range c.rootStore.searchEngine.GetActiveEngines() {
			if engine.IsIndexingEnabled() {
				runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
					if err := engineCopy.IndexChannel(channel); err != nil {
						mlog.Warn("Encountered error indexing channel", mlog.String("channel_id", channel.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
						return
					}
					mlog.Debug("Indexed channel in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("channel_id", channel.ID))
				})
			}
		}
	}
}

func (c *SearchChannelStore) Save(channel *model.Channel, maxChannels int64) (*model.Channel, error) {
	newChannel, err := c.ChannelStore.Save(channel, maxChannels)
	if err == nil {
		c.indexChannel(newChannel)
	}
	return newChannel, err
}

func (c *SearchChannelStore) Update(channel *model.Channel) (*model.Channel, error) {
	updatedChannel, err := c.ChannelStore.Update(channel)
	if err == nil {
		c.indexChannel(updatedChannel)
	}
	return updatedChannel, err
}

func (c *SearchChannelStore) UpdateMember(cm *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := c.ChannelStore.UpdateMember(cm)
	if err == nil {
		c.rootStore.indexUserFromID(cm.UserID)
		channel, channelErr := c.ChannelStore.Get(member.ChannelID, true)
		if channelErr != nil {
			mlog.Warn("Encountered error indexing user in channel", mlog.String("channel_id", member.ChannelID), mlog.Err(channelErr))
		} else {
			c.rootStore.indexUserFromID(channel.CreatorID)
		}
	}
	return member, err
}

func (c *SearchChannelStore) SaveMember(cm *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := c.ChannelStore.SaveMember(cm)
	if err == nil {
		c.rootStore.indexUserFromID(cm.UserID)
		channel, channelErr := c.ChannelStore.Get(member.ChannelID, true)
		if channelErr != nil {
			mlog.Warn("Encountered error indexing user in channel", mlog.String("channel_id", member.ChannelID), mlog.Err(channelErr))
		} else {
			c.rootStore.indexUserFromID(channel.CreatorID)
		}
	}
	return member, err
}

func (c *SearchChannelStore) RemoveMember(channelID, userIDToRemove string) error {
	err := c.ChannelStore.RemoveMember(channelID, userIDToRemove)
	if err == nil {
		c.rootStore.indexUserFromID(userIDToRemove)
	}
	return err
}

func (c *SearchChannelStore) RemoveMembers(channelID string, userIDs []string) error {
	if err := c.ChannelStore.RemoveMembers(channelID, userIDs); err != nil {
		return err
	}

	for _, uid := range userIDs {
		c.rootStore.indexUserFromID(uid)
	}
	return nil
}

func (c *SearchChannelStore) CreateDirectChannel(user *model.User, otherUser *model.User, channelOptions ...model.ChannelOption) (*model.Channel, error) {
	channel, err := c.ChannelStore.CreateDirectChannel(user, otherUser, channelOptions...)
	if err == nil {
		c.rootStore.indexUserFromID(user.ID)
		c.rootStore.indexUserFromID(otherUser.ID)
	}
	return channel, err
}

func (c *SearchChannelStore) SaveDirectChannel(directchannel *model.Channel, member1 *model.ChannelMember, member2 *model.ChannelMember) (*model.Channel, error) {
	channel, err := c.ChannelStore.SaveDirectChannel(directchannel, member1, member2)
	if err != nil {
		c.rootStore.indexUserFromID(member1.UserID)
		c.rootStore.indexUserFromID(member2.UserID)
	}
	return channel, err
}

func (c *SearchChannelStore) AutocompleteInTeam(teamID string, term string, includeDeleted bool) (*model.ChannelList, error) {
	var channelList *model.ChannelList
	var err error

	allFailed := true
	for _, engine := range c.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsAutocompletionEnabled() {
			channelList, err = c.searchAutocompleteChannels(engine, teamID, term, includeDeleted)
			if err != nil {
				mlog.Warn("Encountered error on AutocompleteChannels through SearchEngine. Falling back to default autocompletion.", mlog.String("search_engine", engine.GetName()), mlog.Err(err))
				continue
			}
			allFailed = false
			mlog.Debug("Using the first available search engine", mlog.String("search_engine", engine.GetName()))
			break
		}
	}

	if allFailed {
		mlog.Debug("Using database search because no other search engine is available")
		channelList, err = c.ChannelStore.AutocompleteInTeam(teamID, term, includeDeleted)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to autocomplete channels in team")
		}
	}

	if err != nil {
		return channelList, err
	}

	return channelList, nil
}

func (c *SearchChannelStore) searchAutocompleteChannels(engine searchengine.SearchEngineInterface, teamID, term string, includeDeleted bool) (*model.ChannelList, error) {
	channelIDs, err := engine.SearchChannels(teamID, term)
	if err != nil {
		return nil, err
	}

	channelList := model.ChannelList{}
	if len(channelIDs) > 0 {
		channels, err := c.ChannelStore.GetChannelsByIDs(channelIDs, includeDeleted)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get channels by ids")
		}

		for _, ch := range channels {
			channelList = append(channelList, ch)
		}
	}

	return &channelList, nil
}

func (c *SearchChannelStore) PermanentDeleteMembersByUser(userID string) error {
	err := c.ChannelStore.PermanentDeleteMembersByUser(userID)
	if err == nil {
		c.rootStore.indexUserFromID(userID)
	}
	return err
}

func (c *SearchChannelStore) RemoveAllDeactivatedMembers(channelID string) error {
	profiles, errProfiles := c.rootStore.User().GetAllProfilesInChannel(context.Background(), channelID, true)
	if errProfiles != nil {
		mlog.Warn("Encountered error indexing users for channel", mlog.String("channel_id", channelID), mlog.Err(errProfiles))
	}

	err := c.ChannelStore.RemoveAllDeactivatedMembers(channelID)
	if err == nil && errProfiles == nil {
		for _, user := range profiles {
			if user.DeleteAt != 0 {
				c.rootStore.indexUser(user)
			}
		}
	}
	return err
}

func (c *SearchChannelStore) PermanentDeleteMembersByChannel(channelID string) error {
	profiles, errProfiles := c.rootStore.User().GetAllProfilesInChannel(context.Background(), channelID, true)
	if errProfiles != nil {
		mlog.Warn("Encountered error indexing users for channel", mlog.String("channel_id", channelID), mlog.Err(errProfiles))
	}

	err := c.ChannelStore.PermanentDeleteMembersByChannel(channelID)
	if err == nil && errProfiles == nil {
		for _, user := range profiles {
			c.rootStore.indexUser(user)
		}
	}
	return err
}

func (c *SearchChannelStore) PermanentDelete(channelID string) error {
	channel, channelErr := c.ChannelStore.Get(channelID, true)
	if channelErr != nil {
		mlog.Warn("Encountered error deleting channel", mlog.String("channel_id", channelID), mlog.Err(channelErr))
	}
	err := c.ChannelStore.PermanentDelete(channelID)
	if err == nil && channelErr == nil {
		c.deleteChannelIndex(channel)
	}
	return err
}
