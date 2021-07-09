// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchUserStore struct {
	store.UserStore
	rootStore *SearchStore
}

func (s *SearchUserStore) deleteUserIndex(user *model.User) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteUser(user); err != nil {
					mlog.Error("Encountered error deleting user", mlog.String("user_id", user.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed user from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("user_id", user.ID))
			})
		}
	}
}

func (s *SearchUserStore) Search(teamID, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsSearchEnabled() {
			listOfAllowedChannels, nErr := s.getListOfAllowedChannelsForTeam(teamID, options.ViewRestrictions)
			if nErr != nil {
				mlog.Warn("Encountered error on Search.", mlog.String("search_engine", engine.GetName()), mlog.Err(nErr))
				continue
			}

			if listOfAllowedChannels != nil && len(listOfAllowedChannels) == 0 {
				return []*model.User{}, nil
			}

			sanitizedTerm := sanitizeSearchTerm(term)

			usersIDs, err := engine.SearchUsersInTeam(teamID, listOfAllowedChannels, sanitizedTerm, options)
			if err != nil {
				mlog.Warn("Encountered error on Search", mlog.String("search_engine", engine.GetName()), mlog.Err(err))
				continue
			}

			users, nErr := s.UserStore.GetProfileByIDs(context.Background(), usersIDs, nil, false)
			if nErr != nil {
				mlog.Warn("Encountered error on Search", mlog.String("search_engine", engine.GetName()), mlog.Err(nErr))
				continue
			}

			mlog.Debug("Using the first available search engine", mlog.String("search_engine", engine.GetName()))
			return users, nil
		}
	}

	mlog.Debug("Using database search because no other search engine is available")

	return s.UserStore.Search(teamID, term, options)
}

func (s *SearchUserStore) Update(user *model.User, trustedUpdateData bool) (*model.UserUpdate, error) {
	userUpdate, err := s.UserStore.Update(user, trustedUpdateData)

	if err == nil {
		s.rootStore.indexUser(userUpdate.New)
	}
	return userUpdate, err
}

func (s *SearchUserStore) Save(user *model.User) (*model.User, error) {
	nuser, err := s.UserStore.Save(user)

	if err == nil {
		s.rootStore.indexUser(nuser)
	}
	return nuser, err
}

func (s *SearchUserStore) PermanentDelete(userID string) error {
	user, userErr := s.UserStore.Get(context.Background(), userID)
	if userErr != nil {
		mlog.Warn("Encountered error deleting user", mlog.String("user_id", userID), mlog.Err(userErr))
	}
	err := s.UserStore.PermanentDelete(userID)
	if err == nil && userErr == nil {
		s.deleteUserIndex(user)
	}
	return err
}

func (s *SearchUserStore) autocompleteUsersInChannelByEngine(engine searchengine.SearchEngineInterface, teamID, channelID, term string, options *model.UserSearchOptions) (*model.UserAutocompleteInChannel, error) {
	var err *model.AppError
	uchanIDs := []string{}
	nuchanIDs := []string{}
	sanitizedTerm := sanitizeSearchTerm(term)
	if channelID != "" && options.ListOfAllowedChannels != nil && !strings.Contains(strings.Join(options.ListOfAllowedChannels, "."), channelID) {
		nuchanIDs, err = engine.SearchUsersInTeam(teamID, options.ListOfAllowedChannels, sanitizedTerm, options)
	} else {
		uchanIDs, nuchanIDs, err = engine.SearchUsersInChannel(teamID, channelID, options.ListOfAllowedChannels, sanitizedTerm, options)
	}
	if err != nil {
		return nil, err
	}

	uchan := make(chan store.StoreResult, 1)
	go func() {
		users, nErr := s.UserStore.GetProfileByIDs(context.Background(), uchanIDs, nil, false)
		uchan <- store.StoreResult{Data: users, NErr: nErr}
		close(uchan)
	}()

	nuchan := make(chan store.StoreResult, 1)
	go func() {
		users, nErr := s.UserStore.GetProfileByIDs(context.Background(), nuchanIDs, nil, false)
		nuchan <- store.StoreResult{Data: users, NErr: nErr}
		close(nuchan)
	}()

	autocomplete := &model.UserAutocompleteInChannel{}

	result := <-uchan
	if result.NErr != nil {
		return nil, errors.Wrap(result.NErr, "failed to get user profiles by ids")
	}
	inUsers := result.Data.([]*model.User)
	autocomplete.InChannel = inUsers

	result = <-nuchan
	if result.NErr != nil {
		return nil, errors.Wrap(result.NErr, "failed to get user profiles by ids")
	}
	outUsers := result.Data.([]*model.User)
	autocomplete.OutOfChannel = outUsers

	return autocomplete, nil
}

