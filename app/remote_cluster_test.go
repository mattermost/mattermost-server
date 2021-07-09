// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

func TestAddRemoteCluster(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("adding remote cluster with duplicate site url and remote team id", func(t *testing.T) {
		remoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test1",
			SiteURL:      "http://www1.example.com:8065",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		_, err := th.App.AddRemoteCluster(remoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		remoteCluster.RemoteID = model.NewID()
		_, err = th.App.AddRemoteCluster(remoteCluster)
		require.NotNil(t, err, "Adding a duplicate remote cluster should error")
		assert.Contains(t, err.Error(), i18n.T("api.remote_cluster.save_not_unique.app_error"))
	})

	t.Run("adding remote cluster with duplicate site url or remote team id is allowed", func(t *testing.T) {
		remoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test2",
			SiteURL:      "http://www2.exmaple.com:8065",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		existingRemoteCluster, err := th.App.AddRemoteCluster(remoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		// Same site url but different remote team id
		remoteCluster.RemoteID = model.NewID()
		remoteCluster.RemoteTeamID = model.NewID()
		remoteCluster.SiteURL = existingRemoteCluster.SiteURL
		_, err = th.App.AddRemoteCluster(remoteCluster)
		assert.Nil(t, err, "Adding a remote cluster should not error")

		// Same remote team id but different site url
		remoteCluster.RemoteID = model.NewID()
		remoteCluster.RemoteTeamID = existingRemoteCluster.RemoteTeamID
		remoteCluster.SiteURL = existingRemoteCluster.SiteURL + "/new"
		_, err = th.App.AddRemoteCluster(remoteCluster)
		assert.Nil(t, err, "Adding a remote cluster should not error")
	})
}

func TestUpdateRemoteCluster(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("update remote cluster with an already existing site url and team id", func(t *testing.T) {
		remoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test3",
			SiteURL:      "http://www3.exmaple.com:8065",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		otherRemoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test4",
			SiteURL:      "http://www4.example.com:8066",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		_, err := th.App.AddRemoteCluster(remoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		savedRemoteClustered, err := th.App.AddRemoteCluster(otherRemoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		savedRemoteClustered.SiteURL = remoteCluster.SiteURL
		savedRemoteClustered.RemoteTeamID = remoteCluster.RemoteTeamID
		_, err = th.App.UpdateRemoteCluster(savedRemoteClustered)
		require.NotNil(t, err, "Updating remote cluster with duplicate site url should error")
		assert.Contains(t, err.Error(), i18n.T("api.remote_cluster.update_not_unique.app_error"))
	})

	t.Run("update remote cluster with an already existing site url or team id, is allowed", func(t *testing.T) {
		remoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test5",
			SiteURL:      "http://www5.example.com:8065",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		otherRemoteCluster := &model.RemoteCluster{
			RemoteTeamID: model.NewID(),
			Name:         "test6",
			SiteURL:      "http://www6.example.com:8065",
			Token:        model.NewID(),
			RemoteToken:  model.NewID(),
			Topics:       "",
			CreatorID:    th.BasicUser.ID,
		}

		existingRemoteCluster, err := th.App.AddRemoteCluster(remoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		anotherExistingRemoteClustered, err := th.App.AddRemoteCluster(otherRemoteCluster)
		require.Nil(t, err, "Adding a remote cluster should not error")

		// Same site url but different remote team id
		anotherExistingRemoteClustered.SiteURL = existingRemoteCluster.SiteURL
		anotherExistingRemoteClustered.RemoteTeamID = model.NewID()
		_, err = th.App.UpdateRemoteCluster(anotherExistingRemoteClustered)
		assert.Nil(t, err, "Updating remote cluster should not error")

		// Same remote team id but different site url
		anotherExistingRemoteClustered.SiteURL = existingRemoteCluster.SiteURL + "/new"
		anotherExistingRemoteClustered.RemoteTeamID = existingRemoteCluster.RemoteTeamID
		_, err = th.App.UpdateRemoteCluster(anotherExistingRemoteClustered)
		assert.Nil(t, err, "Updating remote cluster should not error")
	})
}
