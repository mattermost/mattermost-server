// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	model "github.com/mattermost/mattermost-server/v5/model"
	store "github.com/mattermost/mattermost-server/v5/store"
)

type SearchTeamStore struct {
	store.TeamStore
	rootStore *SearchStore
}

func (s SearchTeamStore) SaveMember(teamMember *model.TeamMember, maxUsersPerTeam int) (*model.TeamMember, error) {
	member, err := s.TeamStore.SaveMember(teamMember, maxUsersPerTeam)
	if err == nil {
		s.rootStore.indexUserFromID(member.UserID)
	}
	return member, err
}

func (s SearchTeamStore) UpdateMember(teamMember *model.TeamMember) (*model.TeamMember, error) {
	member, err := s.TeamStore.UpdateMember(teamMember)
	if err == nil {
		s.rootStore.indexUserFromID(member.UserID)
	}
	return member, err
}

func (s SearchTeamStore) RemoveMember(teamID string, userID string) error {
	err := s.TeamStore.RemoveMember(teamID, userID)
	if err == nil {
		s.rootStore.indexUserFromID(userID)
	}
	return err
}

func (s SearchTeamStore) RemoveAllMembersByUser(userID string) error {
	err := s.TeamStore.RemoveAllMembersByUser(userID)
	if err == nil {
		s.rootStore.indexUserFromID(userID)
	}
	return err
}
