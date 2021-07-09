// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	_, response := th.Client.GetGroup(g.ID, "")
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.GetGroup(g.ID, "")
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	group, response := th.SystemAdminClient.GetGroup(g.ID, "")
	CheckNoError(t, response)

	assert.Equal(t, g.DisplayName, group.DisplayName)
	assert.Equal(t, g.Name, group.Name)
	assert.Equal(t, g.Source, group.Source)
	assert.Equal(t, g.Description, group.Description)
	assert.Equal(t, g.RemoteID, group.RemoteID)
	assert.Equal(t, g.CreateAt, group.CreateAt)
	assert.Equal(t, g.UpdateAt, group.UpdateAt)
	assert.Equal(t, g.DeleteAt, group.DeleteAt)

	_, response = th.SystemAdminClient.GetGroup(model.NewID(), "")
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.GetGroup("12345", "")
	CheckBadRequestStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.GetGroup(group.ID, "")
	CheckUnauthorizedStatus(t, response)
}

func TestPatchGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	updateFmt := "%s_updated"

	newName := fmt.Sprintf(updateFmt, *g.Name)
	newDisplayName := fmt.Sprintf(updateFmt, g.DisplayName)
	newDescription := fmt.Sprintf(updateFmt, g.Description)

	gp := &model.GroupPatch{
		Name:        &newName,
		DisplayName: &newDisplayName,
		Description: &newDescription,
	}

	_, response := th.Client.PatchGroup(g.ID, gp)
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroup(g.ID, gp)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	group2, response := th.SystemAdminClient.PatchGroup(g.ID, gp)
	CheckOKStatus(t, response)

	group, response := th.SystemAdminClient.GetGroup(g.ID, "")
	CheckNoError(t, response)

	assert.Equal(t, *gp.DisplayName, group.DisplayName)
	assert.Equal(t, *gp.DisplayName, group2.DisplayName)
	assert.Equal(t, *gp.Name, *group.Name)
	assert.Equal(t, *gp.Name, *group2.Name)
	assert.Equal(t, *gp.Description, group.Description)
	assert.Equal(t, *gp.Description, group2.Description)

	assert.Equal(t, group2.UpdateAt, group.UpdateAt)

	assert.Equal(t, g.Source, group.Source)
	assert.Equal(t, g.Source, group2.Source)
	assert.Equal(t, g.RemoteID, group.RemoteID)
	assert.Equal(t, g.RemoteID, group2.RemoteID)
	assert.Equal(t, g.CreateAt, group.CreateAt)
	assert.Equal(t, g.CreateAt, group2.CreateAt)
	assert.Equal(t, g.DeleteAt, group.DeleteAt)
	assert.Equal(t, g.DeleteAt, group2.DeleteAt)

	_, response = th.SystemAdminClient.PatchGroup(model.NewID(), gp)
	CheckNotFoundStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.PatchGroup(group.ID, gp)
	CheckUnauthorizedStatus(t, response)
}

func TestLinkGroupTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	_, response := th.Client.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response = th.Client.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.NotNil(t, response.Error)

	th.UpdateUserToTeamAdmin(th.BasicUser, th.BasicTeam)
	th.Client.Logout()
	th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

	groupTeam, response := th.Client.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)
	assert.NotNil(t, groupTeam)
}

func TestLinkGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	_, response := th.Client.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	groupTeam, response := th.Client.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)
	assert.Equal(t, th.BasicChannel.TeamID, groupTeam.TeamID)
	assert.NotNil(t, groupTeam)

	_, response = th.SystemAdminClient.UpdateChannelRoles(th.BasicChannel.ID, th.BasicUser.ID, "")
	require.Nil(t, response.Error)
	th.Client.Logout()
	th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

	_, response = th.Client.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.NotNil(t, response.Error)
}

func TestUnlinkGroupTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)

	th.App.Srv().SetLicense(nil)

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	CheckNotImplementedStatus(t, response)

	response = th.SystemAdminClient.UnlinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	assert.NotNil(t, response.Error)
	time.Sleep(2 * time.Second) // A hack to let "go c.App.SyncRolesAndMembership" finish before moving on.
	th.UpdateUserToTeamAdmin(th.BasicUser, th.BasicTeam)
	ok, response := th.Client.Logout()
	assert.True(t, ok)
	CheckOKStatus(t, response)
	_, response = th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
	CheckOKStatus(t, response)

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	CheckOKStatus(t, response)
}

func TestUnlinkGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)

	th.App.Srv().SetLicense(nil)

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	CheckNotImplementedStatus(t, response)

	response = th.SystemAdminClient.UnlinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response = th.SystemAdminClient.UpdateChannelRoles(th.BasicChannel.ID, th.BasicUser.ID, "")
	require.Nil(t, response.Error)
	th.Client.Logout()
	th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	assert.NotNil(t, response.Error)

	_, response = th.SystemAdminClient.UpdateChannelRoles(th.BasicChannel.ID, th.BasicUser.ID, "channel_admin channel_user")
	require.Nil(t, response.Error)
	th.Client.Logout()
	th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

	response = th.Client.UnlinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	assert.Nil(t, response.Error)
}

func TestGetGroupTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	_, response := th.Client.GetGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	_, response = th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)

	groupSyncable, response := th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckOKStatus(t, response)
	assert.NotNil(t, groupSyncable)

	assert.Equal(t, g.ID, groupSyncable.GroupID)
	assert.Equal(t, th.BasicTeam.ID, groupSyncable.SyncableID)
	assert.Equal(t, *patch.AutoAdd, groupSyncable.AutoAdd)

	_, response = th.SystemAdminClient.GetGroupSyncable(model.NewID(), th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, model.NewID(), model.GroupSyncableTypeTeam, "")
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable("asdfasdfe3", th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckBadRequestStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, "asdfasdfe3", model.GroupSyncableTypeTeam, "")
	CheckBadRequestStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, "")
	CheckUnauthorizedStatus(t, response)
}

func TestGetGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	_, response := th.Client.GetGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	_, response = th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)

	groupSyncable, response := th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckOKStatus(t, response)
	assert.NotNil(t, groupSyncable)

	assert.Equal(t, g.ID, groupSyncable.GroupID)
	assert.Equal(t, th.BasicChannel.ID, groupSyncable.SyncableID)
	assert.Equal(t, *patch.AutoAdd, groupSyncable.AutoAdd)

	_, response = th.SystemAdminClient.GetGroupSyncable(model.NewID(), th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, model.NewID(), model.GroupSyncableTypeChannel, "")
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable("asdfasdfe3", th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckBadRequestStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, "asdfasdfe3", model.GroupSyncableTypeChannel, "")
	CheckBadRequestStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.GetGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, "")
	CheckUnauthorizedStatus(t, response)
}

func TestGetGroupTeams(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	for i := 0; i < 10; i++ {
		team := th.CreateTeam()
		_, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, team.ID, model.GroupSyncableTypeTeam, patch)
		assert.Equal(t, http.StatusCreated, response.StatusCode)
	}

	th.App.Srv().SetLicense(nil)

	_, response := th.Client.GetGroupSyncables(g.ID, model.GroupSyncableTypeTeam, "")
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeTeam, "")
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response = th.Client.GetGroupSyncables(g.ID, model.GroupSyncableTypeTeam, "")
	assert.Equal(t, http.StatusForbidden, response.StatusCode)

	groupSyncables, response := th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeTeam, "")
	CheckOKStatus(t, response)

	assert.Len(t, groupSyncables, 10)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeTeam, "")
	CheckUnauthorizedStatus(t, response)
}

func TestGetGroupChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	for i := 0; i < 10; i++ {
		channel := th.CreatePublicChannel()
		_, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, channel.ID, model.GroupSyncableTypeChannel, patch)
		assert.Equal(t, http.StatusCreated, response.StatusCode)
	}

	th.App.Srv().SetLicense(nil)

	_, response := th.Client.GetGroupSyncables(g.ID, model.GroupSyncableTypeChannel, "")
	CheckNotImplementedStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeChannel, "")
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response = th.Client.GetGroupSyncables(g.ID, model.GroupSyncableTypeChannel, "")
	assert.Equal(t, http.StatusForbidden, response.StatusCode)

	groupSyncables, response := th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeChannel, "")
	CheckOKStatus(t, response)

	assert.Len(t, groupSyncables, 10)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.GetGroupSyncables(g.ID, model.GroupSyncableTypeChannel, "")
	CheckUnauthorizedStatus(t, response)
}

func TestPatchGroupTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	groupSyncable, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)
	assert.NotNil(t, groupSyncable)
	assert.True(t, groupSyncable.AutoAdd)

	_, response = th.Client.PatchGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	assert.Equal(t, http.StatusForbidden, response.StatusCode)

	th.App.Srv().SetLicense(nil)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch.AutoAdd = model.NewBool(false)
	groupSyncable, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckOKStatus(t, response)
	assert.False(t, groupSyncable.AutoAdd)

	assert.Equal(t, g.ID, groupSyncable.GroupID)
	assert.Equal(t, th.BasicTeam.ID, groupSyncable.SyncableID)
	assert.Equal(t, model.GroupSyncableTypeTeam, groupSyncable.Type)

	patch.AutoAdd = model.NewBool(true)
	groupSyncable, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckOKStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(model.NewID(), th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, model.NewID(), model.GroupSyncableTypeTeam, patch)
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable("abc", th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckBadRequestStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, "abc", model.GroupSyncableTypeTeam, patch)
	CheckBadRequestStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam, patch)
	CheckUnauthorizedStatus(t, response)
}

func TestPatchGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	g, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	patch := &model.GroupSyncablePatch{
		AutoAdd: model.NewBool(true),
	}

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	groupSyncable, response := th.SystemAdminClient.LinkGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.Equal(t, http.StatusCreated, response.StatusCode)
	assert.NotNil(t, groupSyncable)
	assert.True(t, groupSyncable.AutoAdd)

	role, err := th.App.GetRoleByName(context.Background(), "channel_user")
	require.Nil(t, err)
	originalPermissions := role.Permissions
	_, err = th.App.PatchRole(role, &model.RolePatch{Permissions: &[]string{}})
	require.Nil(t, err)

	_, response = th.Client.PatchGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	assert.Equal(t, http.StatusForbidden, response.StatusCode)

	_, err = th.App.PatchRole(role, &model.RolePatch{Permissions: &originalPermissions})
	require.Nil(t, err)

	th.App.Srv().SetLicense(nil)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	patch.AutoAdd = model.NewBool(false)
	groupSyncable, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckOKStatus(t, response)
	assert.False(t, groupSyncable.AutoAdd)

	assert.Equal(t, g.ID, groupSyncable.GroupID)
	assert.Equal(t, th.BasicChannel.ID, groupSyncable.SyncableID)
	assert.Equal(t, th.BasicChannel.TeamID, groupSyncable.TeamID)
	assert.Equal(t, model.GroupSyncableTypeChannel, groupSyncable.Type)

	patch.AutoAdd = model.NewBool(true)
	groupSyncable, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckOKStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(model.NewID(), th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, model.NewID(), model.GroupSyncableTypeChannel, patch)
	CheckNotFoundStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable("abc", th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckBadRequestStatus(t, response)

	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, "abc", model.GroupSyncableTypeChannel, patch)
	CheckBadRequestStatus(t, response)

	th.SystemAdminClient.Logout()
	_, response = th.SystemAdminClient.PatchGroupSyncable(g.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel, patch)
	CheckUnauthorizedStatus(t, response)
}

func TestGetGroupsByChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	groupSyncable, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: th.BasicChannel.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group.ID,
	})
	assert.Nil(t, err)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, response := client.GetGroupsByChannel("asdfasdf", opts)
		CheckBadRequestStatus(t, response)
	})

	th.App.Srv().SetLicense(nil)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, response := client.GetGroupsByChannel(th.BasicChannel.ID, opts)
		CheckNotImplementedStatus(t, response)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	privateChannel := th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypePrivate)

	_, _, response := th.Client.GetGroupsByChannel(privateChannel.ID, opts)
	CheckForbiddenStatus(t, response)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		groups, _, response := client.GetGroupsByChannel(th.BasicChannel.ID, opts)
		assert.Nil(t, response.Error)
		assert.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
		require.NotNil(t, groups[0].SchemeAdmin)
		require.False(t, *groups[0].SchemeAdmin)
	})

	// set syncable to true
	groupSyncable.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(groupSyncable)
	require.Nil(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		groups, _, response := client.GetGroupsByChannel(th.BasicChannel.ID, opts)
		assert.Nil(t, response.Error)
		// ensure that SchemeAdmin field is updated
		assert.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(true)}}, groups)
		require.NotNil(t, groups[0].SchemeAdmin)
		require.True(t, *groups[0].SchemeAdmin)

		groups, _, response = client.GetGroupsByChannel(model.NewID(), opts)
		assert.Equal(t, "app.channel.get.existing.app_error", response.Error.ID)
		assert.Empty(t, groups)
	})
}

func TestGetGroupsAssociatedToChannelsByTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	groupSyncable, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: th.BasicChannel.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group.ID,
	})
	assert.Nil(t, err)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	_, response := th.SystemAdminClient.GetGroupsAssociatedToChannelsByTeam("asdfasdf", opts)
	CheckBadRequestStatus(t, response)

	th.App.Srv().SetLicense(nil)

	_, response = th.SystemAdminClient.GetGroupsAssociatedToChannelsByTeam(th.BasicTeam.ID, opts)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	groups, response := th.SystemAdminClient.GetGroupsAssociatedToChannelsByTeam(th.BasicTeam.ID, opts)
	assert.Nil(t, response.Error)

	assert.Equal(t, map[string][]*model.GroupWithSchemeAdmin{
		th.BasicChannel.ID: {
			{Group: *group, SchemeAdmin: model.NewBool(false)},
		},
	}, groups)

	require.NotNil(t, groups[th.BasicChannel.ID][0].SchemeAdmin)
	require.False(t, *groups[th.BasicChannel.ID][0].SchemeAdmin)

	// set syncable to true
	groupSyncable.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(groupSyncable)
	require.Nil(t, err)

	// ensure that SchemeAdmin field is updated
	groups, response = th.SystemAdminClient.GetGroupsAssociatedToChannelsByTeam(th.BasicTeam.ID, opts)
	assert.Nil(t, response.Error)

	assert.Equal(t, map[string][]*model.GroupWithSchemeAdmin{
		th.BasicChannel.ID: {
			{Group: *group, SchemeAdmin: model.NewBool(true)},
		},
	}, groups)

	require.NotNil(t, groups[th.BasicChannel.ID][0].SchemeAdmin)
	require.True(t, *groups[th.BasicChannel.ID][0].SchemeAdmin)

	groups, response = th.SystemAdminClient.GetGroupsAssociatedToChannelsByTeam(model.NewID(), opts)
	assert.Nil(t, response.Error)
	assert.Empty(t, groups)
}

func TestGetGroupsByTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	groupSyncable, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: th.BasicTeam.ID,
		Type:       model.GroupSyncableTypeTeam,
		GroupID:    group.ID,
	})
	assert.Nil(t, err)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, response := client.GetGroupsByTeam("asdfasdf", opts)
		CheckBadRequestStatus(t, response)
	})

	th.App.Srv().SetLicense(nil)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, response := client.GetGroupsByTeam(th.BasicTeam.ID, opts)
		CheckNotImplementedStatus(t, response)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		groups, _, response := client.GetGroupsByTeam(th.BasicTeam.ID, opts)
		assert.Nil(t, response.Error)
		assert.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
		require.NotNil(t, groups[0].SchemeAdmin)
		require.False(t, *groups[0].SchemeAdmin)
	})

	// set syncable to true
	groupSyncable.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(groupSyncable)
	require.Nil(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		groups, _, response := client.GetGroupsByTeam(th.BasicTeam.ID, opts)
		assert.Nil(t, response.Error)
		// ensure that SchemeAdmin field is updated
		assert.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(true)}}, groups)
		require.NotNil(t, groups[0].SchemeAdmin)
		require.True(t, *groups[0].SchemeAdmin)

		groups, _, response = client.GetGroupsByTeam(model.NewID(), opts)
		assert.Nil(t, response.Error)
		assert.Empty(t, groups)
	})
}

