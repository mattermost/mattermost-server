// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sharedchannel

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
)

// msgCache caches the work of converting a change in the Posts table to a remote cluster message.
// Maps Post id to syncMsg.
type msgCache map[string]syncMsg

// syncMsg represents a change in content (post add/edit/delete, reaction add/remove, users).
// It is sent to remote clusters as the payload of a `RemoteClusterMsg`.
type syncMsg struct {
	Post      *model.Post       `json:"post"`
	Users     []*model.User     `json:"users"`
	Reactions []*model.Reaction `json:"reactions"`
}

// postsToMsg takes a slice of posts and converts to a `RemoteClusterMsg` which can be
// sent to a remote cluster
func (scs *Service) postsToMsg(posts []*model.Post, cache msgCache) (model.RemoteClusterMsg, error) {
	syncMessages := make([]syncMsg, 0, len(posts))

	for _, p := range posts {
		if sm, ok := cache[p.Id]; ok {
			syncMessages = append(syncMessages, sm)
			continue
		}

		reactions, err := scs.server.GetStore().Reaction().GetForPost(p.Id, true)
		if err != nil {
			return model.RemoteClusterMsg{}, err
		}

		users, err := scs.usersForPost(p)
		if err != nil {
			return model.RemoteClusterMsg{}, err
		}

		sm := syncMsg{
			Post:      p,
			Users:     users,
			Reactions: reactions,
		}
		syncMessages = append(syncMessages, sm)
		cache[p.Id] = sm
	}

	json, err := json.Marshal(syncMessages)
	if err != nil {
		return model.RemoteClusterMsg{}, err
	}

	msg := model.NewRemoteClusterMsg(TopicSync, json)
	return msg, nil
}

// usersForPost provides a list of Users associated with the post that need to be sync'ed.
func (scs *Service) usersForPost(post *model.Post) ([]*model.User, error) {
	users := make([]*model.User, 0)
	creator, err := scs.server.GetStore().User().Get(post.UserId)
	if err == nil {
		users = append(users, creator)
	}

	// extract @mentions?

	return users, nil
}

// usersForPost provides a list of Users associated with the post that need to be sync'ed.
func (scs *Service) shouldUserSync(user *model.User) (bool, error) {
	return false, fmt.Errorf("not implemented yet")

}