// getListOfAllowedChannelsForTeam return the list of allowed channels to search user based on the
//	next scenarios:
//		- If there isn't view restrictions (team or channel) and no team id to filter them, then all
//		  channels are allowed (nil return)
//	   	- If we receive a team Id and either we don't have view restrictions or the provided team id is included in the
//		  list of restricted teams, then we return all the team channels
//		- If we don't receive team id or the provided team id is not in the list of allowed teams to search of and we
//		  don't have channel restrictions then we return an empty result because we cannot get channels
//		- If we receive channels restrictions we get:
//			- If we don't have team id, we get those restricted channels (guest accounts and quick search)
//			- If we have a team id then we only return those restricted channels that belongs to that team
func (s *SearchUserStore) getListOfAllowedChannelsForTeam(teamID string, viewRestrictions *model.ViewUsersRestrictions) ([]string, error) {
	var listOfAllowedChannels []string
	if viewRestrictions == nil && teamID == "" {
		// nil return without error means all channels are allowed
		return nil, nil
	}

	if teamID != "" && (viewRestrictions == nil || strings.Contains(strings.Join(viewRestrictions.Teams, "."), teamID)) {
		channels, err := s.rootStore.Channel().GetTeamChannels(teamID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get team channels")
		}
		for _, channel := range *channels {
			listOfAllowedChannels = append(listOfAllowedChannels, channel.ID)
		}
		return listOfAllowedChannels, nil
	}

	if len(viewRestrictions.Channels) > 0 {
		channels, err := s.rootStore.Channel().GetChannelsByIDs(viewRestrictions.Channels, false)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get channels by ids")
		}
		for _, c := range channels {
			if teamID == "" || (teamID != "" && c.TeamID == teamID) {
				listOfAllowedChannels = append(listOfAllowedChannels, c.ID)
			}
		}
		return listOfAllowedChannels, nil
	}

	return []string{}, nil
}

func (s *SearchUserStore) AutocompleteUsersInChannel(teamID, channelID, term string, options *model.UserSearchOptions) (*model.UserAutocompleteInChannel, error) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsAutocompletionEnabled() {
			listOfAllowedChannels, nErr := s.getListOfAllowedChannelsForTeam(teamID, options.ViewRestrictions)
			if nErr != nil {
				mlog.Warn("Encountered error on AutocompleteUsersInChannel.", mlog.String("search_engine", engine.GetName()), mlog.Err(nErr))
				continue
			}
			if listOfAllowedChannels != nil && len(listOfAllowedChannels) == 0 {
				return &model.UserAutocompleteInChannel{}, nil
			}
			options.ListOfAllowedChannels = listOfAllowedChannels
			autocomplete, nErr := s.autocompleteUsersInChannelByEngine(engine, teamID, channelID, term, options)
			if nErr != nil {
				mlog.Warn("Encountered error on AutocompleteUsersInChannel.", mlog.String("search_engine", engine.GetName()), mlog.Err(nErr))
				continue
			}
			mlog.Debug("Using the first available search engine", mlog.String("search_engine", engine.GetName()))
			return autocomplete, nil
		}
	}

	mlog.Debug("Using database search because no other search engine is available")
	return s.UserStore.AutocompleteUsersInChannel(teamID, channelID, term, options)
}