func TestGetGroups(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// make sure "createdDate" for next group is after one created in InitBasic()
	time.Sleep(2 * time.Millisecond)
	id := model.NewID()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)
	start := group.UpdateAt - 1

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	th.App.Srv().SetLicense(nil)

	_, response := th.SystemAdminClient.GetGroups(opts)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	_, response = th.SystemAdminClient.GetGroups(opts)
	require.Nil(t, response.Error)

	_, response = th.SystemAdminClient.UpdateChannelRoles(th.BasicChannel.ID, th.BasicUser.ID, "")
	require.Nil(t, response.Error)

	opts.NotAssociatedToChannel = th.BasicChannel.ID

	_, response = th.SystemAdminClient.UpdateChannelRoles(th.BasicChannel.ID, th.BasicUser.ID, "channel_user channel_admin")
	require.Nil(t, response.Error)

	groups, response := th.SystemAdminClient.GetGroups(opts)
	assert.Nil(t, response.Error)
	assert.ElementsMatch(t, []*model.Group{group, th.Group}, groups)
	assert.Nil(t, groups[0].MemberCount)

	opts.IncludeMemberCount = true
	groups, _ = th.SystemAdminClient.GetGroups(opts)
	assert.NotNil(t, groups[0].MemberCount)
	opts.IncludeMemberCount = false

	opts.Q = "-fOo"
	groups, _ = th.SystemAdminClient.GetGroups(opts)
	assert.Len(t, groups, 1)
	opts.Q = ""

	_, response = th.SystemAdminClient.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, "")
	require.Nil(t, response.Error)

	opts.NotAssociatedToTeam = th.BasicTeam.ID

	_, response = th.SystemAdminClient.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, "team_user team_admin")
	require.Nil(t, response.Error)

	_, response = th.Client.GetGroups(opts)
	assert.Nil(t, response.Error)

	// test "since", should only return group created in this test, not th.Group
	opts.Since = start
	groups, response = th.Client.GetGroups(opts)
	assert.Nil(t, response.Error)
	assert.Len(t, groups, 1)
	// test correct group returned
	assert.Equal(t, groups[0].ID, group.ID)

	// delete group, should still return
	th.App.DeleteGroup(group.ID)
	groups, response = th.Client.GetGroups(opts)
	assert.Nil(t, response.Error)
	assert.Len(t, groups, 1)
	assert.Equal(t, groups[0].ID, group.ID)

	// test with current since value, return none
	opts.Since = model.GetMillis()
	groups, response = th.Client.GetGroups(opts)
	assert.Nil(t, response.Error)
	assert.Empty(t, groups)

	// make sure delete group is not returned without Since
	opts.Since = 0
	groups, response = th.Client.GetGroups(opts)
	assert.Nil(t, response.Error)
	//'Normal getGroups should not return delete groups
	assert.Len(t, groups, 1)
	// make sure it returned th.Group,not group
	assert.Equal(t, groups[0].ID, th.Group.ID)
}

func TestGetGroupsByUserID(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	group1, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	user1, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user1", Password: "test-password-1", Username: "test-user-1", Roles: model.SystemUserRoleID})
	assert.Nil(t, err)
	user1.Password = "test-password-1"
	_, err = th.App.UpsertGroupMember(group1.ID, user1.ID)
	assert.Nil(t, err)

	id = model.NewID()
	group2, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	_, err = th.App.UpsertGroupMember(group2.ID, user1.ID)
	assert.Nil(t, err)

	th.App.Srv().SetLicense(nil)
	_, response := th.SystemAdminClient.GetGroupsByUserID(user1.ID)
	CheckNotImplementedStatus(t, response)

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))
	_, response = th.SystemAdminClient.GetGroupsByUserID("")
	CheckBadRequestStatus(t, response)

	_, response = th.SystemAdminClient.GetGroupsByUserID("notvaliduserid")
	CheckBadRequestStatus(t, response)

	groups, response := th.SystemAdminClient.GetGroupsByUserID(user1.ID)
	require.Nil(t, response.Error)
	assert.ElementsMatch(t, []*model.Group{group1, group2}, groups)

	// test permissions
	th.Client.Logout()
	th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
	_, response = th.Client.GetGroupsByUserID(user1.ID)
	CheckForbiddenStatus(t, response)

	th.Client.Logout()
	th.Client.Login(user1.Email, user1.Password)
	groups, response = th.Client.GetGroupsByUserID(user1.ID)
	require.Nil(t, response.Error)
	assert.ElementsMatch(t, []*model.Group{group1, group2}, groups)

}

func TestGetGroupStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteID:    model.NewID(),
	})
	assert.Nil(t, err)

	var response *model.Response
	var stats *model.GroupStats

	t.Run("Requires ldap license", func(t *testing.T) {
		_, response = th.SystemAdminClient.GetGroupStats(group.ID)
		CheckNotImplementedStatus(t, response)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	t.Run("Requires manage system permission to access group stats", func(t *testing.T) {
		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		_, response = th.Client.GetGroupStats(group.ID)
		CheckForbiddenStatus(t, response)
	})

	t.Run("Returns stats for a group with no members", func(t *testing.T) {
		stats, _ = th.SystemAdminClient.GetGroupStats(group.ID)
		assert.Equal(t, stats.GroupID, group.ID)
		assert.Equal(t, stats.TotalMemberCount, int64(0))
	})

	user1, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user1", Password: "test-password-1", Username: "test-user-1", Roles: model.SystemUserRoleID})
	assert.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group.ID, user1.ID)
	assert.Nil(t, err)

	t.Run("Returns stats for a group with members", func(t *testing.T) {
		stats, _ = th.SystemAdminClient.GetGroupStats(group.ID)
		assert.Equal(t, stats.GroupID, group.ID)
		assert.Equal(t, stats.TotalMemberCount, int64(1))
	})
}

func TestGetGroupsGroupConstrainedParentTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	var groups []*model.Group
	for i := 0; i < 4; i++ {
		id := model.NewID()
		group, err := th.App.CreateGroup(&model.Group{
			DisplayName: fmt.Sprintf("dn-foo_%d", i),
			Name:        model.NewString("name" + id),
			Source:      model.GroupSourceLdap,
			Description: "description_" + id,
			RemoteID:    model.NewID(),
		})
		require.Nil(t, err)
		groups = append(groups, group)
	}

	team := th.CreateTeam()

	id := model.NewID()
	channel := &model.Channel{
		DisplayName:      "dn_" + id,
		Name:             "name" + id,
		Type:             model.ChannelTypePrivate,
		TeamID:           team.ID,
		GroupConstrained: model.NewBool(true),
	}
	channel, err := th.App.CreateChannel(th.Context, channel, false)
	require.Nil(t, err)

	// normal result of groups are returned if the team is not group-constrained
	apiGroups, response := th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID})
	require.Nil(t, response.Error)
	require.Contains(t, apiGroups, groups[0])
	require.Contains(t, apiGroups, groups[1])
	require.Contains(t, apiGroups, groups[2])

	team.GroupConstrained = model.NewBool(true)
	team, err = th.App.UpdateTeam(team)
	require.Nil(t, err)

	// team is group-constrained but has no associated groups
	apiGroups, response = th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID, FilterParentTeamPermitted: true})
	require.Nil(t, response.Error)
	require.Len(t, apiGroups, 0)

	for _, group := range []*model.Group{groups[0], groups[2], groups[3]} {
		_, err = th.App.UpsertGroupSyncable(model.NewGroupTeam(group.ID, team.ID, false))
		require.Nil(t, err)
	}

	// set of the teams groups are returned
	apiGroups, response = th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID, FilterParentTeamPermitted: true})
	require.Nil(t, response.Error)
	require.Contains(t, apiGroups, groups[0])
	require.NotContains(t, apiGroups, groups[1])
	require.Contains(t, apiGroups, groups[2])

	// paged results function as expected
	apiGroups, response = th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID, FilterParentTeamPermitted: true, PageOpts: &model.PageOpts{PerPage: 2, Page: 0}})
	require.Nil(t, response.Error)
	require.Len(t, apiGroups, 2)
	require.Equal(t, apiGroups[0].ID, groups[0].ID)
	require.Equal(t, apiGroups[1].ID, groups[2].ID)

	apiGroups, response = th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID, FilterParentTeamPermitted: true, PageOpts: &model.PageOpts{PerPage: 2, Page: 1}})
	require.Nil(t, response.Error)
	require.Len(t, apiGroups, 1)
	require.Equal(t, apiGroups[0].ID, groups[3].ID)

	_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(groups[0].ID, channel.ID, false))
	require.Nil(t, err)

	// as usual it doesn't return groups already associated to the channel
	apiGroups, response = th.SystemAdminClient.GetGroups(model.GroupSearchOpts{NotAssociatedToChannel: channel.ID})
	require.Nil(t, response.Error)
	require.NotContains(t, apiGroups, groups[0])
	require.Contains(t, apiGroups, groups[2])
}
