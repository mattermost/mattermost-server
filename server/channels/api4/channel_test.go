// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/v8/channels/web"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/mattermost/mattermost/server/v8/channels/utils/testutils"
	einterfacesmocks "github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
)

func TestCreateChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamId: team.Id}

	rchannel, resp, err := client.CreateChannel(context.Background(), channel)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	require.Equal(t, channel.Name, rchannel.Name, "names did not match")
	require.Equal(t, channel.DisplayName, rchannel.DisplayName, "display names did not match")
	require.Equal(t, channel.TeamId, rchannel.TeamId, "team ids did not match")

	rprivate, _, err := client.CreateChannel(context.Background(), private)
	require.NoError(t, err)

	require.Equal(t, private.Name, rprivate.Name, "names did not match")
	require.Equal(t, model.ChannelTypePrivate, rprivate.Type, "wrong channel type")
	require.Equal(t, th.BasicUser.Id, rprivate.CreatorId, "wrong creator id")

	_, resp, err = client.CreateChannel(context.Background(), channel)
	CheckErrorID(t, err, "store.sql_channel.save_channel.exists.app_error")
	CheckBadRequestStatus(t, resp)

	direct := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeDirect, TeamId: team.Id}
	_, resp, err = client.CreateChannel(context.Background(), direct)
	CheckErrorID(t, err, "api.channel.create_channel.direct_channel.app_error")
	CheckBadRequestStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.CreateChannel(context.Background(), channel)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	userNotOnTeam := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), userNotOnTeam.Email, userNotOnTeam.Password)
	require.NoError(t, err)

	_, resp, err = client.CreateChannel(context.Background(), channel)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.CreateChannel(context.Background(), private)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer func() {
		th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
	}()

	th.AddPermissionToRole(t, model.PermissionCreatePublicChannel.Id, model.TeamUserRoleId)
	th.AddPermissionToRole(t, model.PermissionCreatePrivateChannel.Id, model.TeamUserRoleId)

	th.LoginBasic(t)

	channel.Name = GenerateTestChannelName()
	_, _, err = client.CreateChannel(context.Background(), channel)
	require.NoError(t, err)

	private.Name = GenerateTestChannelName()
	_, _, err = client.CreateChannel(context.Background(), private)
	require.NoError(t, err)

	th.AddPermissionToRole(t, model.PermissionCreatePublicChannel.Id, model.TeamAdminRoleId)
	th.AddPermissionToRole(t, model.PermissionCreatePrivateChannel.Id, model.TeamAdminRoleId)
	th.RemovePermissionFromRole(t, model.PermissionCreatePublicChannel.Id, model.TeamUserRoleId)
	th.RemovePermissionFromRole(t, model.PermissionCreatePrivateChannel.Id, model.TeamUserRoleId)

	_, resp, err = client.CreateChannel(context.Background(), channel)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.CreateChannel(context.Background(), private)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin(t)

	channel.Name = GenerateTestChannelName()
	_, _, err = client.CreateChannel(context.Background(), channel)
	require.NoError(t, err)

	private.Name = GenerateTestChannelName()
	_, _, err = client.CreateChannel(context.Background(), private)
	require.NoError(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channel.Name = GenerateTestChannelName()
		_, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		private.Name = GenerateTestChannelName()
		_, _, err = client.CreateChannel(context.Background(), private)
		require.NoError(t, err)
	})

	t.Run("null value", func(t *testing.T) {
		var channel *model.Channel
		_, resp, err = client.CreateChannel(context.Background(), channel)

		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	// Test posting Garbage
	r, err := client.DoAPIPost(context.Background(), "/channels", "garbage")
	require.Error(t, err, "expected error")
	require.Equal(t, http.StatusBadRequest, r.StatusCode, "Expected 400 Bad Request")

	// Test GroupConstrained flag
	groupConstrainedChannel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id, GroupConstrained: new(true)}
	rchannel, _, err = client.CreateChannel(context.Background(), groupConstrainedChannel)
	require.NoError(t, err)

	require.Equal(t, *groupConstrainedChannel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")

	t.Run("Test create channel with missing team id", func(t *testing.T) {
		channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: ""}

		_, resp, err = client.CreateChannel(context.Background(), channel)
		CheckErrorID(t, err, "api.context.invalid_body_param.app_error")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Test create channel with missing display name", func(t *testing.T) {
		channel := &model.Channel{DisplayName: "", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id}

		_, resp, err = client.CreateChannel(context.Background(), channel)
		CheckErrorID(t, err, "api.context.invalid_body_param.app_error")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Can create channel with banner info", func(t *testing.T) {
		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		var createdChannel *model.Channel
		createdChannel, resp, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.True(t, *createdChannel.BannerInfo.Enabled)
		require.Equal(t, "banner text", *createdChannel.BannerInfo.Text)
		require.Equal(t, "#dddddd", *createdChannel.BannerInfo.BackgroundColor)
	})

	t.Run("Cannot create channel with banner enabled but not configured", func(t *testing.T) {
		channel := &model.Channel{
			DisplayName: "",
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
			BannerInfo: &model.ChannelBannerInfo{
				Enabled: new(true),
			},
		}

		_, resp, err = client.CreateChannel(context.Background(), channel)
		CheckErrorID(t, err, "api.context.invalid_body_param.app_error")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("should override channel name with server-generated ID when UseAnonymousURLs is enabled and not otherwise", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.UseAnonymousURLs = true })

		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		originalName := GenerateTestChannelName()
		ch := &model.Channel{DisplayName: "Anonymous URL Channel", Name: originalName, Type: model.ChannelTypeOpen, TeamId: team.Id}
		createdChannel, response, err := th.SystemAdminClient.CreateChannel(context.Background(), ch)
		require.NoError(t, err)
		CheckCreatedStatus(t, response)

		require.NotEqual(t, originalName, createdChannel.Name, "channel name should be overridden by server")
		require.True(t, model.IsValidId(createdChannel.Name))
		require.Equal(t, "Anonymous URL Channel", createdChannel.DisplayName, "display name should remain unchanged")

		// setting UseAnonymousURLs to false should preserve names
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.UseAnonymousURLs = false })

		ch = &model.Channel{DisplayName: "Regular Channel", Name: originalName, Type: model.ChannelTypeOpen, TeamId: team.Id}
		createdChannel, response, err = th.SystemAdminClient.CreateChannel(context.Background(), ch)
		require.NoError(t, err)
		CheckCreatedStatus(t, response)
		require.Equal(t, originalName, createdChannel.Name)

		// setting license to something other than Enterprise Advanced should also preserve team name
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.UseAnonymousURLs = true })

		originalName = GenerateTestChannelName()
		ch = &model.Channel{DisplayName: "Regular Channel", Name: originalName, Type: model.ChannelTypeOpen, TeamId: team.Id}
		createdChannel, response, err = th.SystemAdminClient.CreateChannel(context.Background(), ch)
		require.NoError(t, err)
		CheckCreatedStatus(t, response)
		require.Equal(t, originalName, createdChannel.Name)
	})

	t.Run("Guest users", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		guestUser := th.CreateUser(t)
		appErr := th.App.VerifyUserEmail(th.Context, guestUser.Id, guestUser.Email)
		require.Nil(t, appErr)

		appErr = th.App.DemoteUserToGuest(th.Context, guestUser)
		require.Nil(t, appErr)

		_, _, appErr = th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, guestUser.Id, "")
		require.Nil(t, appErr)

		guestClient := th.CreateClient()
		_, _, err := guestClient.Login(context.Background(), guestUser.Username, guestUser.Password)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, lErr := guestClient.Logout(context.Background())
			require.NoError(t, lErr)
		})

		userOutsideOfChannels := th.CreateUser(t)
		_, _, err = th.Client.AddTeamMember(context.Background(), team.Id, userOutsideOfChannels.Id)
		require.NoError(t, err)

		public := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id}
		private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamId: team.Id}

		t.Run("Guest user should not be able to create channels", func(t *testing.T) {
			_, resp, err = guestClient.CreateChannel(context.Background(), public)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			private.Name = GenerateTestChannelName()
			_, resp, err = guestClient.CreateChannel(context.Background(), private)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("Guest user should not be able to add channel members if they have no common channels", func(t *testing.T) {
			// Now actually create the channels with the main client
			public, _, err = th.Client.CreateChannel(context.Background(), public)
			require.NoError(t, err)
			private, _, err = th.Client.CreateChannel(context.Background(), private)
			require.NoError(t, err)

			// Add the guest user to the private channel
			_, _, err = th.Client.AddChannelMember(context.Background(), private.Id, guestUser.Id)
			require.NoError(t, err)

			// Verify that the guest user can access the private channel they were added to
			_, _, err = guestClient.GetChannel(context.Background(), private.Id)
			require.NoError(t, err)

			// Verify that the guest user cannot add members to the private channel
			_, resp, err = guestClient.AddChannelMember(context.Background(), private.Id, userOutsideOfChannels.Id)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			// Add the guest user to the public channel
			_, _, err = th.Client.AddChannelMember(context.Background(), public.Id, guestUser.Id)
			require.NoError(t, err)

			// Verify that the guest user can access the public channel they were added to
			_, _, err = guestClient.GetChannel(context.Background(), public.Id)
			require.NoError(t, err)

			// Verify that the guest user cannot add members to the public channel
			_, resp, err = guestClient.AddChannelMember(context.Background(), public.Id, userOutsideOfChannels.Id)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			// Update team guest permissions to allow creating private channels
			th.AddPermissionToRole(t, model.PermissionCreatePrivateChannel.Id, model.TeamGuestRoleId)
			privateGuest := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamId: team.Id}
			privateGuest, resp, err = guestClient.CreateChannel(context.Background(), privateGuest)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)

			// Verify that the guest user can't add users they have no visibility to
			_, resp, err = guestClient.AddChannelMember(context.Background(), privateGuest.Id, userOutsideOfChannels.Id)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
		})
	})
}

func TestCreateChannelWithPropertyValues(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
		cfg.FeatureFlags.ChannelAttributesRequired = true
	}).InitBasic(t)

	// The attribute validation hook is registered against the access_control
	// group only, and channel attributes need Enterprise Advanced.
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	memberLevel := model.PermissionLevelMember
	sysadminLevel := model.PermissionLevelSysadmin

	// Required attributes are system-wide, so one left behind by a subtest would
	// have to be satisfied by every later one. Each field lives for one subtest.
	// Fields are seeded through the property service: App.CreatePropertyField
	// refuses one created already required while any active channel lacks a value.
	createField := func(t *testing.T, fieldType model.PropertyFieldType, level model.PermissionLevel, attrs model.StringInterface) *model.PropertyField {
		t.Helper()
		field, fieldErr := th.App.Srv().PropertyService().CreatePropertyField(th.Context, &model.PropertyField{
			Name:              "f_" + model.NewId(),
			Type:              fieldType,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &level,
			PermissionOptions: &memberLevel,
			Attrs:             attrs,
		})
		require.NoError(t, fieldErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, field.ID, true, ""))
		})
		return field
	}

	newRequest := func(items ...model.PropertyValuePatchItem) (*model.ChannelCreateRequest, string) {
		name := "attr-" + model.NewId()
		return &model.ChannelCreateRequest{
			Channel: model.Channel{
				DisplayName: "Attributes",
				Name:        name,
				Type:        model.ChannelTypeOpen,
				TeamId:      th.BasicTeam.Id,
			},
			PropertyValues: items,
		}, name
	}

	valuesFor := func(t *testing.T, channelID string) []*model.PropertyValue {
		t.Helper()
		values, valuesErr := th.App.SearchPropertyValues(th.Context, group.ID, model.PropertyValueSearchOpts{
			TargetType: model.PropertyValueTargetTypeChannel,
			TargetIDs:  []string{channelID},
			PerPage:    20,
		})
		require.Nil(t, valuesErr)
		return values
	}

	requireNoSuchChannel := func(t *testing.T, name string) {
		t.Helper()
		_, chErr := th.App.GetChannelByName(th.Context, name, th.BasicTeam.Id, false)
		require.NotNil(t, chErr, "the channel must not survive a failed attribute write")
	}

	th.LoginBasic(t)

	t.Run("a required attribute with no value is refused, without naming it", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		req, name := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.channel.create_channel.missing_required_attributes.app_error")
		require.NotContains(t, err.Error(), field.Name)
		requireNoSuchChannel(t, name)
	})

	t.Run("a required attribute present only as an empty value is refused", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		req, name := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"   "`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		requireNoSuchChannel(t, name)
	})

	t.Run("a required attribute with a value is stored with the channel", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		req, _ := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"SECRET"`)})
		channel, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		values := valuesFor(t, channel.Id)
		require.Len(t, values, 1)
		require.Equal(t, field.ID, values[0].FieldID)
		require.JSONEq(t, `"SECRET"`, string(values[0].Value))
	})

	t.Run("a required attribute the caller cannot set does not block creation", func(t *testing.T) {
		// Otherwise one sysadmin-only required attribute makes channel creation
		// impossible for everyone else, with a 400 they cannot act on.
		createField(t, model.PropertyFieldTypeText, sysadminLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		req, _ := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})

	t.Run("the field lookup names the caller", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, nil)
		untaggedCalls := observeUntaggedPropertyCalls(t, th)

		req, _ := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"SECRET"`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Empty(t, untaggedCalls())
	})

	t.Run("a value for an unknown field is refused and creates nothing", func(t *testing.T) {
		req, name := newRequest(model.PropertyValuePatchItem{FieldID: model.NewId(), Value: json.RawMessage(`"x"`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.channel.create_channel.invalid_attribute.app_error")
		requireNoSuchChannel(t, name)
	})

	t.Run("a value the hook rejects leaves no channel behind", func(t *testing.T) {
		// A select field only accepts one of its own option IDs, and that check
		// runs after the channel row is inserted — so this is the compensation path.
		field := createField(t, model.PropertyFieldTypeSelect, memberLevel, model.StringInterface{
			model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Alpha"}},
		})

		req, name := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"not-an-option"`)})
		_, _, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		requireNoSuchChannel(t, name)
	})

	t.Run("classification value is accepted even when ChannelAttributes is off", func(t *testing.T) {
		// Classification predates channel attributes and ships behind
		// ClassificationMarkings + Enterprise, not ChannelAttributes + Enterprise
		// Advanced — so it must still work with the newer flag off.
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = false
			cfg.FeatureFlags.ClassificationMarkings = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = true
		})

		optionID := model.NewId()
		templateField, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       "classification",
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "template",
			TargetType: "system",
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": optionID, "name": "SECRET"}},
			},
		}, false, "")
		require.Nil(t, fieldErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, templateField.ID, true, ""))
		})

		classificationField, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:             "classification",
			Type:             model.PropertyFieldTypeSelect,
			GroupID:          group.ID,
			ObjectType:       "channel",
			TargetType:       "system",
			PermissionField:  &memberLevel,
			PermissionValues: &memberLevel,
			LinkedFieldID:    &templateField.ID,
		}, false, "")
		require.Nil(t, fieldErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, classificationField.ID, true, ""))
		})

		req, _ := newRequest(model.PropertyValuePatchItem{FieldID: classificationField.ID, Value: json.RawMessage(`"` + optionID + `"`)})
		channel, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		values := valuesFor(t, channel.Id)
		require.Len(t, values, 1)
		require.Equal(t, classificationField.ID, values[0].FieldID)
	})

	t.Run("a required classification value is not enforced when ChannelAttributes is off", func(t *testing.T) {
		// requiredAttributesEnforced folds the ChannelAttributes umbrella flag
		// in, and classification gets no special treatment from it: it is
		// enforced or not by the same single condition as every other field.
		//
		// The field itself is authored first, under default (enforced)
		// conditions -- a channel field cannot be newly marked required once
		// enforcement is already off, so this mirrors the real scenario the
		// flag exists for: a field configured before, disabled after.
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ClassificationMarkings = true
		})

		optionID := model.NewId()
		templateField, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       "classification",
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "template",
			TargetType: "system",
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": optionID, "name": "SECRET"}},
			},
		}, false, "")
		require.Nil(t, fieldErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, templateField.ID, true, ""))
		})

		classificationField, classificationErr := th.App.Srv().PropertyService().CreatePropertyField(th.Context, &model.PropertyField{
			Name:             "classification",
			Type:             model.PropertyFieldTypeSelect,
			GroupID:          group.ID,
			ObjectType:       "channel",
			TargetType:       "system",
			PermissionField:  &memberLevel,
			PermissionValues: &memberLevel,
			LinkedFieldID:    &templateField.ID,
			Attrs: model.StringInterface{
				model.PropertyFieldAttrRequired: true,
			},
		})
		require.NoError(t, classificationErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, classificationField.ID, true, ""))
		})

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = true
		})

		req, _ := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})

	t.Run("a non-classification value is still refused when ChannelAttributes is off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = false
			cfg.FeatureFlags.ClassificationMarkings = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = true
		})

		field := createField(t, model.PropertyFieldTypeText, memberLevel, nil)

		req, name := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"x"`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.channel.create_channel.attributes_feature_disabled.app_error")
		requireNoSuchChannel(t, name)
	})

	t.Run("values are refused when the feature flag is off", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, nil)

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = true
		})

		req, name := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"x"`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.channel.create_channel.attributes_feature_disabled.app_error")
		requireNoSuchChannel(t, name)
	})

	t.Run("values are refused below Enterprise Advanced", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, memberLevel, nil)

		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

		req, name := newRequest(model.PropertyValuePatchItem{FieldID: field.ID, Value: json.RawMessage(`"x"`)})
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.channel.create_channel.attributes_feature_disabled.app_error")
		requireNoSuchChannel(t, name)
	})

	t.Run("a required attribute is not enforced when the feature flag is off", func(t *testing.T) {
		// Creation has to stay byte-identical to its pre-feature behaviour, or
		// turning the flag on becomes the only safe state for every integration.
		createField(t, model.PropertyFieldTypeText, memberLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributes = true
		})

		req, _ := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})

	t.Run("a required attribute with no value is allowed when ChannelAttributesRequired is off", func(t *testing.T) {
		createField(t, model.PropertyFieldTypeText, memberLevel, model.StringInterface{
			model.PropertyFieldAttrRequired: true,
		})

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributesRequired = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributesRequired = true
		})

		req, _ := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})

	t.Run("a required classification value is not enforced either when ChannelAttributesRequired is off", func(t *testing.T) {
		// The field is authored first with enforcement on — a channel field
		// cannot be newly marked required once enforcement is off, so this
		// mirrors the real scenario: configured before, enforcement disabled after.
		optionID := model.NewId()
		templateField, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       "classification",
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "template",
			TargetType: "system",
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": optionID, "name": "SECRET"}},
			},
		}, false, "")
		require.Nil(t, fieldErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, templateField.ID, true, ""))
		})

		classificationField, classificationErr := th.App.Srv().PropertyService().CreatePropertyField(th.Context, &model.PropertyField{
			Name:             "classification",
			Type:             model.PropertyFieldTypeSelect,
			GroupID:          group.ID,
			ObjectType:       "channel",
			TargetType:       "system",
			PermissionField:  &memberLevel,
			PermissionValues: &memberLevel,
			LinkedFieldID:    &templateField.ID,
			Attrs: model.StringInterface{
				model.PropertyFieldAttrRequired: true,
			},
		})
		require.NoError(t, classificationErr)
		t.Cleanup(func() {
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, classificationField.ID, true, ""))
		})

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributesRequired = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.ChannelAttributesRequired = true
		})

		req, _ := newRequest()
		_, resp, err := th.Client.CreateChannelWithPropertyValues(context.Background(), req)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})
}

// The tiers that need a session are covered by TestCreateChannelWithPropertyValues.
// This covers the ones that must answer no without consulting one, because on the
// create path this function is the only gate: a field the property API would refuse
// a value for must not become settable by creating a channel.
func TestCanSetChannelAttributeOnCreate(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	c := &Context{App: th.App, AppContext: th.Context}

	member := model.PermissionLevelMember
	admin := model.PermissionLevelAdmin
	none := model.PermissionLevelNone
	unknown := model.PermissionLevel("supervisor")

	t.Run("member and admin are satisfied by creation itself", func(t *testing.T) {
		require.True(t, canSetChannelAttributeOnCreate(c, &model.PropertyField{PermissionValues: &member}))
		require.True(t, canSetChannelAttributeOnCreate(c, &model.PropertyField{PermissionValues: &admin}))
	})

	t.Run("none is never settable", func(t *testing.T) {
		require.False(t, canSetChannelAttributeOnCreate(c, &model.PropertyField{PermissionValues: &none}))
	})

	t.Run("an unset tier is not settable", func(t *testing.T) {
		require.False(t, canSetChannelAttributeOnCreate(c, &model.PropertyField{}))
	})

	t.Run("an unrecognised tier is not settable", func(t *testing.T) {
		require.False(t, canSetChannelAttributeOnCreate(c, &model.PropertyField{PermissionValues: &unknown}))
	})

	sysadmin := model.PermissionLevelSysadmin

	t.Run("sysadmin tier is settable by a caller with manage_system", func(t *testing.T) {
		sysadminContext := &Context{App: th.App, AppContext: th.Context.WithSession(&model.Session{
			Roles: model.SystemAdminRoleId,
		})}
		require.True(t, canSetChannelAttributeOnCreate(sysadminContext, &model.PropertyField{PermissionValues: &sysadmin}))
	})

	t.Run("sysadmin tier is not settable by a caller without manage_system", func(t *testing.T) {
		memberContext := &Context{App: th.App, AppContext: th.Context.WithSession(&model.Session{
			Roles: model.SystemUserRoleId,
		})}
		require.False(t, canSetChannelAttributeOnCreate(memberContext, &model.PropertyField{PermissionValues: &sysadmin}))
	})
}

func TestCreateChannelRejectsSpaceType(t *testing.T) {
	mainHelper.Parallel(t)

	for _, enableDocs := range []bool{false, true} {
		t.Run(fmt.Sprintf("EnableDocs=%v", enableDocs), func(t *testing.T) {
			th := SetupConfig(t, func(cfg *model.Config) {
				cfg.FeatureFlags.EnableDocs = enableDocs
			}).InitBasic(t)

			space := &model.Channel{
				DisplayName: "Space",
				Name:        "space-" + model.NewId(),
				Type:        model.ChannelTypeSpace,
				TeamId:      th.BasicTeam.Id,
			}
			_, resp, err := th.Client.CreateChannel(context.Background(), space)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		})
	}
}

func TestCreateChannelManagedCategory(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)
	th.ConfigStore.SetReadOnlyFF(false)
	t.Cleanup(func() {
		th.ConfigStore.SetReadOnlyFF(true)
	})
	client := th.Client
	team := th.BasicTeam

	t.Run("should ignore managed category when no enterprise license", func(t *testing.T) {
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)

		channel := &model.Channel{
			DisplayName:         "Managed No License",
			Name:                GenerateTestChannelName(),
			Type:                model.ChannelTypeOpen,
			TeamId:              team.Id,
			ManagedCategoryName: "Operations",
		}
		created, resp, err := client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Empty(t, created.ManagedCategoryName, "managed category should be cleared without license")
	})

	t.Run("should ignore managed category when feature is disabled", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ManagedChannelCategories = false })
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		channel := &model.Channel{
			DisplayName:         "Managed Disabled Feature",
			Name:                GenerateTestChannelName(),
			Type:                model.ChannelTypeOpen,
			TeamId:              team.Id,
			ManagedCategoryName: "Operations",
		}
		created, resp, err := client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Empty(t, created.ManagedCategoryName, "managed category should be cleared when feature is disabled")
	})

	t.Run("should set managed category when feature is enabled with license", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ManagedChannelCategories = true })
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		channel := &model.Channel{
			DisplayName:         "Managed Enabled",
			Name:                GenerateTestChannelName(),
			Type:                model.ChannelTypeOpen,
			TeamId:              team.Id,
			ManagedCategoryName: "Operations",
		}
		created, resp, err := client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Equal(t, "Operations", created.ManagedCategoryName)

		resp2, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", team.Id), "")
		require.NoError(t, err)
		defer resp2.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&mappings))
		assert.Equal(t, "Operations", mappings[created.Id])
	})
}

func TestUpdateChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamId: team.Id}

	channel, _, err := client.CreateChannel(context.Background(), channel)
	require.NoError(t, err)
	private, _, err = client.CreateChannel(context.Background(), private)
	require.NoError(t, err)

	// Update a open channel
	channel.DisplayName = "My new display name"
	channel.Header = "My fancy header"
	channel.Purpose = "Mattermost ftw!"

	newChannel, _, err := client.UpdateChannel(context.Background(), channel)
	require.NoError(t, err)

	require.Equal(t, channel.DisplayName, newChannel.DisplayName, "Update failed for DisplayName")
	require.Equal(t, channel.Header, newChannel.Header, "Update failed for Header")
	require.Equal(t, channel.Purpose, newChannel.Purpose, "Update failed for Purpose")

	// Test GroupConstrained flag
	channel.GroupConstrained = new(true)
	rchannel, resp, err := client.UpdateChannel(context.Background(), channel)
	require.NoError(t, err)
	CheckOKStatus(t, resp)

	require.Equal(t, *channel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")

	// Update a private channel
	private.DisplayName = "My new display name for private channel"
	private.Header = "My fancy private header"
	private.Purpose = "Mattermost ftw! in private mode"

	newPrivateChannel, _, err := client.UpdateChannel(context.Background(), private)
	require.NoError(t, err)

	require.Equal(t, private.DisplayName, newPrivateChannel.DisplayName, "Update failed for DisplayName in private channel")
	require.Equal(t, private.Header, newPrivateChannel.Header, "Update failed for Header in private channel")
	require.Equal(t, private.Purpose, newPrivateChannel.Purpose, "Update failed for Purpose in private channel")

	// Test updating default channel's name and returns error
	defaultChannel, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)
	defaultChannel.Name = "testing"
	_, resp, err = client.UpdateChannel(context.Background(), defaultChannel)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// Test that changing the type fails and returns error
	private.Type = model.ChannelTypeOpen
	_, resp, err = client.UpdateChannel(context.Background(), private)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// Test that keeping the same type succeeds

	private.Type = model.ChannelTypePrivate
	_, _, err = client.UpdateChannel(context.Background(), private)
	require.NoError(t, err)

	// Non existing channel
	channel1 := &model.Channel{DisplayName: "Test API Name for apiv4", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id}
	_, resp, err = client.UpdateChannel(context.Background(), channel1)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	// Try to update with not logged user
	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.UpdateChannel(context.Background(), channel)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	// Try to update using another user
	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)

	channel.DisplayName = "Should not update"
	_, resp, err = client.UpdateChannel(context.Background(), channel)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser(t)
	user2 := th.CreateUser(t)
	user3 := th.CreateUser(t)

	groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id})
	require.NoError(t, err)

	groupChannel.Header = "lolololol"
	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = client.Login(context.Background(), user3.Email, user3.Password)
	require.NoError(t, err)
	_, resp, err = client.UpdateChannel(context.Background(), groupChannel)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)

	directChannel, _, err := client.CreateDirectChannel(context.Background(), user.Id, user1.Id)
	require.NoError(t, err)

	directChannel.Header = "lolololol"
	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = client.Login(context.Background(), user3.Email, user3.Password)
	require.NoError(t, err)
	_, resp, err = client.UpdateChannel(context.Background(), directChannel)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	t.Run("null value", func(t *testing.T) {
		r, err := client.DoAPIPut(context.Background(), fmt.Sprintf("/channels"+"/%v", channel.Id), "null")
		resp := model.BuildResponse(r)
		defer closeBody(r)

		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
	t.Run("Should block changes to name, display name or purpose for group messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)
		user3 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		updatedChannel := &model.Channel{Id: groupChannel.Id, Name: "test name"}
		_, resp, err := client.UpdateChannel(context.Background(), updatedChannel)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		updatedChannel2 := &model.Channel{Id: groupChannel.Id, DisplayName: "test display name"}
		_, resp, err = client.UpdateChannel(context.Background(), updatedChannel2)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		updatedChannel3 := &model.Channel{Id: groupChannel.Id, Purpose: "test purpose"}
		_, resp, err = client.UpdateChannel(context.Background(), updatedChannel3)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should block changes to name, display name or purpose for direct messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		directChannel, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
		require.NoError(t, err)

		updatedChannel := &model.Channel{Id: directChannel.Id, Name: "test name"}
		_, resp, err := client.UpdateChannel(context.Background(), updatedChannel)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		updatedChannel2 := &model.Channel{Id: directChannel.Id, DisplayName: "test display name"}
		_, resp, err = client.UpdateChannel(context.Background(), updatedChannel2)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		updatedChannel3 := &model.Channel{Id: directChannel.Id, Purpose: "test purpose"}
		_, resp, err = client.UpdateChannel(context.Background(), updatedChannel3)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestPatchChannelGroupConstrained(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	t.Run("Test GroupConstrained flag", func(t *testing.T) {
		// Test GroupConstrained flag
		patch := &model.ChannelPatch{}
		patch.GroupConstrained = new(true)
		rchannel, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Equal(t, *rchannel.GroupConstrained, *patch.GroupConstrained, "GroupConstrained flags do not match")
		patch.GroupConstrained = nil

		_, resp, err = client.PatchChannel(context.Background(), "junk", patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.PatchChannel(context.Background(), model.NewId(), patch)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)

		user := th.CreateUser(t)
		_, _, err = client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)

		patch.GroupConstrained = new(false)

		_, resp, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, _, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
			require.NoError(t, err)

			_, _, err = client.PatchChannel(context.Background(), th.BasicPrivateChannel.Id, patch)
			require.NoError(t, err)
		})
	})

	t.Run("Test GroupConstrained flag set to true and non group members are removed", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		// Create a test group
		group := th.CreateGroup(t)

		// Create a channel and set it as group-constrained
		channel := th.CreatePrivateChannel(t)
		// Add user to the channel
		th.AddUserToChannel(t, th.BasicUser2, channel)

		// Create a group user
		groupUser := th.CreateUser(t)
		th.LinkUserToTeam(t, groupUser, th.BasicTeam)

		// Create a group member
		_, appErr := th.App.UpsertGroupMember(group.Id, groupUser.Id)
		require.Nil(t, appErr)

		// Associate the group with the channel
		autoAdd := true
		schemeAdmin := true
		_, r, err := th.SystemAdminClient.LinkGroupSyncable(context.Background(), group.Id, channel.Id, model.GroupSyncableTypeChannel, &model.GroupSyncablePatch{AutoAdd: &autoAdd, SchemeAdmin: &schemeAdmin})
		require.NoError(t, err)
		CheckCreatedStatus(t, r)

		patch := &model.ChannelPatch{}
		patch.GroupConstrained = new(true)
		_, r, err = th.SystemAdminClient.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, r)

		// Wait for the user to be removed from the channel by polling until they're gone
		// or until we hit the timeout
		timeout := time.After(3 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		userRemoved := false
		for !userRemoved {
			select {
			case <-timeout:
				require.Fail(t, "Timed out waiting for user to be removed from channel")
				return
			case <-ticker.C:
				// Check if the user is still a member
				_, r, err = th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
				if err != nil && r.StatusCode == http.StatusNotFound {
					// User has been removed, we can continue the test
					userRemoved = true
				}
			}
		}

		// Verify the user is no longer a member of the channel
		_, r, err = th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, r)
	})

	t.Run("Test GroupConstrained flag changed from true to false and non group members are not removed", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		// Create a test group
		group := th.CreateGroup(t)

		// Create a channel and set it as group-constrained
		channel := th.CreatePrivateChannel(t)

		// Create a group user
		groupUser := th.CreateUser(t)
		th.LinkUserToTeam(t, groupUser, th.BasicTeam)

		// Create a group member
		_, appErr := th.App.UpsertGroupMember(group.Id, groupUser.Id)
		require.Nil(t, appErr)

		// Associate the group with the channel
		autoAdd := true
		schemeAdmin := true
		_, r, err := th.SystemAdminClient.LinkGroupSyncable(context.Background(), group.Id, channel.Id, model.GroupSyncableTypeChannel, &model.GroupSyncablePatch{AutoAdd: &autoAdd, SchemeAdmin: &schemeAdmin})
		require.NoError(t, err)
		CheckCreatedStatus(t, r)

		// Wait for the user to be added to the channel by polling until you see them
		// or until we hit the timeout
		timeout := time.After(3 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var cm *model.ChannelMember
		userFound := false
		for !userFound {
			select {
			case <-timeout:
				require.Fail(t, "Timed out waiting for user to be added to the channel")
				return
			case <-ticker.C:
				// Check if the user is now a member
				cm, _, err = th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, groupUser.Id, "")
				if err == nil && cm.UserId == groupUser.Id {
					// User has been added, we can continue the test
					userFound = true
				}
			}
		}

		patch := &model.ChannelPatch{}
		patch.GroupConstrained = new(true)
		_, r, err = th.SystemAdminClient.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, r)

		// Change the GroupConstrained flag to false
		patch.GroupConstrained = new(false)
		_, r, err = th.SystemAdminClient.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, r)

		// Unlink the group
		r, err = th.SystemAdminClient.UnlinkGroupSyncable(context.Background(), group.Id, channel.Id, model.GroupSyncableTypeChannel)
		require.NoError(t, err)
		CheckOKStatus(t, r)

		// Wait for a reasonable amount of time to ensure the user is not removed because the channel is no longer group constrained
		timeout = time.After(2 * time.Second)
		ticker = time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		userStillPresent := true
		for userStillPresent {
			select {
			case <-timeout:
				// If we reach the timeout, the user is still present, which is what we want
				// Verify the user is still a member of the channel
				cm, r, err = th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, groupUser.Id, "")
				require.NoError(t, err)
				CheckOKStatus(t, r)
				require.Equal(t, groupUser.Id, cm.UserId)
				return
			case <-ticker.C:
				// Check if the user is still a member
				_, r, err = th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, groupUser.Id, "")
				if err != nil && r.StatusCode == http.StatusNotFound {
					// User has been removed, which is not what we want
					require.Fail(t, "User was incorrectly removed from the channel")
					userStillPresent = false
				}
			}
		}
	})
}

func TestPatchChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	team := th.BasicTeam

	t.Run("should be unable to apply a null patch", func(t *testing.T) {
		var nullPatch *model.ChannelPatch

		_, nullResp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, nullPatch)
		require.Error(t, err)
		CheckBadRequestStatus(t, nullResp)
	})

	t.Run("should be able to patch values", func(t *testing.T) {
		patch := &model.ChannelPatch{
			Name:        new(string),
			DisplayName: new(string),
			Header:      new(string),
			Purpose:     new(string),
		}
		*patch.Name = model.NewId()
		*patch.DisplayName = model.NewId()
		*patch.Header = model.NewId()
		*patch.Purpose = model.NewId()

		channel, _, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)

		require.Equal(t, *patch.Name, channel.Name, "do not match")
		require.Equal(t, *patch.DisplayName, channel.DisplayName, "do not match")
		require.Equal(t, *patch.Header, channel.Header, "do not match")
		require.Equal(t, *patch.Purpose, channel.Purpose, "do not match")
	})

	t.Run("should be able to patch with no name", func(t *testing.T) {
		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		var err error
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		patch := &model.ChannelPatch{
			Header:  new(string),
			Purpose: new(string),
		}

		oldName := channel.Name
		patchedChannel, _, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)

		require.Equal(t, oldName, patchedChannel.Name, "should not have updated")
	})

	t.Run("Test updating default channel's name and returns error", func(t *testing.T) {
		// Test updating default channel's name and returns error
		defaultChannel, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
		require.Nil(t, appErr)
		defaultChannelPatch := &model.ChannelPatch{
			Name: new(string),
		}
		*defaultChannelPatch.Name = "testing"
		_, resp, err := client.PatchChannel(context.Background(), defaultChannel.Id, defaultChannelPatch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Test updating the header of someone else's GM channel", func(t *testing.T) {
		// Test updating the header of someone else's GM channel.
		user := th.CreateUser(t)
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)
		user3 := th.CreateUser(t)

		groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id})
		require.NoError(t, err)

		_, err = client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user3.Email, user3.Password)
		require.NoError(t, err)

		channelPatch := &model.ChannelPatch{}
		channelPatch.Header = new(string)
		*channelPatch.Header = "lolololol"

		_, resp, err := client.PatchChannel(context.Background(), groupChannel.Id, channelPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, err = client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)

		directChannel, _, err := client.CreateDirectChannel(context.Background(), user.Id, user1.Id)
		require.NoError(t, err)

		_, err = client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user3.Email, user3.Password)
		require.NoError(t, err)
		_, resp, err = client.PatchChannel(context.Background(), directChannel.Id, channelPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Should block changes to name, display name or purpose for group messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)
		user3 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		groupChannelPatch := &model.ChannelPatch{
			Name: new(string),
		}
		*groupChannelPatch.Name = "testing"
		_, resp, err := client.PatchChannel(context.Background(), groupChannel.Id, groupChannelPatch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		groupChannelPatch2 := &model.ChannelPatch{
			DisplayName: new(string),
		}
		*groupChannelPatch2.DisplayName = "test display name"
		_, resp, err = client.PatchChannel(context.Background(), groupChannel.Id, groupChannelPatch2)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		groupChannelPatch3 := &model.ChannelPatch{
			Purpose: new(string),
		}
		*groupChannelPatch3.Purpose = "test purpose"
		_, resp, err = client.PatchChannel(context.Background(), groupChannel.Id, groupChannelPatch3)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should block changes to name, display name or purpose for direct messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		directChannel, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
		require.NoError(t, err)

		directChannelPatch := &model.ChannelPatch{
			Name: new(string),
		}
		*directChannelPatch.Name = "test"
		_, resp, err := client.PatchChannel(context.Background(), directChannel.Id, directChannelPatch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		directChannelPatch2 := &model.ChannelPatch{
			DisplayName: new(string),
		}
		*directChannelPatch2.DisplayName = "test display name"
		_, resp, err = client.PatchChannel(context.Background(), directChannel.Id, directChannelPatch2)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		directChannelPatch3 := &model.ChannelPatch{
			Purpose: new(string),
		}
		*directChannelPatch3.Purpose = "test purpose"
		_, resp, err = client.PatchChannel(context.Background(), directChannel.Id, directChannelPatch3)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should block setting group_constrained on group and direct messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)
		user3 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		patch := &model.ChannelPatch{GroupConstrained: model.NewPointer(true)}
		_, resp, err := client.PatchChannel(context.Background(), groupChannel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		stats, _, err := client.GetChannelStats(context.Background(), groupChannel.Id, "", false)
		require.NoError(t, err)
		require.Equal(t, int64(3), stats.MemberCount)

		directChannel, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
		require.NoError(t, err)

		_, resp, err = client.PatchChannel(context.Background(), directChannel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should block changes to default_category_name for group messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)
		user3 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)

		categoryName := "Operations"
		patch := &model.ChannelPatch{DefaultCategoryName: &categoryName}
		_, resp, err := client.PatchChannel(context.Background(), groupChannel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should block changes to default_category_name for direct messages", func(t *testing.T) {
		user1 := th.CreateUser(t)
		user2 := th.CreateUser(t)

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		_, _, err = client.Login(context.Background(), user1.Email, user1.Password)
		require.NoError(t, err)

		directChannel, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
		require.NoError(t, err)

		categoryName := "Operations"
		patch := &model.ChannelPatch{DefaultCategoryName: &categoryName}
		_, resp, err := client.PatchChannel(context.Background(), directChannel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Should be able to patch default_category_name on an open channel", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		categoryName := "Operations"
		patch := &model.ChannelPatch{DefaultCategoryName: &categoryName}
		patched, _, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		require.Equal(t, categoryName, patched.DefaultCategoryName)

		fetched, _, err := client.GetChannel(context.Background(), channel.Id)
		require.NoError(t, err)
		require.Equal(t, categoryName, fetched.DefaultCategoryName)

		emptyName := ""
		clearPatch := &model.ChannelPatch{DefaultCategoryName: &emptyName}
		cleared, _, err := client.PatchChannel(context.Background(), channel.Id, clearPatch)
		require.NoError(t, err)
		require.Equal(t, "", cleared.DefaultCategoryName)
	})

	t.Run("Should not be able to configure channel banner without a license", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)

		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		require.Nil(t, patchedChannel)
	})

	t.Run("Should not be able to configure channel banner with a professional license", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuProfessional))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		require.Nil(t, patchedChannel)
	})

	t.Run("Should be able to configure channel banner on a channel", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, patchedChannel.BannerInfo)
		require.True(t, *patchedChannel.BannerInfo.Enabled)
		require.Equal(t, "banner text", *patchedChannel.BannerInfo.Text)
		require.Equal(t, "#dddddd", *patchedChannel.BannerInfo.BackgroundColor)
	})

	t.Run("Should not be able to configure channel banner on a channel as a non-admin channel member", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		_, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Should be able to configure channel banner as a team admin", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginTeamAdmin(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), th.BasicChannel2.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, patchedChannel.BannerInfo)
		require.True(t, *patchedChannel.BannerInfo.Enabled)
		require.Equal(t, "banner text", *patchedChannel.BannerInfo.Text)
		require.Equal(t, "#dddddd", *patchedChannel.BannerInfo.BackgroundColor)
	})

	t.Run("Cannot enable channel banner without configuring it", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		channel := &model.Channel{
			DisplayName: GenerateTestChannelName(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err = client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled: new(true),
			},
		}

		_, resp, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		// now we will configure it first, then enable it
		patch = &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         nil,
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, patchedChannel.BannerInfo)
		require.Nil(t, patchedChannel.BannerInfo.Enabled)
		require.Equal(t, "banner text", *patchedChannel.BannerInfo.Text)
		require.Equal(t, "#dddddd", *patchedChannel.BannerInfo.BackgroundColor)

		patch = &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled: new(true),
			},
		}

		patchedChannel, resp, err = client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, patchedChannel.BannerInfo)
		require.True(t, *patchedChannel.BannerInfo.Enabled)
		require.Equal(t, "banner text", *patchedChannel.BannerInfo.Text)
		require.Equal(t, "#dddddd", *patchedChannel.BannerInfo.BackgroundColor)
	})

	t.Run("Cannot configure channel banner on a DM channel", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		dmChannel, resp, err := client.CreateDirectChannel(context.Background(), th.BasicUser.Id, th.BasicUser2.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), dmChannel.Id, patch)
		require.Error(t, err)
		require.Equal(t, "Channel banner can only be configured on Public and Private channels.", err.Error())
		CheckBadRequestStatus(t, resp)
		require.Nil(t, patchedChannel)
	})

	t.Run("Cannot configure channel banner on a GM channel", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		user3 := th.CreateUser(t)
		gmChannel, resp, err := client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, user3.Id})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			BannerInfo: &model.ChannelBannerInfo{
				Enabled:         new(true),
				Text:            new("banner text"),
				BackgroundColor: new("#dddddd"),
			},
		}

		patchedChannel, resp, err := client.PatchChannel(context.Background(), gmChannel.Id, patch)
		require.Error(t, err)
		require.Equal(t, "Channel banner can only be configured on Public and Private channels.", err.Error())
		CheckBadRequestStatus(t, resp)
		require.Nil(t, patchedChannel)
	})

	t.Run("Patch channel with no changes returns 400", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		patch := &model.ChannelPatch{}
		_, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Patch channel with autotranslation when feature is available properly updates the channel for admins", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := th.SystemAdminClient.Logout(context.Background())
		require.NoError(t, err)
		th.LoginSystemAdmin(t)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err := th.SystemAdminClient.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		patchedChannel, appErr := th.App.GetChannel(th.Context, th.BasicChannel.Id)
		require.Nil(t, appErr)
		require.True(t, patchedChannel.AutoTranslation)

		patch = &model.ChannelPatch{
			AutoTranslation: new(false),
		}

		_, resp, err = th.SystemAdminClient.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		patchedChannel, appErr = th.App.GetChannel(th.Context, th.BasicChannel.Id)
		require.Nil(t, appErr)
		require.False(t, patchedChannel.AutoTranslation)
	})

	t.Run("Patch channel with autotranslation when feature is available properly updates the channel for users only with the proper permissions", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		privateChannel := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypePrivate)
		th.AddUserToChannel(t, th.BasicUser, privateChannel)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = client.PatchChannel(context.Background(), privateChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		th.AddPermissionToRole(t, model.PermissionManagePrivateChannelAutoTranslation.Id, model.SystemUserRoleId)
		defer th.RemovePermissionFromRole(t, model.PermissionManagePrivateChannelAutoTranslation.Id, model.SystemUserRoleId)

		_, resp, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, _, err = client.PatchChannel(context.Background(), privateChannel.Id, patch)
		require.NoError(t, err)
		patchedChannel, appErr := th.App.GetChannel(th.Context, privateChannel.Id)
		require.Nil(t, appErr)
		require.True(t, patchedChannel.AutoTranslation)

		th.AddPermissionToRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)
		defer th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)

		_, _, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		patchedChannel, appErr = th.App.GetChannel(th.Context, privateChannel.Id)
		require.Nil(t, appErr)
		require.True(t, patchedChannel.AutoTranslation)
	})

	t.Run("Patch channel with AutoTranslation when feature not available returns 403", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(false)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		var appErr *model.AppError
		require.True(t, errors.As(err, &appErr))
		require.Contains(t, []string{"api.channel.patch_update_channel.feature_not_available.app_error", "api.channel.patch_update_channel.auto_translation_restricted.app_error"}, appErr.Id)
	})

	t.Run("Patch channel with autotranslation on DM is only available for members", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		dmChannel, resp, err := client.CreateDirectChannel(context.Background(), th.BasicUser.Id, th.BasicUser2.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		nonMemberDmChannel, resp, err := th.SystemAdminClient.CreateDirectChannel(context.Background(), th.BasicUser2.Id, th.SystemAdminUser.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err = client.PatchChannel(context.Background(), dmChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		patchedChannel, appErr := th.App.GetChannel(th.Context, dmChannel.Id)
		require.Nil(t, appErr)
		require.True(t, patchedChannel.AutoTranslation)

		_, resp, err = client.PatchChannel(context.Background(), nonMemberDmChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Patch channel with autotranslation on GM is only available for members", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		user3 := th.CreateUser(t)

		gmChannel, resp, err := client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, user3.Id})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		nonMemberGmChannel, resp, err := th.SystemAdminClient.CreateGroupChannel(context.Background(), []string{th.BasicUser2.Id, th.SystemAdminUser.Id, user3.Id})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err = client.PatchChannel(context.Background(), gmChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		patchedChannel, appErr := th.App.GetChannel(th.Context, gmChannel.Id)
		require.Nil(t, appErr)
		require.True(t, patchedChannel.AutoTranslation)

		_, resp, err = client.PatchChannel(context.Background(), nonMemberGmChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Patch DM with AutoTranslation when RestrictDMAndGM is true returns 403", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.AutoTranslationSettings.RestrictDMAndGM = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.AutoTranslationSettings.RestrictDMAndGM = false
		})

		dmChannel, resp, err := client.CreateDirectChannel(context.Background(), th.BasicUser.Id, th.BasicUser2.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err = client.PatchChannel(context.Background(), dmChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		// May be feature_not_available when AutoTranslation is nil, or auto_translation_restricted when RestrictDMAndGM applies
		var appErr *model.AppError
		require.True(t, errors.As(err, &appErr))
		require.Contains(t, []string{"api.channel.patch_update_channel.feature_not_available.app_error", "api.channel.patch_update_channel.auto_translation_restricted.app_error"}, appErr.Id)
	})

	t.Run("Patch GM with AutoTranslation when RestrictDMAndGM is true returns 403", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.AutoTranslationSettings.RestrictDMAndGM = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.AutoTranslationSettings.RestrictDMAndGM = false
		})

		user3 := th.CreateUser(t)
		gmChannel, resp, err := client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, user3.Id})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		patch := &model.ChannelPatch{
			AutoTranslation: new(true),
		}

		_, resp, err = client.PatchChannel(context.Background(), gmChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		// May be feature_not_available when AutoTranslation is nil, or auto_translation_restricted when RestrictDMAndGM applies
		var appErr *model.AppError
		require.True(t, errors.As(err, &appErr))
		require.Contains(t, []string{"api.channel.patch_update_channel.feature_not_available.app_error", "api.channel.patch_update_channel.auto_translation_restricted.app_error"}, appErr.Id)
	})

	t.Run("Mixed patch only gets through if all permissions are met", func(t *testing.T) {
		mockAutoTranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutoTranslation.On("IsFeatureAvailable").Return(true)
		mockAutoTranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
		mockAutoTranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutoTranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		th.LoginBasic(t)

		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		// Mixed patch (channel property + AutoTranslation) fails when user lacks AutoTranslation permission
		newHeader := "mixed patch header"
		mixedPatch := &model.ChannelPatch{
			Header:          &newHeader,
			AutoTranslation: new(true),
			BannerInfo: &model.ChannelBannerInfo{
				Enabled: new(false),
				Text:    new("mixed patch banner"),
			},
		}

		// Permissions missing: AutoTranslation, BannerInfo
		_, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, mixedPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Permissions missing: Channel properties
		th.AddPermissionToRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)
		defer th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)
		th.AddPermissionToRole(t, model.PermissionManagePublicChannelBanner.Id, model.SystemUserRoleId)
		defer th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelBanner.Id, model.SystemUserRoleId)
		th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelProperties.Id, model.ChannelUserRoleId)
		defer th.AddPermissionToRole(t, model.PermissionManagePublicChannelProperties.Id, model.ChannelUserRoleId)

		_, resp, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, mixedPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Permissions missing: AutoTranslation
		th.AddPermissionToRole(t, model.PermissionManagePublicChannelProperties.Id, model.ChannelUserRoleId)
		th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)

		_, resp, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, mixedPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Permission missing: BannerInfo
		th.AddPermissionToRole(t, model.PermissionManagePublicChannelAutoTranslation.Id, model.SystemUserRoleId)
		th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelBanner.Id, model.SystemUserRoleId)
		_, resp, err = client.PatchChannel(context.Background(), th.BasicChannel.Id, mixedPatch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// No missing permissions
		th.AddPermissionToRole(t, model.PermissionManagePublicChannelBanner.Id, model.SystemUserRoleId)

		patchedChannel, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, mixedPatch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newHeader, patchedChannel.Header)
		require.True(t, patchedChannel.AutoTranslation)
	})
}

func TestCanEditChannelBanner(t *testing.T) {
	th := Setup(t).InitBasic(t)

	t.Run("when license is nil", func(t *testing.T) {
		channel := &model.Channel{
			Type: model.ChannelTypeOpen,
		}

		th.App.Srv().SetLicense(nil)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: "channel_id",
			},
		}

		canEditChannelBanner(webContext, channel)

		require.NotNil(t, webContext.Err)
		assert.Equal(t, "api.context.permissions.app_error", webContext.Err.Id)
		assert.Equal(t, http.StatusForbidden, webContext.Err.StatusCode)
	})

	t.Run("when license is not E20 or Enterprise", func(t *testing.T) {
		license := model.NewTestLicenseSKU(model.LicenseShortSkuProfessional)
		th.App.Srv().SetLicense(license)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: "channel_id",
			},
		}

		channel := &model.Channel{
			Type: model.ChannelTypeOpen,
		}

		canEditChannelBanner(webContext, channel)

		require.NotNil(t, webContext.Err)
		assert.Equal(t, "api.context.permissions.app_error", webContext.Err.Id)
		assert.Equal(t, http.StatusForbidden, webContext.Err.StatusCode)
	})

	t.Run("when channel type is direct message", func(t *testing.T) {
		license := model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced)
		th.App.Srv().SetLicense(license)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: "channel_id",
			},
		}

		channel := &model.Channel{
			Type: model.ChannelTypeDirect,
		}

		canEditChannelBanner(webContext, channel)

		require.NotNil(t, webContext.Err)
		assert.Equal(t, "api.channel.update_channel.banner_info.channel_type.not_allowed", webContext.Err.Id)
		assert.Equal(t, http.StatusBadRequest, webContext.Err.StatusCode)
	})

	t.Run("when channel type is group message", func(t *testing.T) {
		license := model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced)
		th.App.Srv().SetLicense(license)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: "channel_id",
			},
		}

		channel := &model.Channel{
			Type: model.ChannelTypeGroup,
		}

		canEditChannelBanner(webContext, channel)
		require.NotNil(t, webContext.Err)
		assert.Equal(t, "api.channel.update_channel.banner_info.channel_type.not_allowed", webContext.Err.Id)
		assert.Equal(t, http.StatusBadRequest, webContext.Err.StatusCode)
	})

	t.Run("when channel type is open and license is valid", func(t *testing.T) {
		license := model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced)
		th.App.Srv().SetLicense(license)

		channel := th.CreatePublicChannel(t)
		th.MakeUserChannelAdmin(t, th.BasicUser, channel)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: channel.Id,
			},
		}

		webContext.AppContext = webContext.AppContext.WithSession(&model.Session{
			UserId: th.BasicUser.Id,
		})

		canEditChannelBanner(webContext, channel)
		assert.Nil(t, webContext.Err)
	})

	t.Run("when channel type is private and license is valid", func(t *testing.T) {
		license := model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced)
		th.App.Srv().SetLicense(license)

		channel := th.CreatePrivateChannel(t)
		th.MakeUserChannelAdmin(t, th.BasicUser, channel)

		webContext := &Context{
			App:        th.App,
			AppContext: th.Context,
			Params: &web.Params{
				ChannelId: channel.Id,
			},
		}

		webContext.AppContext = webContext.AppContext.WithSession(&model.Session{
			UserId: th.BasicUser.Id,
		})

		canEditChannelBanner(webContext, channel)
		assert.Nil(t, webContext.Err)
	})
}

func TestChannelUnicodeNames(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	team := th.BasicTeam

	t.Run("create channel unicode", func(t *testing.T) {
		channel := &model.Channel{
			Name:        "\u206cenglish\u206dchannel",
			DisplayName: "The \u206cEnglish\u206d Channel",
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}

		rchannel, resp, err := client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, "englishchannel", rchannel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "The English Channel", rchannel.DisplayName, "bad unicode should be filtered from display name")
	})

	t.Run("update channel unicode", func(t *testing.T) {
		channel := &model.Channel{
			DisplayName: "Test API Name",
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      team.Id,
		}
		channel, _, err := client.CreateChannel(context.Background(), channel)
		require.NoError(t, err)

		channel.Name = "\u206ahistorychannel"
		channel.DisplayName = "UFO's and \ufff9stuff\ufffb."

		newChannel, _, err := client.UpdateChannel(context.Background(), channel)
		require.NoError(t, err)

		require.Equal(t, "historychannel", newChannel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "UFO's and stuff.", newChannel.DisplayName, "bad unicode should be filtered from display name")
	})

	t.Run("patch channel unicode", func(t *testing.T) {
		patch := &model.ChannelPatch{
			Name:        new(string),
			DisplayName: new(string),
			Header:      new(string),
			Purpose:     new(string),
		}
		*patch.Name = "\u206ecommunitychannel\u206f"
		*patch.DisplayName = "Natalie Tran's \ufffcAwesome Channel"

		channel, _, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)

		require.Equal(t, "communitychannel", channel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "Natalie Tran's Awesome Channel", channel.DisplayName, "bad unicode should be filtered from display name")
	})
}

func TestCreateDirectChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser(t)

	dm, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
	require.NoError(t, err)

	channelName := ""
	if user2.Id > user1.Id {
		channelName = user1.Id + "__" + user2.Id
	} else {
		channelName = user2.Id + "__" + user1.Id
	}

	require.Equal(t, channelName, dm.Name, "dm name didn't match")

	_, resp, err := client.CreateDirectChannel(context.Background(), "junk", user2.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.CreateDirectChannel(context.Background(), user1.Id, model.NewId())
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.CreateDirectChannel(context.Background(), model.NewId(), user1.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.CreateDirectChannel(context.Background(), model.NewId(), user2.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	r, err := client.DoAPIPost(context.Background(), "/channels/direct", "garbage")
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	_, _, err = th.SystemAdminClient.CreateDirectChannel(context.Background(), user3.Id, user2.Id)
	require.NoError(t, err)

	// Normal client should not be allowed to create a direct channel if users are
	// restricted to messaging members of their own team
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.RestrictDirectMessage = model.DirectMessageTeam
	})
	user4 := th.CreateUser(t)
	_, resp, err = th.Client.CreateDirectChannel(context.Background(), user1.Id, user4.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	th.LinkUserToTeam(t, user4, th.BasicTeam)
	_, _, err = th.Client.CreateDirectChannel(context.Background(), user1.Id, user4.Id)
	require.NoError(t, err)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.CreateDirectChannel(context.Background(), model.NewId(), user2.Id)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)
}

func TestCreateDirectChannelAsGuest(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user1 := th.BasicUser

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guestPassword := model.NewTestPassword()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      guestPassword,
		EmailVerified: true,
	}
	guest, appErr := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, appErr)

	_, _, err := client.Login(context.Background(), guest.Username, guestPassword)
	require.NoError(t, err)

	t.Run("Try to created DM with not visible user", func(t *testing.T) {
		var resp *model.Response
		_, resp, err = client.CreateDirectChannel(context.Background(), guest.Id, user1.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = client.CreateDirectChannel(context.Background(), user1.Id, guest.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating DM with visible user", func(t *testing.T) {
		th.LinkUserToTeam(t, guest, th.BasicTeam)
		th.AddUserToChannel(t, guest, th.BasicChannel)

		_, _, err = client.CreateDirectChannel(context.Background(), guest.Id, user1.Id)
		require.NoError(t, err)
	})
}

func TestDeleteDirectChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2

	rgc, resp, err := client.CreateDirectChannel(context.Background(), user.Id, user2.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	require.NotNil(t, rgc, "should have created a direct channel")

	_, err = client.DeleteChannel(context.Background(), rgc.Id)
	CheckErrorID(t, err, "api.channel.delete_channel.type.invalid")
}

func TestCreateGroupChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser(t)

	userIds := []string{user.Id, user2.Id, user3.Id}

	rgc, resp, err := client.CreateGroupChannel(context.Background(), userIds)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	require.NotNil(t, rgc, "should have created a group channel")
	require.Equal(t, model.ChannelTypeGroup, rgc.Type, "should have created a channel of group type")

	m, appErr := th.App.GetChannelMembersPage(th.Context, rgc.Id, 0, 10)
	require.Nil(t, appErr)
	require.Len(t, m, 3, "should have 3 channel members")

	// saving duplicate group channel
	rgc2, _, err := client.CreateGroupChannel(context.Background(), []string{user3.Id, user2.Id})
	require.NoError(t, err)
	require.Equal(t, rgc.Id, rgc2.Id, "should have returned existing channel")

	m2, appErr := th.App.GetChannelMembersPage(th.Context, rgc2.Id, 0, 10)
	require.Nil(t, appErr)
	require.ElementsMatch(t, m, m2)

	_, resp, err = client.CreateGroupChannel(context.Background(), []string{user2.Id})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	user4 := th.CreateUser(t)
	user5 := th.CreateUser(t)
	user6 := th.CreateUser(t)
	user7 := th.CreateUser(t)
	user8 := th.CreateUser(t)
	user9 := th.CreateUser(t)

	rgc, resp, err = client.CreateGroupChannel(context.Background(), []string{user.Id, user2.Id, user3.Id, user4.Id, user5.Id, user6.Id, user7.Id, user8.Id, user9.Id})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rgc)

	_, resp, err = client.CreateGroupChannel(context.Background(), []string{user.Id, user2.Id, user3.Id, GenerateTestID()})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.CreateGroupChannel(context.Background(), []string{user.Id, user2.Id, user3.Id, "junk"})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	_, resp, err = client.CreateGroupChannel(context.Background(), userIds)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, _, err = th.SystemAdminClient.CreateGroupChannel(context.Background(), userIds)
	require.NoError(t, err)
}

func TestCreateGroupChannelAsGuest(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser(t)
	user4 := th.CreateUser(t)
	user5 := th.CreateUser(t)
	th.LinkUserToTeam(t, user2, th.BasicTeam)
	th.AddUserToChannel(t, user2, th.BasicChannel)
	th.LinkUserToTeam(t, user3, th.BasicTeam)
	th.AddUserToChannel(t, user3, th.BasicChannel)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guestPassword := model.NewTestPassword()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      guestPassword,
		EmailVerified: true,
	}
	guest, appErr := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, appErr)

	_, _, err := client.Login(context.Background(), guest.Username, guestPassword)
	require.NoError(t, err)

	var resp *model.Response

	t.Run("Try to created GM with not visible users", func(t *testing.T) {
		_, resp, err = client.CreateGroupChannel(context.Background(), []string{guest.Id, user1.Id, user2.Id, user3.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, guest.Id, user3.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Try to created GM with visible and not visible users", func(t *testing.T) {
		th.LinkUserToTeam(t, guest, th.BasicTeam)
		th.AddUserToChannel(t, guest, th.BasicChannel)

		_, resp, err = client.CreateGroupChannel(context.Background(), []string{guest.Id, user1.Id, user3.Id, user4.Id, user5.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, guest.Id, user4.Id, user5.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating GM with visible users", func(t *testing.T) {
		_, _, err = client.CreateGroupChannel(context.Background(), []string{guest.Id, user1.Id, user2.Id, user3.Id})
		require.NoError(t, err)
	})
}

func TestDeleteGroupChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser(t)

	userIds := []string{user.Id, user2.Id, user3.Id}

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rgc, resp, err := th.Client.CreateGroupChannel(context.Background(), userIds)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, rgc, "should have created a group channel")
		_, err = client.DeleteChannel(context.Background(), rgc.Id)
		CheckErrorID(t, err, "api.channel.delete_channel.type.invalid")
	})
}

func TestGetChannel(t *testing.T) {
	mainHelper.Parallel(t)

	th := Setup(t).InitBasic(t)
	client := th.Client

	channel, _, err := client.GetChannel(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	require.Equal(t, th.BasicChannel.Id, channel.Id, "ids did not match")

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, _, err = client.GetChannel(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)

	channel, _, err = client.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
	require.NoError(t, err)
	require.Equal(t, th.BasicPrivateChannel.Id, channel.Id, "ids did not match")

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicPrivateChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, resp, err := client.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.GetChannel(context.Background(), model.NewId())
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannel(context.Background(), th.BasicChannel.Id)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = client.GetChannel(context.Background(), th.BasicChannel.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, err = client.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)

		_, _, err = client.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
		require.NoError(t, err)

		_, resp, err = client.GetChannel(context.Background(), th.BasicUser.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("Content reviewer should be able to get channel without membership with flagged post", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		appErr := setBasicCommonReviewerConfig(th)
		require.Nil(t, appErr)

		contentReviewClient := th.CreateClient()
		_, _, err := contentReviewClient.Login(context.Background(), th.BasicUser.Email, th.BasicUser.Password)
		require.NoError(t, err)

		privateChannel := th.CreateChannelWithClient(t, contentReviewClient, model.ChannelTypePrivate)
		post := th.CreatePostWithClient(t, contentReviewClient, privateChannel)

		response, err := contentReviewClient.FlagPostForContentReview(context.Background(), post.Id, &model.FlagContentRequest{
			Reason:  "Classification mismatch",
			Comment: "This is sensitive content",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		th.RemoveUserFromChannel(t, th.BasicUser, privateChannel)

		// We will fetch the channel providing the required params to indicate that we are fetching it for content review
		fetchedChannel, _, err := contentReviewClient.GetChannelAsContentReviewer(context.Background(), privateChannel.Id, "", post.Id)
		require.NoError(t, err)
		require.Equal(t, privateChannel.Id, fetchedChannel.Id)

		// This also doesn't work if user is not a content reviewer
		contentFlaggingSettings, _, err := th.SystemAdminClient.GetContentFlaggingSettings(context.Background())
		require.NoError(t, err)
		require.NotNil(t, contentFlaggingSettings)

		// Making system admin as a reviewer because there needs to be some reviewers
		contentFlaggingSettings.ReviewerSettings.CommonReviewerIds = []string{th.SystemAdminUser.Id}
		resp, err = th.SystemAdminClient.SaveContentFlaggingSettings(context.Background(), contentFlaggingSettings)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		_, resp, err = contentReviewClient.GetChannelAsContentReviewer(context.Background(), privateChannel.Id, "", post.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Content reviewer should not be able to get a DM or GM channel", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		appErr := setBasicCommonReviewerConfig(th)
		require.Nil(t, appErr)

		contentReviewClient := th.CreateClient()
		_, _, err := contentReviewClient.Login(context.Background(), th.BasicUser.Email, th.BasicUser.Password)
		require.NoError(t, err)

		dmPost := createDmPost(t, th, contentReviewClient)
		_, resp, err := contentReviewClient.GetChannelAsContentReviewer(context.Background(), dmPost.ChannelId, "", dmPost.Id)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.data_spillage.error.invalid_channel_type")

		gmPost := createGmPost(t, th, contentReviewClient)
		_, resp, err = contentReviewClient.GetChannelAsContentReviewer(context.Background(), gmPost.ChannelId, "", gmPost.Id)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.data_spillage.error.invalid_channel_type")
	})
}

func TestGetDeletedChannelsForTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	client := th.Client
	team := th.BasicTeam

	th.LoginTeamAdmin(t)

	channels, _, err := client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.NoError(t, err)
	numInitialChannelsForTeam := len(channels)

	// create and delete public channel
	publicChannel1 := th.CreatePublicChannel(t)
	_, err = client.DeleteChannel(context.Background(), publicChannel1.Id)
	require.NoError(t, err)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
		require.NoError(t, err)
		require.Len(t, channels, numInitialChannelsForTeam+1, "should be 1 deleted channel")
	})

	publicChannel2 := th.CreatePublicChannel(t)
	_, err = client.DeleteChannel(context.Background(), publicChannel2.Id)
	require.NoError(t, err)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
		require.NoError(t, err)
		require.Len(t, channels, numInitialChannelsForTeam+2, "should be 2 deleted channels")
	})

	th.LoginBasic(t)

	privateChannel1 := th.CreatePrivateChannel(t)
	_, err = client.DeleteChannel(context.Background(), privateChannel1.Id)
	require.NoError(t, err)

	channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.NoError(t, err)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	// Login as different user and create private channel
	th.LoginBasic2(t)
	privateChannel2 := th.CreatePrivateChannel(t)
	_, err = client.DeleteChannel(context.Background(), privateChannel2.Id)
	require.NoError(t, err)

	// Log back in as first user
	th.LoginBasic(t)

	channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.NoError(t, err)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
		require.NoError(t, err)
		// Local admin should see private archived channels
		require.Len(t, channels, numInitialChannelsForTeam+4)
	})

	channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 1, "")
	require.NoError(t, err)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, _, err = client.GetDeletedChannelsForTeam(context.Background(), team.Id, 1, 1, "")
	require.NoError(t, err)
	require.Len(t, channels, 1, "should be one channel per page")

	// test non team member
	_, err = th.SystemAdminClient.RemoveTeamMember(context.Background(), team.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, resp, err := client.GetDeletedChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestGetPrivateChannelsForTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	// normal user
	_, resp, err := th.Client.GetPrivateChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		channels, _, err := c.GetPrivateChannelsForTeam(context.Background(), team.Id, 0, 100, "")
		require.NoError(t, err)
		// th.BasicPrivateChannel and th.BasicPrivateChannel2
		require.Len(t, channels, 2, "wrong number of private channels")
		for _, c := range channels {
			// check all channels included are private
			require.Equal(t, model.ChannelTypePrivate, c.Type, "should include private channels only")
		}

		channels, _, err = c.GetPrivateChannelsForTeam(context.Background(), team.Id, 0, 1, "")
		require.NoError(t, err)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, _, err = c.GetPrivateChannelsForTeam(context.Background(), team.Id, 1, 1, "")
		require.NoError(t, err)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, _, err = c.GetPrivateChannelsForTeam(context.Background(), team.Id, 10000, 100, "")
		require.NoError(t, err)
		require.Empty(t, channels, "should be no channel")

		_, resp, err = c.GetPrivateChannelsForTeam(context.Background(), "junk", 0, 100, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestGetPublicChannelsForTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	team := th.BasicTeam
	publicChannel1 := th.BasicChannel
	publicChannel2 := th.BasicChannel2

	channels, _, err := client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.NoError(t, err)
	require.Len(t, channels, 4, "wrong path")

	var foundPublicChannel1, foundPublicChannel2 bool
	for _, c := range channels {
		// check all channels included are open
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should include open channel only")

		// only check the created 2 public channels
		switch c.DisplayName {
		case publicChannel1.DisplayName:
			foundPublicChannel1 = true
		case publicChannel2.DisplayName:
			foundPublicChannel2 = true
		}
	}

	require.True(t, foundPublicChannel1, "failed to find publicChannel1")
	require.True(t, foundPublicChannel2, "failed to find publicChannel2")

	privateChannel := th.CreatePrivateChannel(t)
	channels, _, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.NoError(t, err)
	require.Len(t, channels, 4, "incorrect length of team public channels")

	for _, c := range channels {
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should not include private channel")
		require.NotEqual(t, privateChannel.DisplayName, c.DisplayName, "should not match private channel display name")
	}

	channels, _, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 1, "")
	require.NoError(t, err)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, _, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 1, 1, "")
	require.NoError(t, err)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, _, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 10000, 100, "")
	require.NoError(t, err)
	require.Empty(t, channels, "should be no channel")

	_, resp, err := client.GetPublicChannelsForTeam(context.Background(), "junk", 0, 100, "")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetPublicChannelsForTeam(context.Background(), model.NewId(), 0, 100, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 100, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, err = client.GetPublicChannelsForTeam(context.Background(), team.Id, 0, 100, "")
		require.NoError(t, err)
	})
}

func TestGetRecommendedChannelsForTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("without enterprise license ABAC is disabled so the endpoint returns an empty list", func(t *testing.T) {
		// Be explicit about the license precondition so this subtest is
		// deterministic even if a parallel test elsewhere installed one.
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)

		resp, err := th.Client.DoAPIGet(context.Background(), "/teams/"+th.BasicTeam.Id+"/channels/recommended", "")
		require.NoError(t, err)
		require.NotNil(t, resp)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var channels []*model.Channel
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&channels))
		require.Empty(t, channels)
	})

	t.Run("user must be on the team", func(t *testing.T) {
		otherTeamUser := th.CreateUser(t)
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), otherTeamUser.Email, otherTeamUser.Password)
		require.NoError(t, err)

		resp, err := client.DoAPIGet(context.Background(), "/teams/"+th.BasicTeam.Id+"/channels/recommended", "")
		require.Error(t, err)
		// resp can be nil on transport errors; only defer Close when we
		// actually got an HTTP response back so we can assert the status.
		require.NotNil(t, resp)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("returns policy-enforced channels the requester matches under enterprise license", func(t *testing.T) {
		// License + ABAC config gate the endpoint; without these it short-circuits
		// to an empty list (covered by the no-license subtest above).
		ok := th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		require.True(t, ok, "SetLicense should return true")
		t.Cleanup(func() { _ = th.App.Srv().RemoveLicense() })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.AccessControlSettings.EnableAttributeBasedAccessControl = new(true)
		})

		// Wire a mock ABAC service that allows the requester for `included`
		// and denies for `excluded`. This pins the api4 layer's responsibility
		// (routing, permissions, response shape) to a deterministic policy
		// outcome — the underlying CEL evaluation has its own coverage at the
		// app layer in TestGetRecommendedPublicChannelsForUser.
		mockACS := &einterfacesmocks.AccessControlServiceInterface{}
		originalACS := th.App.Srv().Channels().AccessControl
		th.App.Srv().Channels().AccessControl = mockACS
		t.Cleanup(func() { th.App.Srv().Channels().AccessControl = originalACS })
		// PermanentDeleteChannel during cleanup calls DeletePolicy on the ACS.
		mockACS.On("DeletePolicy", mock.Anything, mock.AnythingOfType("string")).
			Return((*model.AppError)(nil)).Maybe()

		included, _, _ := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        "abac-recommended-" + model.NewId(),
			DisplayName: "ABAC Recommended",
		})
		require.NotNil(t, included)
		t.Cleanup(func() {
			_ = th.App.PermanentDeleteChannel(th.Context, included)
		})

		excluded, _, _ := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        "abac-excluded-" + model.NewId(),
			DisplayName: "ABAC Excluded",
		})
		require.NotNil(t, excluded)
		t.Cleanup(func() {
			_ = th.App.PermanentDeleteChannel(th.Context, excluded)
		})

		// Stamp channel-scope policy rows so SearchAllChannels picks them up
		// as PolicyEnforced=true. The expressions are placeholders — the mock
		// ACS short-circuits evaluation below.
		for _, ch := range []*model.Channel{included, excluded} {
			_, err := th.App.Srv().Store().AccessControlPolicy().Save(th.Context, &model.AccessControlPolicy{
				ID:       ch.Id,
				Type:     model.AccessControlPolicyTypeChannel,
				Version:  model.AccessControlPolicyVersionV0_2,
				Revision: 1,
				Active:   true,
				Rules:    []model.AccessControlPolicyRule{{Actions: []string{"membership"}, Expression: "true"}},
			})
			require.NoError(t, err)
		}

		mockACS.On("AccessEvaluation", mock.Anything, mock.MatchedBy(func(req model.AccessRequest) bool {
			return req.Resource.ID == included.Id
		})).Return(model.AccessDecision{Decision: true}, (*model.AppError)(nil))
		mockACS.On("AccessEvaluation", mock.Anything, mock.MatchedBy(func(req model.AccessRequest) bool {
			return req.Resource.ID == excluded.Id
		})).Return(model.AccessDecision{Decision: false}, (*model.AppError)(nil))

		resp, err := th.Client.DoAPIGet(context.Background(), "/teams/"+th.BasicTeam.Id+"/channels/recommended", "")
		require.NoError(t, err)
		require.NotNil(t, resp)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var channels []*model.Channel
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&channels))

		ids := make(map[string]bool, len(channels))
		for _, ch := range channels {
			ids[ch.Id] = true
		}
		require.True(t, ids[included.Id], "policy-allowed channel must be returned")
		require.False(t, ids[excluded.Id], "policy-denied channel must be filtered out")
	})
}

func TestGetPublicChannelsByIdsForTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	teamId := th.BasicTeam.Id

	t.Run("should return 1 channel", func(t *testing.T) {
		input := []string{th.BasicChannel.Id}
		output := []string{th.BasicChannel.DisplayName}

		channels, _, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, input)
		require.NoError(t, err)
		require.Len(t, channels, 1, "should return 1 channel")
		require.Equal(t, output[0], channels[0].DisplayName, "missing channel")
	})

	t.Run("should return 2 channels", func(t *testing.T) {
		input := []string{th.BasicChannel.Id}
		expectedDisplayNames := []string{th.BasicChannel.DisplayName}

		input = append(input, GenerateTestID())
		input = append(input, th.BasicChannel2.Id)
		input = append(input, th.BasicPrivateChannel.Id)
		expectedDisplayNames = append(expectedDisplayNames, th.BasicChannel2.DisplayName)

		channels, _, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, input)
		require.NoError(t, err)
		require.Len(t, channels, 2, "should return 2 channels")

		actualDisplayNames := make([]string, len(channels))
		for i, c := range channels {
			actualDisplayNames[i] = c.DisplayName
		}
		require.ElementsMatch(t, expectedDisplayNames, actualDisplayNames, "missing channel")
	})

	t.Run("forbidden for invalid team", func(t *testing.T) {
		input := []string{th.BasicChannel.Id, th.BasicChannel2.Id}
		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), GenerateTestID(), input)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("bad request for empty input", func(t *testing.T) {
		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, []string{})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("bad request for junk id", func(t *testing.T) {
		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, []string{"junk"})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("not found for non-existent id", func(t *testing.T) {
		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, []string{GenerateTestID()})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("not found for private channel id", func(t *testing.T) {
		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, []string{th.BasicPrivateChannel.Id})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("unauthorized when logged out", func(t *testing.T) {
		input := []string{th.BasicChannel.Id, th.BasicChannel2.Id}
		_, lErr := client.Logout(context.Background())
		require.NoError(t, lErr)

		_, resp, err := client.GetPublicChannelsByIdsForTeam(context.Background(), teamId, input)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("system admin can get channels", func(t *testing.T) {
		input := []string{th.BasicChannel.Id, th.BasicChannel2.Id}
		_, _, err := th.SystemAdminClient.GetPublicChannelsByIdsForTeam(context.Background(), teamId, input)
		require.NoError(t, err)
	})

	t.Run("guest users should not be able to get channels", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		id := model.NewId()
		guestPassword := model.NewTestPassword()
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      guestPassword,
			EmailVerified: true,
		}
		guest, appErr := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, appErr)

		guestClient := th.CreateClient()
		_, _, err := guestClient.Login(context.Background(), guest.Username, guestPassword)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, lErr := guestClient.Logout(context.Background())
			require.NoError(t, lErr)
		})

		input := []string{th.BasicChannel.Id, th.BasicChannel2.Id}
		_, resp, err := guestClient.GetPublicChannelsByIdsForTeam(context.Background(), teamId, input)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestGetChannelsForTeamForUser(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	t.Run("get channels for the team for user", func(t *testing.T) {
		channels, resp, err := client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)

		found := make([]bool, 3)
		for _, c := range channels {
			if c.Id == th.BasicChannel.Id {
				found[0] = true
			} else if c.Id == th.BasicChannel2.Id {
				found[1] = true
			} else if c.Id == th.BasicPrivateChannel.Id {
				found[2] = true
			}

			require.True(t, c.TeamId == "" || c.TeamId == th.BasicTeam.Id)
		}

		for _, f := range found {
			require.True(t, f, "missing a channel")
		}

		channels, resp, err = client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, resp.Etag)
		require.NoError(t, err)
		CheckEtag(t, channels, resp)

		_, resp, err = client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, "junk", false, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.GetChannelsForTeamForUser(context.Background(), "junk", th.BasicUser.Id, false, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser2.Id, false, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = client.GetChannelsForTeamForUser(context.Background(), model.NewId(), th.BasicUser.Id, false, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, _, err = th.SystemAdminClient.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)
	})

	t.Run("deleted channel could be retrieved using the proper flag", func(t *testing.T) {
		testChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      th.BasicTeam.Id,
			CreatorId:   th.BasicUser.Id,
		}
		testChannel, appErr := th.App.CreateChannel(th.Context, testChannel, true)
		require.Nil(t, appErr)
		defer func() {
			appErr = th.App.PermanentDeleteChannel(th.Context, testChannel)
			require.Nil(t, appErr)
		}()

		channels, _, err := client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)
		assert.Equal(t, 6, len(channels))
		appErr = th.App.DeleteChannel(th.Context, testChannel, th.BasicUser.Id)
		require.Nil(t, appErr)
		channels, _, err = client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)
		assert.Equal(t, 5, len(channels))

		// Should return all channels including basicDeleted.
		channels, _, err = client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, true, "")
		require.NoError(t, err)
		assert.Equal(t, 7, len(channels))

		// Should stil return all channels including basicDeleted.
		now := time.Now().Add(-time.Minute).Unix() * 1000
		channels, _, err = client.GetChannelsForTeamAndUserWithLastDeleteAt(context.Background(), th.BasicTeam.Id, th.BasicUser.Id,
			true, int(now), "")
		require.NoError(t, err)
		assert.Equal(t, 7, len(channels))
	})
}

func TestGetChannelsForUser(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	client := th.Client

	// Adding another team with more channels (public and private)
	myTeam := th.CreateTeam(t)
	ch1 := th.CreateChannelWithClientAndTeam(t, client, model.ChannelTypeOpen, myTeam.Id)
	ch2 := th.CreateChannelWithClientAndTeam(t, client, model.ChannelTypePrivate, myTeam.Id)
	th.LinkUserToTeam(t, th.BasicUser, myTeam)
	_, appErr := th.App.AddUserToChannel(th.Context, th.BasicUser, ch1, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser, ch2, false)
	require.Nil(t, appErr)

	channels, _, err := client.GetChannelsForUserWithLastDeleteAt(context.Background(), th.BasicUser.Id, 0)
	require.NoError(t, err)

	numPrivate := 0
	numPublic := 0
	numOffTopic := 0
	numTownSquare := 0
	for _, ch := range channels {
		if ch.Type == model.ChannelTypeOpen {
			numPublic++
		} else if ch.Type == model.ChannelTypePrivate {
			numPrivate++
		}

		if ch.DisplayName == "Off-Topic" {
			numOffTopic++
		} else if ch.DisplayName == "Town Square" {
			numTownSquare++
		}
	}

	assert.Len(t, channels, 9)
	assert.Equal(t, 2, numPrivate)
	assert.Equal(t, 7, numPublic)
	assert.Equal(t, 2, numOffTopic)
	assert.Equal(t, 2, numTownSquare)

	// Creating some more channels to be exactly 100 to test page size boundaries.
	for range 91 {
		ch1 = th.CreateChannelWithClientAndTeam(t, client, model.ChannelTypeOpen, myTeam.Id)
		_, appErr := th.App.AddUserToChannel(th.Context, th.BasicUser, ch1, false)
		require.Nil(t, appErr)
	}

	channels, _, err = client.GetChannelsForUserWithLastDeleteAt(context.Background(), th.BasicUser.Id, 0)
	require.NoError(t, err)
	assert.Len(t, channels, 100)
}

func TestGetAllChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	th.LoginSystemManager(t)
	client := th.Client

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channels, _, err := client.GetAllChannels(context.Background(), 0, 20, "")
		require.NoError(t, err)

		// At least, all the not-deleted channels created during the InitBasic
		require.True(t, len(channels) >= 3)
		for _, c := range channels {
			require.NotEqual(t, c.TeamId, "")
		}

		channels, _, err = client.GetAllChannels(context.Background(), 0, 10, "")
		require.NoError(t, err)
		require.True(t, len(channels) >= 3)

		channels, _, err = client.GetAllChannels(context.Background(), 1, 1, "")
		require.NoError(t, err)
		require.Len(t, channels, 1)

		channels, _, err = client.GetAllChannels(context.Background(), 10000, 10000, "")
		require.NoError(t, err)
		require.Empty(t, channels)

		channels, _, err = client.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		beforeCount := len(channels)

		deletedChannel := channels[0].Channel

		// Never try to delete the default channel
		if deletedChannel.Name == "town-square" {
			deletedChannel = channels[1].Channel
		}

		_, err = client.DeleteChannel(context.Background(), deletedChannel.Id)
		require.NoError(t, err)

		channels, _, err = client.GetAllChannels(context.Background(), 0, 10000, "")
		var ids []string
		for _, item := range channels {
			ids = append(ids, item.Channel.Id)
		}
		require.NoError(t, err)
		require.Len(t, channels, beforeCount-1)
		require.NotContains(t, ids, deletedChannel.Id)

		channels, _, err = client.GetAllChannelsIncludeDeleted(context.Background(), 0, 10000, "")
		ids = []string{}
		for _, item := range channels {
			ids = append(ids, item.Channel.Id)
		}
		require.NoError(t, err)
		require.True(t, len(channels) > beforeCount)
		require.Contains(t, ids, deletedChannel.Id)
	})

	_, resp, err := client.GetAllChannels(context.Background(), 0, 20, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	sysManagerChannels, resp, err := th.SystemManagerClient.GetAllChannels(context.Background(), 0, 10000, "")
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	policyChannel := (sysManagerChannels)[0]
	policy, err := th.App.Srv().Store().RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:      "Policy 1",
			PostDurationDays: new(int64(30)),
		},
		ChannelIDs: []string{policyChannel.Id},
	})
	require.NoError(t, err)

	t.Run("exclude policy constrained", func(t *testing.T) {
		_, resp, err := th.SystemManagerClient.GetAllChannelsExcludePolicyConstrained(context.Background(), 0, 10000, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		channels, resp, err := th.SystemAdminClient.GetAllChannelsExcludePolicyConstrained(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range channels {
			if channel.Id == policyChannel.Id {
				found = true
				break
			}
		}
		require.False(t, found)
	})

	t.Run("does not return policy ID", func(t *testing.T) {
		channels, resp, err := th.SystemManagerClient.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Nil(t, channel.PolicyID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("returns policy ID", func(t *testing.T) {
		channels, resp, err := th.SystemAdminClient.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Equal(t, *channel.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("verify correct sanitization", func(t *testing.T) {
		channels, resp, err := th.SystemAdminClient.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, len(channels) > 0)
		for _, channel := range channels {
			if channel.DisplayName != "Off-Topic" && channel.DisplayName != "Town Square" {
				require.NotEqual(t, "", channel.CreatorId)
				require.NotEqual(t, "", channel.Name)
			}
		}

		channels, resp, err = th.SystemManagerClient.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, len(channels) > 0)
		for _, channel := range channels {
			if channel.DisplayName != "Off-Topic" && channel.DisplayName != "Town Square" {
				require.NotEqual(t, "", channel.CreatorId)
				require.NotEqual(t, "", channel.Name)
			}
		}

		th.RemovePermissionFromRole(t, model.PermissionSysconsoleReadUserManagementChannels.Id, model.SystemManagerRoleId)
		channels, resp, err = th.SystemManagerClient.GetAllChannels(context.Background(), 0, 10000, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, len(channels) > 0)
		for _, channel := range channels {
			require.Equal(t, "", channel.CreatorId)
			require.Equal(t, "", channel.Name)
		}
	})
}

func TestGetAllChannelsWithCount(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	channels, total, _, err := th.SystemAdminClient.GetAllChannelsWithCount(context.Background(), 0, 20, "")
	require.NoError(t, err)

	// At least, all the not-deleted channels created during the InitBasic
	require.True(t, len(channels) >= 3)
	for _, c := range channels {
		require.NotEqual(t, c.TeamId, "")
	}
	require.Equal(t, int64(6), total)

	channels, _, _, err = th.SystemAdminClient.GetAllChannelsWithCount(context.Background(), 0, 10, "")
	require.NoError(t, err)
	require.True(t, len(channels) >= 3)

	channels, _, _, err = th.SystemAdminClient.GetAllChannelsWithCount(context.Background(), 1, 1, "")
	require.NoError(t, err)
	require.Len(t, channels, 1)

	channels, _, _, err = th.SystemAdminClient.GetAllChannelsWithCount(context.Background(), 10000, 10000, "")
	require.NoError(t, err)
	require.Empty(t, channels)

	_, _, resp, err := client.GetAllChannelsWithCount(context.Background(), 0, 20, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestSearchChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	t.Run("Search using null value", func(t *testing.T) {
		var nullSearch *model.ChannelSearch

		_, resp, err := client.SearchChannels(context.Background(), th.BasicTeam.Id, nullSearch)

		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	channels, _, err := client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
	require.NoError(t, err)

	found := false
	for _, c := range channels {
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should only return public channels")

		if c.Id == th.BasicChannel.Id {
			found = true
		}
	}
	require.True(t, found, "didn't find channel")

	search.Term = th.BasicPrivateChannel.Name
	channels, _, err = client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
	require.NoError(t, err)

	found = false
	for _, c := range channels {
		if c.Id == th.BasicPrivateChannel.Id {
			found = true
		}
	}
	require.False(t, found, "shouldn't find private channel")

	search.Term = ""
	_, _, err = client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
	require.NoError(t, err)

	search.Term = th.BasicChannel.Name
	_, resp, err := client.SearchChannels(context.Background(), model.NewId(), search)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.SearchChannels(context.Background(), "junk", search)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, _, err = th.SystemAdminClient.SearchChannels(context.Background(), th.BasicTeam.Id, search)
	require.NoError(t, err)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer func() {
		th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(t, model.PermissionListTeamChannels.Id, model.TeamUserRoleId)

	t.Run("Search for a BasicChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicChannel.Name
		channelList, _, err := client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
		require.NoError(t, err)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicChannel.Name)
	})

	t.Run("Remove the user from BasicChannel and search again, should not be returned", func(t *testing.T) {
		appErr := th.App.RemoveUserFromChannel(th.Context, th.BasicUser.Id, th.BasicUser.Id, th.BasicChannel)
		require.Nil(t, appErr)

		search.Term = th.BasicChannel.Name
		channelList, _, err := client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
		require.NoError(t, err)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.NotContains(t, channelNames, th.BasicChannel.Name)
	})

	t.Run("Guests only receive autocompletion for which accounts they are a member of", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense(""))
		defer th.App.Srv().SetLicense(nil)

		enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.GuestAccountsSettings.Enable = &enableGuestAccounts })
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		guest := th.CreateUser(t)
		_, appErr := th.SystemAdminClient.DemoteUserToGuest(context.Background(), guest.Id)
		require.NoError(t, appErr)

		_, resp, err := th.SystemAdminClient.AddTeamMember(context.Background(), th.BasicTeam.Id, guest.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		_, resp, err = client.Login(context.Background(), guest.Username, guest.Password)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		search.Term = th.BasicChannel2.Name
		channelList, _, err := client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
		require.NoError(t, err)

		require.Empty(t, channelList)

		_, resp, err = th.SystemAdminClient.AddChannelMember(context.Background(), th.BasicChannel2.Id, guest.Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		search.Term = th.BasicChannel2.Name
		channelList, _, err = client.SearchChannels(context.Background(), th.BasicTeam.Id, search)
		require.NoError(t, err)

		require.NotEmpty(t, channelList)
		require.Equal(t, th.BasicChannel2.Id, channelList[0].Id)
	})
}

func TestSearchAllChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupForSharedChannels(t).InitBasic(t)

	th.LoginSystemManager(t)
	client := th.Client

	openChannel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "SearchAllChannels-FOOBARDISPLAYNAME",
		Name:        "whatever",
		Type:        model.ChannelTypeOpen,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)

	privateChannel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "SearchAllChannels-private1",
		Name:        "private1",
		Type:        model.ChannelTypePrivate,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)

	team := th.CreateTeam(t)
	privateChannel2, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "dn_private2",
		Name:        "private2",
		Type:        model.ChannelTypePrivate,
		TeamId:      team.Id,
	})
	require.NoError(t, err)
	th.LinkUserToTeam(t, th.SystemAdminUser, team)
	th.LinkUserToTeam(t, th.SystemAdminUser, th.BasicTeam)

	groupConstrainedChannel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName:      "SearchAllChannels-groupConstrained-1",
		Name:             "groupconstrained1",
		Type:             model.ChannelTypePrivate,
		GroupConstrained: new(true),
		TeamId:           team.Id,
	})
	require.NoError(t, err)

	// share the open and private channels, one homed locally and the
	// other remotely
	sco := &model.SharedChannel{
		ChannelId: openChannel.Id,
		TeamId:    openChannel.TeamId,
		Home:      true,
		ShareName: "testsharelocal",
		CreatorId: th.BasicChannel.CreatorId,
	}
	_, scoErr := th.App.ShareChannel(th.Context, sco)
	require.NoError(t, scoErr)

	scp := &model.SharedChannel{
		ChannelId: privateChannel.Id,
		TeamId:    privateChannel.TeamId,
		Home:      false,
		RemoteId:  model.NewId(),
		ShareName: "testshareremote",
		CreatorId: th.BasicChannel.CreatorId,
	}
	_, scpErr := th.App.ShareChannel(th.Context, scp)
	require.NoError(t, scpErr)

	testCases := []struct {
		Description        string
		Search             *model.ChannelSearch
		ExpectedChannelIds []string
	}{
		{
			"Middle of word search",
			&model.ChannelSearch{Term: "bardisplay"},
			[]string{openChannel.Id},
		},
		{
			"Prefix search",
			&model.ChannelSearch{Term: "SearchAllChannels-foobar"},
			[]string{openChannel.Id},
		},
		{
			"Suffix search",
			&model.ChannelSearch{Term: "displayname"},
			[]string{openChannel.Id},
		},
		{
			"Name search",
			// Prefix of Name "whatever". Term "what" also matches InitBasic DisplayNames
			// ("dn_"+NewId()); z-base32 includes w/h/a/t but not v.
			&model.ChannelSearch{Term: "whatev"},
			[]string{openChannel.Id},
		},
		{
			"Name suffix search",
			&model.ChannelSearch{Term: "ever"},
			[]string{openChannel.Id},
		},
		{
			"Basic channel name middle of word search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[2:14]},
			[]string{th.BasicChannel.Id},
		},
		{
			"Upper case search",
			&model.ChannelSearch{Term: strings.ToUpper(th.BasicChannel.Name)},
			[]string{th.BasicChannel.Id},
		},
		{
			"Mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[0:2] + strings.ToUpper(th.BasicChannel.Name[2:5]) + th.BasicChannel.Name[5:]},
			[]string{th.BasicChannel.Id},
		},
		{
			"Non mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name},
			[]string{th.BasicChannel.Id},
		},
		{
			"Search private channel name",
			&model.ChannelSearch{Term: th.BasicPrivateChannel.Name},
			[]string{th.BasicPrivateChannel.Id},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Private: true},
			[]string{th.BasicPrivateChannel.Id, privateChannel2.Id, th.BasicPrivateChannel2.Id, privateChannel.Id, groupConstrainedChannel.Id},
		},
		{
			"Search with public channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Public: true},
			[]string{openChannel.Id},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Private: true},
			[]string{privateChannel.Id, groupConstrainedChannel.Id},
		},
		{
			"Search with teamIds channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", TeamIds: []string{th.BasicTeam.Id}},
			[]string{openChannel.Id, privateChannel.Id},
		},
		{
			"Search with deleted without IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name},
			[]string{},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search with deleted Deleted filter and empty term",
			&model.ChannelSearch{Term: "", Deleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search for group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true},
			[]string{groupConstrainedChannel.Id},
		},
		{
			"Search for group constrained and public",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true, Public: true},
			[]string{},
		},
		{
			"Search for exclude group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", ExcludeGroupConstrained: true},
			[]string{openChannel.Id, privateChannel.Id},
		},
		{
			"Search for local only channels",
			&model.ChannelSearch{Term: "SearchAllChannels", ExcludeRemote: true},
			[]string{openChannel.Id, groupConstrainedChannel.Id},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			var channels model.ChannelListWithTeamData
			channels, _, err = th.SystemAdminClient.SearchAllChannels(context.Background(), testCase.Search)
			require.NoError(t, err)
			assert.Equal(t, len(testCase.ExpectedChannelIds), len(channels))
			actualChannelIds := []string{}
			for _, channelWithTeamData := range channels {
				actualChannelIds = append(actualChannelIds, channelWithTeamData.Channel.Id)
			}
			assert.ElementsMatch(t, testCase.ExpectedChannelIds, actualChannelIds)
		})
	}

	userChannels, _, err := th.SystemAdminClient.SearchAllChannelsForUser(context.Background(), "private")
	require.NoError(t, err)
	assert.Len(t, userChannels, 2)

	userChannels, _, err = th.SystemAdminClient.SearchAllChannelsForUser(context.Background(), "FOOBARDISPLAYNAME")
	require.NoError(t, err)
	assert.Len(t, userChannels, 1)

	// Searching with no terms returns all default channels
	allChannels, _, err := th.SystemAdminClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: ""})
	require.NoError(t, err)
	assert.True(t, len(allChannels) >= 3)

	_, resp, err := client.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: ""})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Choose a policy which the system manager can read
	sysManagerChannels, resp, err := th.SystemManagerClient.GetAllChannels(context.Background(), 0, 10000, "")
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	policyChannel := sysManagerChannels[0]
	policy, savePolicyErr := th.App.Srv().Store().RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:      "Policy 1",
			PostDurationDays: new(int64(30)),
		},
		ChannelIDs: []string{policyChannel.Id},
	})
	require.NoError(t, savePolicyErr)

	t.Run("does not return policy ID", func(t *testing.T) {
		channels, resp, err := th.SystemManagerClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: policyChannel.Name})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Nil(t, channel.PolicyID)
				break
			}
		}
		require.True(t, found)
	})
	t.Run("returns policy ID", func(t *testing.T) {
		channels, resp, err := th.SystemAdminClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: policyChannel.Name})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Equal(t, *channel.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("verify correct sanitization", func(t *testing.T) {
		channels, resp, err := th.SystemAdminClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: ""})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, len(channels) > 0)
		for _, channel := range channels {
			if channel.DisplayName != "Off-Topic" && channel.DisplayName != "Town Square" {
				require.NotEqual(t, "", channel.CreatorId)
				require.NotEqual(t, "", channel.Name)
			}
		}

		channels, resp, err = th.SystemManagerClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: ""})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, len(channels) > 0)
		for _, channel := range channels {
			if channel.DisplayName != "Off-Topic" && channel.DisplayName != "Town Square" {
				require.NotEqual(t, "", channel.CreatorId)
				require.NotEqual(t, "", channel.Name)
			}
		}

		th.RemovePermissionFromRole(t, model.PermissionSysconsoleReadUserManagementChannels.Id, model.SystemManagerRoleId)
		channels, resp, err = th.SystemManagerClient.SearchAllChannels(context.Background(), &model.ChannelSearch{Term: ""})
		require.NoError(t, err)
		require.True(t, len(channels) > 0)
		CheckOKStatus(t, resp)
		for _, channel := range channels {
			require.Equal(t, "", channel.CreatorId)
			require.Equal(t, "", channel.Name)
		}
	})
}

func TestSearchAllChannelsNonSysConsoleFiltered(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	// Create a team admin user
	teamAdmin := th.CreateUser(t)
	th.LinkUserToTeam(t, teamAdmin, th.BasicTeam)
	th.UpdateUserToTeamAdmin(t, teamAdmin, th.BasicTeam)
	teamAdminClient := th.CreateClient()
	_, _, err := teamAdminClient.Login(context.Background(), teamAdmin.Email, teamAdmin.Password)
	require.NoError(t, err)

	// Create >50 public channels so the 50-result cap would normally cut off private channels
	pubCount := 0
	for range 55 {
		ch, _, chErr := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
			TeamId:      th.BasicTeam.Id,
			DisplayName: fmt.Sprintf("Public Cap Channel %03d", pubCount),
			Name:        model.NewId()[:20],
			Type:        model.ChannelTypeOpen,
		})
		require.NoError(t, chErr)
		_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), ch.Id, teamAdmin.Id)
		require.NoError(t, err)
		pubCount++
	}

	// Create private channels the team admin is a member of
	private1, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Private Filter Test 1",
		Name:        model.NewId()[:20],
		Type:        model.ChannelTypePrivate,
	})
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), private1.Id, teamAdmin.Id)
	require.NoError(t, err)

	private2, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Private Filter Test 2",
		Name:        model.NewId()[:20],
		Type:        model.ChannelTypePrivate,
	})
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), private2.Id, teamAdmin.Id)
	require.NoError(t, err)

	t.Run("private filter returns private member channels despite >50 public channels", func(t *testing.T) {
		channels, _, err := teamAdminClient.SearchAllChannelsForUserWithOpts(
			context.Background(),
			&model.ChannelSearch{
				TeamIds: []string{th.BasicTeam.Id},
				Private: true,
			},
		)
		require.NoError(t, err)

		ids := make([]string, 0, len(channels))
		for _, ch := range channels {
			ids = append(ids, ch.Id)
			assert.Equal(t, model.ChannelTypePrivate, ch.Type, "only private channels should be returned")
		}
		assert.Contains(t, ids, private1.Id)
		assert.Contains(t, ids, private2.Id)
	})
}

func TestSearchAllChannelsPaged(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}
	search.Term = ""
	search.Page = new(0)
	search.PerPage = new(2)
	channelsWithCount, _, err := th.SystemAdminClient.SearchAllChannelsPaged(context.Background(), search)
	require.NoError(t, err)
	require.Len(t, channelsWithCount.Channels, 2)

	search.Term = th.BasicChannel.Name
	_, resp, err := client.SearchAllChannels(context.Background(), search)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestSearchGroupChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	u1 := th.CreateUserWithClient(t, th.SystemAdminClient)

	// Create a group channel in which base user belongs but not sysadmin
	gc1, _, err := th.Client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, u1.Id})
	require.NoError(t, err)

	gc2, _, err := th.Client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id})
	require.NoError(t, err)

	search := &model.ChannelSearch{Term: th.BasicUser2.Username}

	// sysadmin should only find gc2 as he doesn't belong to gc1
	channels, _, err := th.SystemAdminClient.SearchGroupChannels(context.Background(), search)
	require.NoError(t, err)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// basic user should find both
	_, _, err = client.Login(context.Background(), th.BasicUser.Username, th.BasicUser.Password)
	require.NoError(t, err)
	channels, _, err = client.SearchGroupChannels(context.Background(), search)
	require.NoError(t, err)

	assert.Len(t, channels, 2)
	channelIds := []string{}
	for _, c := range channels {
		channelIds = append(channelIds, c.Id)
	}
	assert.ElementsMatch(t, channelIds, []string{gc1.Id, gc2.Id})

	// searching for sysadmin, it should only find gc1
	search = &model.ChannelSearch{Term: th.SystemAdminUser.Username}
	channels, _, err = client.SearchGroupChannels(context.Background(), search)
	require.NoError(t, err)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// with an empty search, response should be empty
	search = &model.ChannelSearch{Term: ""}
	channels, _, err = client.SearchGroupChannels(context.Background(), search)
	require.NoError(t, err)

	assert.Empty(t, channels)

	// search unprivileged, forbidden
	_, err = th.Client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err := client.SearchAllChannels(context.Background(), search)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	t.Run("search with null value", func(t *testing.T) {
		var search *model.ChannelSearch
		_, _, err := client.Login(context.Background(), th.BasicUser.Username, th.BasicUser.Password)
		require.NoError(t, err)
		_, resp, err := client.SearchGroupChannels(context.Background(), search)

		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestDeleteChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	c := th.Client
	team := th.BasicTeam
	user := th.BasicUser
	user2 := th.BasicUser2

	// successful delete of public channel
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		publicChannel1 := th.CreatePublicChannel(t)
		_, err := client.DeleteChannel(context.Background(), publicChannel1.Id)
		require.NoError(t, err)

		ch, appErr := th.App.GetChannel(th.Context, publicChannel1.Id)
		require.Nil(t, appErr)
		require.True(t, ch.DeleteAt != 0, "should have returned one with a populated DeleteAt.")

		post1 := &model.Post{ChannelId: publicChannel1.Id, Message: "a" + GenerateTestID() + "a"}
		_, resp, err := client.CreatePost(context.Background(), post1)
		require.Error(t, err)
		require.NotNil(t, resp, "expected response to not be nil")

		// successful delete of private channel
		privateChannel2 := th.CreatePrivateChannel(t)
		_, err = client.DeleteChannel(context.Background(), privateChannel2.Id)
		require.NoError(t, err)

		// successful delete of channel with multiple members
		publicChannel3 := th.CreatePublicChannel(t)
		_, appErr = th.App.AddUserToChannel(th.Context, user, publicChannel3, false)
		require.Nil(t, appErr)
		_, appErr = th.App.AddUserToChannel(th.Context, user2, publicChannel3, false)
		require.Nil(t, appErr)
		_, err = client.DeleteChannel(context.Background(), publicChannel3.Id)
		require.NoError(t, err)

		// default channel cannot be deleted.
		defaultChannel, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
		require.Nil(t, appErr)
		resp, err = client.DeleteChannel(context.Background(), defaultChannel.Id)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		// check system admin can delete a channel without any appropriate team or channel membership.
		sdTeam := th.CreateTeamWithClient(t, c)
		sdPublicChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamId:      sdTeam.Id,
		}
		sdPublicChannel, _, err = c.CreateChannel(context.Background(), sdPublicChannel)
		require.NoError(t, err)
		_, err = client.DeleteChannel(context.Background(), sdPublicChannel.Id)
		require.NoError(t, err)

		sdPrivateChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypePrivate,
			TeamId:      sdTeam.Id,
		}
		sdPrivateChannel, _, err = c.CreateChannel(context.Background(), sdPrivateChannel)
		require.NoError(t, err)
		_, err = client.DeleteChannel(context.Background(), sdPrivateChannel.Id)
		require.NoError(t, err)
	})

	th.LoginBasic(t)
	publicChannel5 := th.CreatePublicChannel(t)
	_, err := c.Logout(context.Background())
	require.NoError(t, err)

	// Other users can't delete the channel
	_, _, err = c.Login(context.Background(), user2.Email, user2.Password)
	require.NoError(t, err)
	resp, err := c.DeleteChannel(context.Background(), publicChannel5.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	resp, err = c.DeleteChannel(context.Background(), "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, err = c.Logout(context.Background())
	require.NoError(t, err)
	resp, err = c.DeleteChannel(context.Background(), GenerateTestID())
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	// The creator can delete the channel
	_, err = c.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = c.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	resp, err = c.DeleteChannel(context.Background(), publicChannel5.Id)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
}

func TestDeleteChannel2(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer func() {
		th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
	}()

	th.AddPermissionToRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelUserRoleId)
	th.AddPermissionToRole(t, model.PermissionDeletePrivateChannel.Id, model.ChannelUserRoleId)

	// channels created by SystemAdmin
	publicChannel6 := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypeOpen)
	privateChannel7 := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypePrivate)
	_, appErr := th.App.AddUserToChannel(th.Context, user, publicChannel6, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, user, privateChannel7, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, user, privateChannel7, false)
	require.Nil(t, appErr)

	// successful delete by user
	_, err := client.DeleteChannel(context.Background(), publicChannel6.Id)
	require.NoError(t, err)

	_, err = client.DeleteChannel(context.Background(), privateChannel7.Id)
	require.NoError(t, err)

	// Restrict permissions to Channel Admins
	th.RemovePermissionFromRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelUserRoleId)
	th.RemovePermissionFromRole(t, model.PermissionDeletePrivateChannel.Id, model.ChannelUserRoleId)
	th.AddPermissionToRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelAdminRoleId)
	th.AddPermissionToRole(t, model.PermissionDeletePrivateChannel.Id, model.ChannelAdminRoleId)

	// channels created by SystemAdmin
	publicChannel6 = th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypeOpen)
	privateChannel7 = th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypePrivate)
	_, appErr = th.App.AddUserToChannel(th.Context, user, publicChannel6, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, user, privateChannel7, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, user, privateChannel7, false)
	require.Nil(t, appErr)

	// cannot delete by user
	resp, err := client.DeleteChannel(context.Background(), publicChannel6.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	resp, err = client.DeleteChannel(context.Background(), privateChannel7.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// successful delete by channel admin
	th.MakeUserChannelAdmin(t, user, publicChannel6)
	th.MakeUserChannelAdmin(t, user, privateChannel7)
	th.App.Srv().Store().Channel().ClearCaches()

	_, err = client.DeleteChannel(context.Background(), publicChannel6.Id)
	require.NoError(t, err)

	_, err = client.DeleteChannel(context.Background(), privateChannel7.Id)
	require.NoError(t, err)

	// Make sure team admins don't have permission to delete channels.
	th.RemovePermissionFromRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelAdminRoleId)
	th.RemovePermissionFromRole(t, model.PermissionDeletePrivateChannel.Id, model.ChannelAdminRoleId)

	// last member of a public channel should have required permission to delete
	publicChannel6 = th.CreateChannelWithClient(t, th.Client, model.ChannelTypeOpen)
	resp, err = client.DeleteChannel(context.Background(), publicChannel6.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// last member of a private channel should not be able to delete it if they don't have required permissions
	privateChannel7 = th.CreateChannelWithClient(t, th.Client, model.ChannelTypePrivate)
	resp, err = client.DeleteChannel(context.Background(), privateChannel7.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestPermanentDeleteChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	enableAPIChannelDeletion := *th.App.Config().ServiceSettings.EnableAPIChannelDeletion
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableAPIChannelDeletion = &enableAPIChannelDeletion })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIChannelDeletion = false })

	publicChannel1 := th.CreatePublicChannel(t)
	t.Run("Permanent deletion not available through API if EnableAPIChannelDeletion is not set", func(t *testing.T) {
		resp, err := th.SystemAdminClient.PermanentDeleteChannel(context.Background(), publicChannel1.Id)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Permanent deletion available through local mode even if EnableAPIChannelDeletion is not set", func(t *testing.T) {
		_, err := th.LocalClient.PermanentDeleteChannel(context.Background(), publicChannel1.Id)
		require.NoError(t, err)
	})

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIChannelDeletion = true })
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		publicChannel := th.CreatePublicChannel(t)
		_, err := c.PermanentDeleteChannel(context.Background(), publicChannel.Id)
		require.NoError(t, err)

		_, appErr := th.App.GetChannel(th.Context, publicChannel.Id)
		assert.NotNil(t, appErr)

		resp, err := c.PermanentDeleteChannel(context.Background(), "junk")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	}, "Permanent deletion with EnableAPIChannelDeletion set")
}

func TestUpdateChannelPrivacy(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	defaultChannel, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, th.BasicTeam.Id, false)
	require.Nil(t, appErr)

	type testTable []struct {
		name            string
		channel         *model.Channel
		expectedPrivacy model.ChannelType
	}

	t.Run("Should get a forbidden response if not logged in", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel(t)
		publicChannel := th.CreatePublicChannel(t)

		tt := testTable{
			{"Updating default channel should fail with forbidden status if not logged in", defaultChannel, model.ChannelTypeOpen},
			{"Updating private channel should fail with forbidden status if not logged in", privateChannel, model.ChannelTypePrivate},
			{"Updating public channel should fail with forbidden status if not logged in", publicChannel, model.ChannelTypeOpen},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp, err := th.Client.UpdateChannelPrivacy(context.Background(), tc.channel.Id, tc.expectedPrivacy)
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
			})
		}
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel(t)
		publicChannel := th.CreatePublicChannel(t)

		tt := testTable{
			{"Converting default channel to private should fail", defaultChannel, model.ChannelTypePrivate},
			{"Updating privacy to an invalid setting should fail", publicChannel, "invalid"},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp, err := client.UpdateChannelPrivacy(context.Background(), tc.channel.Id, tc.expectedPrivacy)
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
			})
		}

		tt = testTable{
			{"Default channel should stay public", defaultChannel, model.ChannelTypeOpen},
			{"Public channel should stay public", publicChannel, model.ChannelTypeOpen},
			{"Private channel should stay private", privateChannel, model.ChannelTypePrivate},
			{"Public channel should convert to private", publicChannel, model.ChannelTypePrivate},
			{"Private channel should convert to public", privateChannel, model.ChannelTypeOpen},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				updatedChannel, _, err := client.UpdateChannelPrivacy(context.Background(), tc.channel.Id, tc.expectedPrivacy)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
				updatedChannel, appErr := th.App.GetChannel(th.Context, tc.channel.Id)
				require.Nil(t, appErr)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
			})
		}
	})

	t.Run("Enforces convert channel permissions", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel(t)
		publicChannel := th.CreatePublicChannel(t)

		th.LoginTeamAdmin(t)

		th.RemovePermissionFromRole(t, model.PermissionConvertPublicChannelToPrivate.Id, model.TeamAdminRoleId)
		th.RemovePermissionFromRole(t, model.PermissionConvertPrivateChannelToPublic.Id, model.TeamAdminRoleId)

		_, resp, err := th.Client.UpdateChannelPrivacy(context.Background(), publicChannel.Id, model.ChannelTypePrivate)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = th.Client.UpdateChannelPrivacy(context.Background(), privateChannel.Id, model.ChannelTypeOpen)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		th.AddPermissionToRole(t, model.PermissionConvertPublicChannelToPrivate.Id, model.TeamAdminRoleId)
		th.AddPermissionToRole(t, model.PermissionConvertPrivateChannelToPublic.Id, model.TeamAdminRoleId)

		_, _, err = th.Client.UpdateChannelPrivacy(context.Background(), privateChannel.Id, model.ChannelTypeOpen)
		require.NoError(t, err)
		_, _, err = th.Client.UpdateChannelPrivacy(context.Background(), publicChannel.Id, model.ChannelTypePrivate)
		require.NoError(t, err)
	})
}

func TestRestoreChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	publicChannel1 := th.CreatePublicChannel(t)
	_, err := th.Client.DeleteChannel(context.Background(), publicChannel1.Id)
	require.NoError(t, err)

	privateChannel1 := th.CreatePrivateChannel(t)
	_, err = th.Client.DeleteChannel(context.Background(), privateChannel1.Id)
	require.NoError(t, err)

	_, resp, err := th.Client.RestoreChannel(context.Background(), publicChannel1.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = th.Client.RestoreChannel(context.Background(), privateChannel1.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Make BasicUser, a User Manager
	oldRoles := th.BasicUser.Roles

	// Because the permissions get set on initialization,
	// remove the manage_team permission from the User Management Role
	th.RemovePermissionFromRole(t, model.PermissionManageTeam.Id, model.SystemUserManagerRoleId)
	_, appErr := th.App.UpdateUserRoles(th.Context, th.BasicUser.Id, model.SystemUserManagerRoleId, false)
	require.Nil(t, appErr)
	defer func() {
		_, appErr = th.App.UpdateUserRoles(th.Context, th.BasicUser.Id, oldRoles, false)
		require.Nil(t, appErr)
	}()
	appErr = th.App.Srv().InvalidateAllCaches()
	require.Nil(t, appErr)
	_, _, err = th.Client.Login(context.Background(), th.BasicUser.Email, th.BasicUser.Password)
	require.NoError(t, err)

	_, resp, err = th.Client.RestoreChannel(context.Background(), publicChannel1.Id)
	require.NoError(t, err)
	CheckOKStatus(t, resp)

	_, resp, err = th.Client.RestoreChannel(context.Background(), privateChannel1.Id)
	require.NoError(t, err)
	CheckOKStatus(t, resp)

	_, err = th.Client.DeleteChannel(context.Background(), publicChannel1.Id)
	require.NoError(t, err)
	_, err = th.Client.DeleteChannel(context.Background(), privateChannel1.Id)
	require.NoError(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		defer func() {
			_, err = client.DeleteChannel(context.Background(), publicChannel1.Id)
			require.NoError(t, err)
			_, err = client.DeleteChannel(context.Background(), privateChannel1.Id)
			require.NoError(t, err)
		}()

		_, resp, err = client.RestoreChannel(context.Background(), publicChannel1.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		_, resp, err = client.RestoreChannel(context.Background(), privateChannel1.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}

func TestGetChannelByName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	channel, _, err := client.GetChannelByName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, _, err = client.GetChannelByName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, _, err = client.GetChannelByName(context.Background(), strings.ToUpper(th.BasicPrivateChannel.Name), th.BasicTeam.Id, "")
	require.NoError(t, err)

	_, resp, err := client.GetChannelByName(context.Background(), th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	channel, _, err = client.GetChannelByNameIncludeDeleted(context.Background(), th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, _, err = client.GetChannelByName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Id, "")
	require.NoError(t, err)

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicPrivateChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, resp, err = client.GetChannelByName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelByName(context.Background(), GenerateTestChannelName(), th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelByName(context.Background(), GenerateTestChannelName(), "junk", "")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannelByName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = client.GetChannelByName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, err = client.GetChannelByName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Id, "")
		require.NoError(t, err)
	})

	_, err = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), th.BasicPrivateChannel.Id, th.TeamAdminUser.Id)
	require.NoError(t, err)
	TeamAdminClient := th.CreateClient()
	th.LoginTeamAdminWithClient(t, TeamAdminClient)
	channel, _, err = TeamAdminClient.GetChannelByName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")
}

func TestGetChannelByNameForTeamName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	channel, _, err := th.SystemAdminClient.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	_, err = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), th.BasicPrivateChannel.Id, th.TeamAdminUser.Id)
	require.NoError(t, err)
	TeamAdminClient := th.CreateClient()
	th.LoginTeamAdminWithClient(t, TeamAdminClient)
	channel, _, err = TeamAdminClient.GetChannelByNameForTeamName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	channel, _, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, _, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, resp, err := client.GetChannelByNameForTeamName(context.Background(), th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	channel, _, err = client.GetChannelByNameForTeamNameIncludeDeleted(context.Background(), th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, _, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Name, "")
	require.NoError(t, err)

	_, err = client.RemoveUserFromChannel(context.Background(), th.BasicPrivateChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	_, resp, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicPrivateChannel.Name, th.BasicTeam.Name, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, model.NewRandomString(15), "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelByNameForTeamName(context.Background(), GenerateTestChannelName(), th.BasicTeam.Name, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Name, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = client.GetChannelByNameForTeamName(context.Background(), th.BasicChannel.Name, th.BasicTeam.Name, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembers(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		members, _, err := client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 0, 60, "")
		require.NoError(t, err)
		require.Len(t, members, 3, "should only be 3 users in channel")

		members, _, err = client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 0, 2, "")
		require.NoError(t, err)
		require.Len(t, members, 2, "should only be 2 users")

		members, _, err = client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 1, 1, "")
		require.NoError(t, err)
		require.Len(t, members, 1, "should only be 1 user")

		members, _, err = client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 1000, 100000, "")
		require.NoError(t, err)
		require.Empty(t, members, "should be 0 users")

		_, resp, err := client.GetChannelMembers(context.Background(), "junk", 0, 60, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.GetChannelMembers(context.Background(), "", 0, 60, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, _, err = client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 0, 60, "")
		require.NoError(t, err)
	})

	_, resp, err := th.Client.GetChannelMembers(context.Background(), model.NewId(), 0, 60, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = th.Client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = th.Client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 0, 60, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = th.Client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = th.Client.GetChannelMembers(context.Background(), th.BasicChannel.Id, 0, 60, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersByIds(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	cm, _, err := client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{th.BasicUser.Id})
	require.NoError(t, err)
	require.Equal(t, th.BasicUser.Id, cm[0].UserId, "returned wrong user")

	_, resp, err := client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	cm1, _, err := client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{"junk"})
	require.NoError(t, err)
	require.Empty(t, cm1, "no users should be returned")

	cm1, _, err = client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{"junk", th.BasicUser.Id})
	require.NoError(t, err)
	require.Len(t, cm1, 1, "1 member should be returned")

	cm1, _, err = client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	require.NoError(t, err)
	require.Len(t, cm1, 2, "2 members should be returned")

	_, resp, err = client.GetChannelMembersByIds(context.Background(), "junk", []string{th.BasicUser.Id})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelMembersByIds(context.Background(), model.NewId(), []string{th.BasicUser.Id})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{th.BasicUser.Id})
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, _, err = th.SystemAdminClient.GetChannelMembersByIds(context.Background(), th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	require.NoError(t, err)
}

func TestGetChannelMember(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	c := th.Client
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		member, _, err := client.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.Equal(t, th.BasicChannel.Id, member.ChannelId, "wrong channel id")
		require.Equal(t, th.BasicUser.Id, member.UserId, "wrong user id")

		_, resp, err := client.GetChannelMember(context.Background(), "", th.BasicUser.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)

		_, resp, err = client.GetChannelMember(context.Background(), "junk", th.BasicUser.Id, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, resp, err = client.GetChannelMember(context.Background(), th.BasicChannel.Id, "", "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)

		_, resp, err = client.GetChannelMember(context.Background(), th.BasicChannel.Id, "junk", "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.GetChannelMember(context.Background(), th.BasicChannel.Id, model.NewId(), "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)

		_, _, err = client.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
	})

	_, resp, err := c.GetChannelMember(context.Background(), model.NewId(), th.BasicUser.Id, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = c.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = c.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = c.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = c.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersForUser(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	members, _, err := client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Len(t, members, 6, "should have 6 members on team")

	_, resp, err := client.GetChannelMembersForUser(context.Background(), "", th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelMembersForUser(context.Background(), "junk", th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelMembersForUser(context.Background(), model.NewId(), th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, "", "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, "junk", "")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, model.NewId(), "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	_, resp, err = client.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, th.BasicTeam.Id, "")
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, _, err = th.SystemAdminClient.GetChannelMembersForUser(context.Background(), th.BasicUser.Id, th.BasicTeam.Id, "")
	require.NoError(t, err)
}

func TestViewChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	view := &model.ChannelView{
		ChannelId: th.BasicChannel.Id,
	}

	viewResp, _, err := client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)
	require.Equal(t, "OK", viewResp.Status, "should have passed")

	channel, appErr := th.App.GetChannel(th.Context, th.BasicChannel.Id)
	require.Nil(t, appErr)

	require.Equal(t, channel.LastPostAt, viewResp.LastViewedAtTimes[channel.Id], "LastPostAt does not match returned LastViewedAt time")

	view.PrevChannelId = th.BasicChannel.Id
	_, _, err = client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)

	view.PrevChannelId = ""
	_, _, err = client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)

	view.PrevChannelId = "junk"
	_, resp, err := client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// All blank is OK we use it for clicking off of the browser.
	view.PrevChannelId = ""
	view.ChannelId = ""
	_, _, err = client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)

	view.PrevChannelId = ""
	view.ChannelId = "junk"
	_, resp, err = client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	view.ChannelId = "correctlysizedjunkdddfdfdf"
	viewResult, _, err := client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)
	require.Len(t, viewResult.LastViewedAtTimes, 0)

	view.ChannelId = th.BasicChannel.Id

	member, _, err := client.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.NoError(t, err)
	channel, _, err = client.GetChannel(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	require.Equal(t, channel.TotalMsgCount, member.MsgCount, "should match message counts")
	require.Equal(t, int64(0), member.MentionCount, "should have no mentions")
	require.Equal(t, int64(0), member.MentionCountRoot, "should have no mentions")

	_, resp, err = client.ViewChannel(context.Background(), "junk", view)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.ViewChannel(context.Background(), th.BasicUser2.Id, view)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	r, err := client.DoAPIPost(context.Background(), fmt.Sprintf("/channels/members/%v/view", th.BasicUser.Id), "garbage")
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, _, err = th.SystemAdminClient.ViewChannel(context.Background(), th.BasicUser.Id, view)
	require.NoError(t, err)
}

func TestReadMultipleChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser

	t.Run("Should successfully mark public channels as read for self", func(t *testing.T) {
		channel, _, err := client.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		channel2, _, err := client.GetChannel(context.Background(), th.BasicChannel2.Id)
		require.NoError(t, err)

		channelResponse, _, err := client.ReadMultipleChannels(context.Background(), user.Id, []string{channel.Id, channel2.Id})
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.Equal(t, channel.LastPostAt, channelResponse.LastViewedAtTimes[channel.Id], "wrong number of viewed at times")
		require.Equal(t, channel2.LastPostAt, channelResponse.LastViewedAtTimes[channel2.Id], "wrong number of viewed at times")
	})

	t.Run("Should successfully mark private channels as read for self", func(t *testing.T) {
		channel, _, err := client.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
		require.NoError(t, err)

		// private channel without membership should be ignored
		channelResponse, _, err := client.ReadMultipleChannels(context.Background(), user.Id, []string{channel.Id, th.BasicPrivateChannel2.Id})
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.Equal(t, 1, len(channelResponse.LastViewedAtTimes), "unexpected response")
		require.Equal(t, channel.LastPostAt, channelResponse.LastViewedAtTimes[channel.Id], "wrong number of viewed at times")
	})

	t.Run("Should fail marking public/private channels for other user", func(t *testing.T) {
		channel, _, err := client.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)

		_, _, err = client.ReadMultipleChannels(context.Background(), th.BasicUser2.Id, []string{channel.Id})
		require.Error(t, err)
	})

	t.Run("Admin should succeed in marking public/private channels for other user", func(t *testing.T) {
		adminClient := th.SystemAdminClient
		channel, _, err := adminClient.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		privateChannel, _, err := adminClient.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
		require.NoError(t, err)

		channelResponse, _, err := adminClient.ReadMultipleChannels(context.Background(), th.BasicUser2.Id, []string{channel.Id, privateChannel.Id})
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.Equal(t, channel.LastPostAt, channelResponse.LastViewedAtTimes[channel.Id], "wrong number of viewed at times")
		require.Equal(t, privateChannel.LastPostAt, channelResponse.LastViewedAtTimes[privateChannel.Id], "wrong number of viewed at times")
	})

	t.Run("SystemManager should succeed in marking public/private channels for other user", func(t *testing.T) {
		th.LoginSystemManager(t)
		sysMgrClient := th.SystemManagerClient

		channel, _, err := sysMgrClient.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		privateChannel, _, err := sysMgrClient.GetChannel(context.Background(), th.BasicPrivateChannel.Id)
		require.NoError(t, err)

		_, _, err = sysMgrClient.ReadMultipleChannels(context.Background(), th.BasicUser2.Id, []string{channel.Id, privateChannel.Id})
		require.Error(t, err)
	})

	t.Run("SystemManager without editOtherUsers should fail in marking public/private channels for other user", func(t *testing.T) {
		sysMgrClient := th.SystemManagerClient
		th.RemovePermissionFromRole(t, model.PermissionEditOtherUsers.Id, model.SystemManagerRoleId)

		defer func() {
			th.AddPermissionToRole(t, model.PermissionEditOtherUsers.Id, model.SystemManagerRoleId)
		}()

		_, _, err := sysMgrClient.ReadMultipleChannels(context.Background(), th.BasicUser2.Id, []string{th.BasicChannel.Id})
		require.Error(t, err)
	})
}

func TestReadAllMessages(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = true
	}).InitBasic(t)
	client := th.Client
	user := th.BasicUser

	t.Run("Should fail when feature flag is disabled", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = true
		})

		_, resp, err := client.ReadAllMessages(context.Background(), user.Id)
		require.Error(t, err)
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("Should successfully mark all direct messages as read for self", func(t *testing.T) {
		dmChannel, _, err := client.CreateDirectChannel(context.Background(), user.Id, th.BasicUser2.Id)
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: dmChannel.Id,
			Message:   "test message",
		})
		require.NoError(t, err)

		channelResponse, _, err := client.ReadAllMessages(context.Background(), user.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
	})

	t.Run("Should successfully mark all group messages as read for self", func(t *testing.T) {
		gmChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user.Id, th.BasicUser2.Id, th.TeamAdminUser.Id})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: gmChannel.Id,
			Message:   "test group message",
		})
		require.NoError(t, err)

		channelResponse, _, err := client.ReadAllMessages(context.Background(), user.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
	})

	t.Run("Should fail marking messages for other user without permission", func(t *testing.T) {
		_, _, err := client.ReadAllMessages(context.Background(), th.BasicUser2.Id)
		require.Error(t, err)
	})

	t.Run("Admin should succeed in marking messages for other user", func(t *testing.T) {
		adminClient := th.SystemAdminClient

		dmChannel, _, err := adminClient.CreateDirectChannel(context.Background(), th.BasicUser2.Id, th.TeamAdminUser.Id)
		require.NoError(t, err)

		_, _, err = adminClient.CreatePost(context.Background(), &model.Post{
			ChannelId: dmChannel.Id,
			Message:   "test message for user2",
		})
		require.NoError(t, err)

		channelResponse, _, err := adminClient.ReadAllMessages(context.Background(), th.BasicUser2.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
	})

	t.Run("Should handle empty direct/group message list gracefully", func(t *testing.T) {
		channelResponse, _, err := client.ReadAllMessages(context.Background(), user.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
	})

	t.Run("Should fail with invalid user ID", func(t *testing.T) {
		_, _, err := client.ReadAllMessages(context.Background(), "invalid-user-id")
		require.Error(t, err)
	})
}

func TestReadAllInTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = true
	}).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	team := th.BasicTeam

	t.Run("Should fail when feature flag is disabled", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.EnableShiftEscapeToMarkAllRead = true
		})

		_, resp, err := client.ReadAllInTeam(context.Background(), user.Id, team.Id)

		require.Error(t, err)
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("Should successfully mark all channels and threads as read for self in team", func(t *testing.T) {
		channel, _, err := client.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		channel2, _, err := client.GetChannel(context.Background(), th.BasicChannel2.Id)
		require.NoError(t, err)

		post, _, err := client.CreatePost(context.Background(), &model.Post{
			ChannelId: channel.Id,
			Message:   "test message in channel 1",
		})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: channel2.Id,
			Message:   "test message in channel 2",
		})
		require.NoError(t, err)

		channelResponse, _, err := client.ReadAllInTeam(context.Background(), user.Id, team.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
		require.Contains(t, channelResponse.LastViewedAtTimes, channel.Id)
		require.GreaterOrEqual(t, channelResponse.LastViewedAtTimes[channel.Id], post.CreateAt,
			"channel last_viewed_at should be at or after the latest post in the channel")
	})

	t.Run("Should fail marking channels for other user without permission", func(t *testing.T) {
		_, _, err := client.ReadAllInTeam(context.Background(), th.BasicUser2.Id, team.Id)
		require.Error(t, err)
	})

	t.Run("Should fail with invalid team ID", func(t *testing.T) {
		_, _, err := client.ReadAllInTeam(context.Background(), user.Id, "invalid-team-id")
		require.Error(t, err)
	})

	t.Run("Admin should succeed in marking channels for other user in team", func(t *testing.T) {
		adminClient := th.SystemAdminClient
		channel, _, err := adminClient.GetChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)

		_, _, err = adminClient.CreatePost(context.Background(), &model.Post{
			ChannelId: channel.Id,
			Message:   "test message for user2",
		})
		require.NoError(t, err)

		channelResponse, _, err := adminClient.ReadAllInTeam(context.Background(), th.BasicUser2.Id, team.Id)
		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
	})

	t.Run("Should handle empty channel list gracefully", func(t *testing.T) {
		newTeam := th.CreateTeam(t)
		th.LinkUserToTeam(t, user, newTeam)

		channelResponse, _, err := client.ReadAllInTeam(context.Background(), user.Id, newTeam.Id)

		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.NotEmpty(t, channelResponse.LastViewedAtTimes, "should have viewed at times")
	})

	t.Run("Should only mark channels in the specified team", func(t *testing.T) {
		team2 := th.CreateTeam(t)
		th.LinkUserToTeam(t, user, team2)

		channelTeam1, _, err := client.CreateChannel(context.Background(), &model.Channel{
			TeamId:      team.Id,
			Name:        model.NewId(),
			DisplayName: "Team 1 Channel",
			Type:        model.ChannelTypeOpen,
		})
		require.NoError(t, err)

		channelTeam2, _, err := client.CreateChannel(context.Background(), &model.Channel{
			TeamId:      team2.Id,
			Name:        model.NewId(),
			DisplayName: "Team 2 Channel",
			Type:        model.ChannelTypeOpen,
		})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: channelTeam1.Id,
			Message:   "message in team 1",
		})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: channelTeam2.Id,
			Message:   "message in team 2",
		})
		require.NoError(t, err)

		channelResponse, _, err := client.ReadAllInTeam(context.Background(), user.Id, team.Id)

		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.Contains(t, channelResponse.LastViewedAtTimes, channelTeam1.Id, "team1 channel should be marked as read")
		require.NotContains(t, channelResponse.LastViewedAtTimes, channelTeam2.Id, "team2 channel should not be marked as read")
	})

	t.Run("Should handle both public and private channels in team", func(t *testing.T) {
		_, _, err := client.CreatePost(context.Background(), &model.Post{
			ChannelId: th.BasicChannel.Id,
			Message:   "public message",
		})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{
			ChannelId: th.BasicPrivateChannel.Id,
			Message:   "private message",
		})
		require.NoError(t, err)

		channelResponse, _, err := client.ReadAllInTeam(context.Background(), user.Id, team.Id)

		require.NoError(t, err)
		require.Equal(t, "OK", channelResponse.Status, "invalid status return")
		require.Contains(t, channelResponse.LastViewedAtTimes, th.BasicChannel.Id, "public channel should be marked as read")
		require.Contains(t, channelResponse.LastViewedAtTimes, th.BasicPrivateChannel.Id, "private channel should be marked as read")
	})
}

func TestGetChannelUnread(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	channelUnread, _, err := client.GetChannelUnread(context.Background(), channel.Id, user.Id)
	require.NoError(t, err)
	require.Equal(t, th.BasicTeam.Id, channelUnread.TeamId, "wrong team id returned for a regular user call")
	require.Equal(t, channel.Id, channelUnread.ChannelId, "wrong team id returned for a regular user call")

	_, resp, err := client.GetChannelUnread(context.Background(), "junk", user.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelUnread(context.Background(), channel.Id, "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelUnread(context.Background(), channel.Id, model.NewId())
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.GetChannelUnread(context.Background(), model.NewId(), user.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	newUser := th.CreateUser(t)
	_, _, err = client.Login(context.Background(), newUser.Email, newUser.Password)
	require.NoError(t, err)
	_, resp, err = client.GetChannelUnread(context.Background(), th.BasicChannel.Id, user.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	_, _, err = th.SystemAdminClient.GetChannelUnread(context.Background(), channel.Id, user.Id)
	require.NoError(t, err)

	_, resp, err = th.SystemAdminClient.GetChannelUnread(context.Background(), model.NewId(), user.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = th.SystemAdminClient.GetChannelUnread(context.Background(), channel.Id, model.NewId())
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)
}

func TestGetChannelStats(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	channel := th.CreatePrivateChannel(t)

	stats, _, err := client.GetChannelStats(context.Background(), channel.Id, "", false)
	require.NoError(t, err)

	require.Equal(t, channel.Id, stats.ChannelId, "couldn't get extra info")
	require.Equal(t, int64(1), stats.MemberCount, "got incorrect member count")
	require.Equal(t, int64(0), stats.PinnedPostCount, "got incorrect pinned post count")
	require.Equal(t, int64(0), stats.FilesCount, "got incorrect file count")

	th.CreatePinnedPostWithClient(t, th.Client, channel)
	stats, _, err = client.GetChannelStats(context.Background(), channel.Id, "", false)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.PinnedPostCount, "should have returned 1 pinned post count")

	// create a post with a file
	sent, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	fileResp, _, err := client.UploadFile(context.Background(), sent, channel.Id, "test.png")
	require.NoError(t, err)
	th.CreatePostInChannelWithFiles(t, channel, fileResp.FileInfos...)
	// make sure the file count channel stats is updated
	stats, _, err = client.GetChannelStats(context.Background(), channel.Id, "", false)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.FilesCount, "should have returned 1 file count")

	// exclude file counts
	stats, _, err = client.GetChannelStats(context.Background(), channel.Id, "", true)
	require.NoError(t, err)
	require.Equal(t, int64(-1), stats.FilesCount, "should have returned -1 file count for exclude_files_count=true")

	_, resp, err := client.GetChannelStats(context.Background(), "junk", "", false)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.GetChannelStats(context.Background(), model.NewId(), "", false)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetChannelStats(context.Background(), channel.Id, "", false)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2(t)

	_, resp, err = client.GetChannelStats(context.Background(), channel.Id, "", false)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, _, err = th.SystemAdminClient.GetChannelStats(context.Background(), channel.Id, "", false)
	require.NoError(t, err)
}

func TestGetPinnedPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	channel := th.BasicChannel

	posts, _, err := client.GetPinnedPosts(context.Background(), channel.Id, "")
	require.NoError(t, err)
	require.Empty(t, posts.Posts, "should not have gotten a pinned post")

	pinnedPost := th.CreatePinnedPost(t)
	posts, resp, err := client.GetPinnedPosts(context.Background(), channel.Id, "")
	require.NoError(t, err)
	require.Len(t, posts.Posts, 1, "should have returned 1 pinned post")
	require.Contains(t, posts.Posts, pinnedPost.Id, "missing pinned post")

	posts, resp, err = client.GetPinnedPosts(context.Background(), channel.Id, resp.Etag)
	require.NoError(t, err)
	CheckEtag(t, posts, resp)

	_, resp, err = client.GetPinnedPosts(context.Background(), GenerateTestID(), "")
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.GetPinnedPosts(context.Background(), "junk", "")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.GetPinnedPosts(context.Background(), channel.Id, "")
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, _, err = th.SystemAdminClient.GetPinnedPosts(context.Background(), channel.Id, "")
	require.NoError(t, err)
}

func TestGetPinnedPostsResultCount(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	channelWithPinnedPosts := func(t *testing.T, pinnedCount int) (*model.Channel, []*model.Post) {
		channel := th.CreatePublicChannel(t)

		createdAt := model.GetMillis()
		posts := make([]*model.Post, 0, pinnedCount)
		for i := range pinnedCount {
			posts = append(posts, &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: channel.Id,
				Message:   fmt.Sprintf("pinned %d", i),
				IsPinned:  true,
				CreateAt:  createdAt + int64(i),
			})
		}

		saved, _, nErr := th.App.Srv().Store().Post().SaveMultiple(th.Context, posts)
		require.NoError(t, nErr)

		return channel, saved
	}

	shortChannel, shortPosts := channelWithPinnedPosts(t, 5)
	longChannel, longPosts := channelWithPinnedPosts(t, web.PerPageMaximum+50)

	testCases := []struct {
		description   string
		client        *model.Client4
		channel       *model.Channel
		expectedCount int
	}{
		{
			description:   "returns every pinned post when they fit in a page",
			client:        client,
			channel:       shortChannel,
			expectedCount: 5,
		},
		{
			description:   "returns at most a page of pinned posts",
			client:        client,
			channel:       longChannel,
			expectedCount: web.PerPageMaximum,
		},
		{
			description:   "returns at most a page of pinned posts to a system administrator",
			client:        th.SystemAdminClient,
			channel:       longChannel,
			expectedCount: web.PerPageMaximum,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			posts, _, err := testCase.client.GetPinnedPosts(context.Background(), testCase.channel.Id, "")
			require.NoError(t, err)
			assert.Len(t, posts.Order, testCase.expectedCount)
		})
	}

	t.Run("keeps the most recently created pinned posts when the page is full", func(t *testing.T) {
		posts, _, err := client.GetPinnedPosts(context.Background(), longChannel.Id, "")
		require.NoError(t, err)
		require.Len(t, posts.Order, web.PerPageMaximum)

		oldestKept := longPosts[len(longPosts)-web.PerPageMaximum]
		assert.Contains(t, posts.Posts, oldestKept.Id)
		assert.NotContains(t, posts.Posts, longPosts[0].Id)
		assert.Contains(t, posts.Posts, longPosts[len(longPosts)-1].Id)

		assert.Equal(t, oldestKept.Id, posts.Order[0])
		assert.Equal(t, longPosts[len(longPosts)-1].Id, posts.Order[len(posts.Order)-1])
		for i := 1; i < len(posts.Order); i++ {
			assert.LessOrEqual(t, posts.Posts[posts.Order[i-1]].CreateAt, posts.Posts[posts.Order[i]].CreateAt)
		}
	})

	t.Run("keeps ascending order when every pinned post fits in a page", func(t *testing.T) {
		posts, _, err := client.GetPinnedPosts(context.Background(), shortChannel.Id, "")
		require.NoError(t, err)
		require.Len(t, posts.Order, len(shortPosts))
		assert.Equal(t, shortPosts[0].Id, posts.Order[0])
		assert.Equal(t, shortPosts[len(shortPosts)-1].Id, posts.Order[len(posts.Order)-1])
	})
}

func TestUpdateChannelRoles(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	const ChannelAdmin = "channel_user channel_admin"
	const ChannelMember = "channel_user"

	// User 1 creates a channel, making them channel admin by default.
	channel := th.CreatePublicChannel(t)

	// Adds User 2 to the channel, making them a channel member by default.
	_, appErr := th.App.AddUserToChannel(th.Context, th.BasicUser2, channel, false)
	require.Nil(t, appErr)

	// User 1 promotes User 2
	_, err := client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, ChannelAdmin)
	require.NoError(t, err)

	member, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
	require.NoError(t, err)
	require.Equal(t, ChannelAdmin, member.Roles, "roles don't match")

	// User 1 demotes User 2
	_, err = client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, ChannelMember)
	require.NoError(t, err)

	th.LoginBasic2(t)

	// User 2 cannot demote User 1
	resp, err := client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, ChannelMember)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// User 2 cannot promote self
	resp, err = client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, ChannelAdmin)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic(t)

	// User 1 demotes self
	_, err = client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, ChannelMember)
	require.NoError(t, err)

	// System Admin promotes User 1
	_, err = th.SystemAdminClient.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, ChannelAdmin)
	require.NoError(t, err)

	// System Admin demotes User 1
	_, err = th.SystemAdminClient.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, ChannelMember)
	require.NoError(t, err)

	// System Admin promotes User 1
	_, err = th.SystemAdminClient.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, ChannelAdmin)
	require.NoError(t, err)

	th.LoginBasic(t)

	resp, err = client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser.Id, "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.UpdateChannelRoles(context.Background(), channel.Id, "junk", ChannelMember)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.UpdateChannelRoles(context.Background(), "junk", th.BasicUser.Id, ChannelMember)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.UpdateChannelRoles(context.Background(), channel.Id, model.NewId(), ChannelMember)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	resp, err = client.UpdateChannelRoles(context.Background(), model.NewId(), th.BasicUser.Id, ChannelMember)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestUpdateChannelMemberRolesRejectsNonChannelScopedRoles(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	const channelAdmin = "channel_user channel_admin"
	const channelMember = "channel_user"

	channel := th.CreatePublicChannel(t)

	_, appErr := th.App.AddUserToChannel(th.Context, th.BasicUser2, channel, false)
	require.Nil(t, appErr)

	invalidRoles := []struct {
		name        string
		roles       string
		expectedErr string
	}{
		{name: "system manager with channel user", roles: channelMember + " " + model.SystemManagerRoleId},
		{name: "system user manager with channel user", roles: channelMember + " " + model.SystemUserManagerRoleId},
		{name: "system admin with channel admin", roles: channelAdmin + " " + model.SystemAdminRoleId},
		{name: "team user with channel user", roles: channelMember + " " + model.TeamUserRoleId},
		{name: "team admin with channel user", roles: channelMember + " " + model.TeamAdminRoleId},
		{name: "team post all with channel user", roles: channelMember + " " + model.TeamPostAllRoleId},
		{name: "system post all with channel user", roles: channelMember + " " + model.SystemPostAllRoleId},
		{name: "system read only admin with channel user", roles: channelMember + " " + model.SystemReadOnlyAdminRoleId},
		{name: "custom group user with channel user", roles: channelMember + " " + model.CustomGroupUserRoleId},
		{name: "channel guest with channel admin", roles: model.ChannelGuestRoleId + " " + model.ChannelAdminRoleId, expectedErr: "api.channel.update_channel_member_roles.guest_and_admin.app_error"},
	}

	for _, tc := range invalidRoles {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			memberBefore, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
			require.NoError(t, err)
			rolesBefore := memberBefore.Roles

			resp, err := client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, tc.roles)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
			if tc.expectedErr != "" {
				CheckErrorID(t, err, tc.expectedErr)
			}

			memberAfter, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
			require.NoError(t, err)
			require.Equal(t, rolesBefore, memberAfter.Roles)
		})
	}

	validRoles := []struct {
		name  string
		roles string
	}{
		{name: "channel member", roles: channelMember},
		{name: "channel admin", roles: channelAdmin},
	}

	for _, tc := range validRoles {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			_, err := client.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, tc.roles)
			require.NoError(t, err)

			member, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
			require.NoError(t, err)
			require.Equal(t, tc.roles, member.Roles)
		})
	}

	t.Run("rejects system manager assigned by system admin", func(t *testing.T) {
		memberBefore, _, err := th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		rolesBefore := memberBefore.Roles

		resp, err := th.SystemAdminClient.UpdateChannelRoles(context.Background(), channel.Id, th.BasicUser2.Id, channelMember+" "+model.SystemManagerRoleId)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		memberAfter, _, err := th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		require.Equal(t, rolesBefore, memberAfter.Roles)
	})
}

func TestUpdateChannelMemberSchemeRoles(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         th.GenerateTestEmail(),
		Nickname:      "nn_" + id,
		FirstName:     "f_" + id,
		LastName:      "l_" + id,
		Password:      model.NewTestPassword(),
		EmailVerified: true,
	}
	guest, appError := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, appError)
	_, _, appError = th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, guest.Id, "")
	th.AddUserToChannel(t, guest, th.BasicChannel)

	require.Nil(t, appError)

	SystemAdminClient := th.SystemAdminClient
	WebSocketClient := th.CreateConnectedWebSocketClient(t)

	th.LoginBasic(t)

	// cannot change the user scheme to false
	s1 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, err := SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s1)
	require.Error(t, err)

	tm1, _, err := SystemAdminClient.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.NoError(t, err)
	assert.Equal(t, false, tm1.SchemeGuest)
	assert.Equal(t, true, tm1.SchemeUser)
	assert.Equal(t, false, tm1.SchemeAdmin)

	s2 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s2)
	require.NoError(t, err)

	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.EventType() == model.WebsocketEventChannelMemberUpdated {
				require.Equal(t, model.WebsocketEventChannelMemberUpdated, event.EventType())
				waiting = false
			}
		case <-time.After(2 * time.Second):
			require.Fail(t, "Should have received event channel member websocket event and not timedout")
			waiting = false
		}
	}

	tm2, _, err := SystemAdminClient.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.NoError(t, err)
	assert.Equal(t, false, tm2.SchemeGuest)
	assert.Equal(t, true, tm2.SchemeUser)
	assert.Equal(t, false, tm2.SchemeAdmin)

	// cannot set Guest to User for single channel
	resp, err := SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, guest.Id, s2)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	s3 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s3)
	require.NoError(t, err)

	tm3, _, err := SystemAdminClient.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, "")
	require.NoError(t, err)
	assert.Equal(t, false, tm3.SchemeGuest)
	assert.Equal(t, true, tm3.SchemeUser)
	assert.Equal(t, true, tm3.SchemeAdmin)

	s4 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: true,
	}
	// cannot set user to guest for a single channel
	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s4)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	s5 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: true,
	}
	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s5)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), model.NewId(), th.BasicUser.Id, s3)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, model.NewId(), s3)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), "ASDF", th.BasicUser.Id, s3)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, "ASDF", s3)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	th.LoginBasic2(t)
	resp, err = th.Client.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, s3)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = SystemAdminClient.Logout(context.Background())
	require.NoError(t, err)
	resp, err = SystemAdminClient.UpdateChannelMemberSchemeRoles(context.Background(), th.BasicChannel.Id, th.SystemAdminUser.Id, s4)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateChannelMemberAutotranslation(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	mockAutotranslation := &einterfacesmocks.AutoTranslationInterface{}
	mockAutotranslation.On("IsFeatureAvailable").Return(true)
	mockAutotranslation.On("IsChannelEnabled", mock.Anything).Return(true, nil)
	mockAutotranslation.On("Translate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	originalAutoTranslation := th.Server.AutoTranslation
	th.Server.AutoTranslation = mockAutotranslation
	defer func() {
		th.Server.AutoTranslation = originalAutoTranslation
	}()

	channel := th.CreatePublicChannel(t)
	_, appErr := th.App.AddUserToChannel(th.Context, th.BasicUser2, channel, false)
	require.Nil(t, appErr)

	t.Run("user can disable own autotranslation", func(t *testing.T) {
		_, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser.Id, true)
		require.NoError(t, err)

		member, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.True(t, member.AutoTranslationDisabled, "autotranslation should be disabled")
	})

	t.Run("user can enable own autotranslation", func(t *testing.T) {
		_, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser.Id, false)
		require.NoError(t, err)

		member, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.False(t, member.AutoTranslationDisabled, "autotranslation should be enabled")
	})

	t.Run("user cannot update other user autotranslation without permission", func(t *testing.T) {
		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser2.Id, true)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		member, _, err := client.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		require.False(t, member.AutoTranslationDisabled, "autotranslation should remain enabled when update is forbidden")
	})

	t.Run("user with PermissionEditOtherUsers can update other user autotranslation", func(t *testing.T) {
		_, err := th.SystemAdminClient.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser2.Id, true)
		require.NoError(t, err)

		member, _, err := th.SystemAdminClient.GetChannelMember(context.Background(), channel.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		require.True(t, member.AutoTranslationDisabled, "autotranslation should be disabled")
	})

	t.Run("feature is disabled returns forbidden response", func(t *testing.T) {
		// Use a dedicated mock so IsFeatureAvailable returns false. The handler returns
		// before calling IsChannelEnabled/Translate, so we only need this expectation.
		featureDisabledMock := &einterfacesmocks.AutoTranslationInterface{}
		featureDisabledMock.On("IsFeatureAvailable").Return(false)
		th.Server.AutoTranslation = featureDisabledMock
		defer func() { th.Server.AutoTranslation = mockAutotranslation }()

		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser.Id, true)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel autotranslation is disabled returns bad request", func(t *testing.T) {
		// Use a dedicated mock so IsChannelEnabled returns false.
		channelDisabledMock := &einterfacesmocks.AutoTranslationInterface{}
		channelDisabledMock.On("IsFeatureAvailable").Return(true)
		channelDisabledMock.On("IsChannelEnabled", channel.Id).Return(false, nil)
		th.Server.AutoTranslation = channelDisabledMock
		defer func() { th.Server.AutoTranslation = mockAutotranslation }()

		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser.Id, true)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid channel id returns bad request", func(t *testing.T) {
		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), "junk", th.BasicUser.Id, true)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid user id returns bad request", func(t *testing.T) {
		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, "junk", true)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("nonexistent channel returns not found", func(t *testing.T) {
		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), model.NewId(), th.BasicUser.Id, true)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("nonexistent user returns not found", func(t *testing.T) {
		// Use SystemAdminClient so permission check passes and we get the "member not found" error from the app
		resp, err := th.SystemAdminClient.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, model.NewId(), true)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("unauthorized when not logged in", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		defer th.LoginBasic(t)

		resp, err := client.UpdateChannelMemberAutotranslation(context.Background(), channel.Id, th.BasicUser.Id, true)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestUpdateChannelNotifyProps(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	props := map[string]string{}
	props[model.DesktopNotifyProp] = model.ChannelNotifyMention
	props[model.MarkUnreadNotifyProp] = model.ChannelMarkUnreadMention

	_, err := client.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, props)
	require.NoError(t, err)

	member, appErr := th.App.GetChannelMember(th.Context, th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.Equal(t, model.ChannelNotifyMention, member.NotifyProps[model.DesktopNotifyProp], "bad update")
	require.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp], "bad update")

	resp, err := client.UpdateChannelNotifyProps(context.Background(), "junk", th.BasicUser.Id, props)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, "junk", props)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.UpdateChannelNotifyProps(context.Background(), model.NewId(), th.BasicUser.Id, props)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	resp, err = client.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, model.NewId(), props)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, map[string]string{})
	require.NoError(t, err)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	resp, err = client.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, props)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, err = th.SystemAdminClient.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, th.BasicUser.Id, props)
	require.NoError(t, err)
}

func TestAddChannelMember(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	publicChannel := th.CreatePublicChannel(t)
	privateChannel := th.CreatePrivateChannel(t)

	user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
	_, _, err := th.SystemAdminClient.AddTeamMember(context.Background(), team.Id, user3.Id)
	require.NoError(t, err)

	cm, resp, err := client.AddChannelMember(context.Background(), publicChannel.Id, user2.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	require.Equal(t, publicChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to public channel")

	cm, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user2.Id)
	require.NoError(t, err)
	require.Equal(t, privateChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to private channel")

	post := &model.Post{ChannelId: publicChannel.Id, Message: "a" + GenerateTestID() + "a"}
	rpost, _, err := client.CreatePost(context.Background(), post)
	require.NoError(t, err)

	_, err = client.RemoveUserFromChannel(context.Background(), publicChannel.Id, user.Id)
	require.NoError(t, err)
	_, resp, err = client.AddChannelMemberWithRootId(context.Background(), publicChannel.Id, user.Id, rpost.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	_, err = client.RemoveUserFromChannel(context.Background(), publicChannel.Id, user.Id)
	require.NoError(t, err)
	_, resp, err = client.AddChannelMemberWithRootId(context.Background(), publicChannel.Id, user.Id, "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.AddChannelMemberWithRootId(context.Background(), publicChannel.Id, user.Id, GenerateTestID())
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, _, err = client.AddChannelMember(context.Background(), publicChannel.Id, user.Id)
	require.NoError(t, err)

	cm, resp, err = client.AddChannelMember(context.Background(), publicChannel.Id, "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, cm, "should return nothing")

	_, resp, err = client.AddChannelMember(context.Background(), publicChannel.Id, GenerateTestID())
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.AddChannelMember(context.Background(), "junk", user2.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, resp, err = client.AddChannelMember(context.Background(), GenerateTestID(), user2.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	otherUser := th.CreateUser(t)
	otherChannel := th.CreatePublicChannel(t)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = client.Login(context.Background(), user2.Email, user2.Password)
	require.NoError(t, err)

	_, resp, err = client.AddChannelMember(context.Background(), publicChannel.Id, otherUser.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.AddChannelMember(context.Background(), privateChannel.Id, otherUser.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, resp, err = client.AddChannelMember(context.Background(), otherChannel.Id, otherUser.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, _, err = client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)

	// should fail adding user who is not a member of the team
	_, resp, err = client.AddChannelMember(context.Background(), otherChannel.Id, otherUser.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	_, err = client.DeleteChannel(context.Background(), otherChannel.Id)
	require.NoError(t, err)

	// Adding user to a deleted channel is fine
	_, resp, err = client.AddChannelMember(context.Background(), otherChannel.Id, user2.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	_, err = client.Logout(context.Background())
	require.NoError(t, err)
	_, resp, err = client.AddChannelMember(context.Background(), publicChannel.Id, user2.Id)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	_, resp, err = client.AddChannelMember(context.Background(), privateChannel.Id, user2.Id)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, err = client.AddChannelMember(context.Background(), publicChannel.Id, user2.Id)
		require.NoError(t, err)

		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user2.Id)
		require.NoError(t, err)
	})

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer func() {
		th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
	}()

	th.AddPermissionToRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelUserRoleId)

	// Check that a regular channel user can add other users.
	_, _, err = client.Login(context.Background(), user2.Username, user2.Password)
	require.NoError(t, err)
	privateChannel = th.CreatePrivateChannel(t)
	_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
	require.NoError(t, err)
	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	_, _, err = client.Login(context.Background(), user.Username, user.Password)
	require.NoError(t, err)
	_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user3.Id)
	require.NoError(t, err)
	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelAdminRoleId)
	th.RemovePermissionFromRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelUserRoleId)

	_, _, err = client.Login(context.Background(), user2.Username, user2.Password)
	require.NoError(t, err)
	privateChannel = th.CreatePrivateChannel(t)
	_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
	require.NoError(t, err)
	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	_, _, err = client.Login(context.Background(), user.Username, user.Password)
	require.NoError(t, err)
	_, resp, err = client.AddChannelMember(context.Background(), privateChannel.Id, user3.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	th.MakeUserChannelAdmin(t, user, privateChannel)
	appErr := th.App.Srv().InvalidateAllCaches()
	require.Nil(t, appErr)

	_, _, err = client.Login(context.Background(), user.Username, user.Password)
	require.NoError(t, err)
	_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user3.Id)
	require.NoError(t, err)
	_, err = client.Logout(context.Background())
	require.NoError(t, err)

	// Set a channel to group-constrained
	privateChannel.GroupConstrained = new(true)
	_, appErr = th.App.UpdateChannel(th.Context, privateChannel)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// User is not in associated groups so shouldn't be allowed
		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
		CheckErrorID(t, err, "api.channel.add_members.user_denied")
	})

	// Associate group to team
	_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    th.Group.Id,
		SyncableId: privateChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, appErr)

	// Add user to group
	_, appErr = th.App.UpsertGroupMember(th.Group.Id, user.Id)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
		require.NoError(t, err)
	})

	t.Run("requester is not a member of the team and tries to add a user to a channel where it is already a member", func(t *testing.T) {
		// Create two teams using SystemAdminClient
		t1 := th.CreateTeamWithClient(t, th.SystemAdminClient)
		t2 := th.CreateTeamWithClient(t, th.SystemAdminClient)

		// Use existing users - user will be BasicUser, user2 will be BasicUser2
		u1 := th.BasicUser
		u2 := th.BasicUser2

		// Add user1 to team1 and user2 to team2 (they're already on BasicTeam)
		th.LinkUserToTeam(t, u1, t1)
		th.LinkUserToTeam(t, u2, t2)

		// Create a public channel in team1
		pubChannel := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, t1.Id)

		// Add user1 to the public channel
		th.AddUserToChannel(t, u1, pubChannel)

		// Create client for user2
		client2 := th.CreateClient()
		_, _, err := client2.Login(context.Background(), u2.Email, u2.Password)
		require.NoError(t, err)

		// Try to add user1 to the public channel using user2's credentials
		// This should fail with 403 since user2 is not a member of the team
		_, resp, err := client2.AddChannelMember(context.Background(), pubChannel.Id, u1.Id)
		CheckForbiddenStatus(t, resp)
		require.Error(t, err)
	})

	t.Run("invalid request data", func(t *testing.T) {
		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			// correct type for user ids (string) but invalid value.
			requestBody := map[string]any{"user_ids": []string{"invalid", user2.Id}}
			requestData, err := json.Marshal(requestBody)
			require.NoError(t, err)

			res, err := client.DoAPIPost(context.Background(), "/channels/"+publicChannel.Id+"/members", string(requestData))
			if client == th.LocalClient {
				require.EqualError(t, err, "Invalid or missing user_id in request body.")
			} else {
				require.EqualError(t, err, "Invalid or missing user_id in user_ids in request body.")
			}
			require.Equal(t, http.StatusBadRequest, res.StatusCode)

			// invalid type for user ids (should be string).
			requestBody = map[string]any{"user_ids": []any{45, user2.Id}}
			requestData, err = json.Marshal(requestBody)
			require.NoError(t, err)

			res, err = client.DoAPIPost(context.Background(), "/channels/"+privateChannel.Id+"/members", string(requestData))
			if client == th.LocalClient {
				require.EqualError(t, err, "Invalid or missing user_id in request body.")
			} else {
				require.EqualError(t, err, "Invalid or missing user_id in user_ids in request body.")
			}
			require.Equal(t, http.StatusBadRequest, res.StatusCode)
		})
	})
}

func TestAddChannelMembers(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	publicChannel := th.CreatePublicChannel(t)
	privateChannel := th.CreatePrivateChannel(t)

	user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
	_, _, err := th.SystemAdminClient.AddTeamMember(context.Background(), team.Id, user3.Id)
	require.NoError(t, err)

	cm, resp, err := client.AddChannelMembers(context.Background(), publicChannel.Id, "", []string{user.Id, user2.Id, user3.Id})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	require.Equal(t, publicChannel.Id, cm[0].ChannelId, "should have returned exact channel")
	require.Equal(t, user.Id, cm[0].UserId, "should have returned exact user added to public channel")
	require.Equal(t, user2.Id, cm[1].UserId, "should have returned exact user added to public channel")
	require.Equal(t, user3.Id, cm[2].UserId, "should have returned exact user added to public channel")

	cm, _, err = client.AddChannelMembers(context.Background(), privateChannel.Id, "", []string{user.Id, user2.Id, user3.Id})
	require.NoError(t, err)
	require.Equal(t, privateChannel.Id, cm[0].ChannelId, "should have returned exact channel")
	require.Equal(t, user.Id, cm[0].UserId, "should have returned exact user added to public channel")
	require.Equal(t, user2.Id, cm[1].UserId, "should have returned exact user added to public channel")
	require.Equal(t, user3.Id, cm[2].UserId, "should have returned exact user added to public channel")
}

func TestAddChannelMemberFromThread(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
	_, _, err := th.SystemAdminClient.AddTeamMember(context.Background(), team.Id, user3.Id)
	require.NoError(t, err)

	wsClient := th.CreateConnectedWebSocketClient(t)

	publicChannel := th.CreatePublicChannel(t)

	_, resp, err := th.Client.AddChannelMember(context.Background(), publicChannel.Id, user3.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	_, resp, err = th.Client.AddChannelMember(context.Background(), publicChannel.Id, user2.Id)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	post := &model.Post{
		ChannelId: publicChannel.Id,
		Message:   "A root post",
		UserId:    user3.Id,
	}
	rpost, _, err := th.SystemAdminClient.CreatePost(context.Background(), post)
	require.NoError(t, err)

	_, _, err = th.SystemAdminClient.CreatePost(context.Background(),
		&model.Post{
			ChannelId: publicChannel.Id,
			Message:   "A reply post with mention @" + user.Username,
			UserId:    user2.Id,
			RootId:    rpost.Id,
		})
	require.NoError(t, err)

	_, _, err = th.SystemAdminClient.CreatePost(context.Background(),
		&model.Post{
			ChannelId: publicChannel.Id,
			Message:   "Another reply post with mention @" + user.Username,
			UserId:    user2.Id,
			RootId:    rpost.Id,
		})
	require.NoError(t, err)

	// Simulate adding a user to a channel from a thread
	_, _, err = th.SystemAdminClient.AddChannelMemberWithRootId(context.Background(), publicChannel.Id, user.Id, rpost.Id)
	require.NoError(t, err)

	// Thread membership and mention counts may take a moment to propagate (MM-41285).
	require.Eventually(t, func() bool {
		ut, _, getErr := th.Client.GetUserThread(context.Background(), user.Id, team.Id, rpost.Id, false)
		return getErr == nil && ut.UnreadMentions >= 2
	}, 10*time.Second, 200*time.Millisecond, "expected at least 2 unread mentions in thread")

	// Thread updates arrive as incremental deltas (0→1→2), not a single
	// 0→2 jump, so we only assert on the final state. The previous_unread_*
	// values depend on how the replies are batched and aren't meaningful
	// to pin down here.
	var caught bool
	func() {
		for {
			select {
			case ev := <-wsClient.EventChannel:
				if ev.EventType() == model.WebsocketEventThreadUpdated {
					var thread model.ThreadResponse
					data := ev.GetData()
					jsonErr := json.Unmarshal([]byte(data["thread"].(string)), &thread)
					require.NoError(t, jsonErr)
					if thread.UnreadReplies == 2 && thread.UnreadMentions == 2 {
						caught = true
						return
					}
				}
			case <-time.After(15 * time.Second):
				return
			}
		}
	}()
	require.Truef(t, caught, "User should have received %s event", model.WebsocketEventThreadUpdated)
}

func TestAddChannelMemberGuestAccessControl(t *testing.T) {
	th := Setup(t).InitBasic(t)

	// Enable guest accounts and add license
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GuestAccountsSettings.Enable = true
	})
	th.App.Srv().SetLicense(model.NewTestLicense())

	// Create a guest user
	guest, guestClient := th.CreateGuestAndClient(t)

	// Create a public channel to which the guest doesn't belong
	publicChannel := th.CreatePublicChannel(t)

	// Try to add another user to the channel using the guest's client
	// This should fail with a permission error, validating our fix
	_, resp, err := guestClient.AddChannelMember(context.Background(), publicChannel.Id, th.BasicUser2.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Also verify that using user IDs in the request body doesn't bypass the check
	_, resp, err = guestClient.AddChannelMembers(context.Background(), publicChannel.Id, "", []string{th.BasicUser2.Id})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Verify that the guest can get channel members for channels they belong to
	channelWithGuest := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, guest, channelWithGuest)

	// Guest should be able to read members of channels they belong to
	members, _, err := guestClient.GetChannelMembers(context.Background(), channelWithGuest.Id, 0, 100, "")
	require.NoError(t, err)
	require.NotEmpty(t, members)
}

func TestAddChannelMemberAddMyself(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, th.BasicTeam)
	notMemberPublicChannel1 := th.CreatePublicChannel(t)
	notMemberPublicChannel2 := th.CreatePublicChannel(t)
	notMemberPrivateChannel := th.CreatePrivateChannel(t)

	memberPublicChannel := th.CreatePublicChannel(t)
	memberPrivateChannel := th.CreatePrivateChannel(t)
	th.AddUserToChannel(t, user, memberPublicChannel)
	th.AddUserToChannel(t, user, memberPrivateChannel)

	testCases := []struct {
		Name                     string
		Channel                  *model.Channel
		WithJoinPublicPermission bool
		ExpectedError            string
	}{
		{
			"Add myself to a public channel with JoinPublicChannel permission",
			notMemberPublicChannel1,
			true,
			"",
		},
		{
			"Try to add myself to a private channel with the JoinPublicChannel permission",
			notMemberPrivateChannel,
			true,
			"api.context.permissions.app_error",
		},
		{
			"Try to add myself to a public channel without the JoinPublicChannel permission",
			notMemberPublicChannel2,
			false,
			"api.context.permissions.app_error",
		},
		{
			"Add myself a public channel where I'm already a member, not having JoinPublicChannel or ManageMembers permission",
			memberPublicChannel,
			false,
			"",
		},
		{
			"Add myself a private channel where I'm already a member, not having JoinPublicChannel or ManageMembers permission",
			memberPrivateChannel,
			false,
			"",
		},
	}
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Check the appropriate permissions are enforced.
			defaultRolePermissions := th.SaveDefaultRolePermissions(t)
			defer func() {
				th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
			}()

			if !tc.WithJoinPublicPermission {
				th.RemovePermissionFromRole(t, model.PermissionJoinPublicChannels.Id, model.TeamUserRoleId)
			}

			_, _, err := client.AddChannelMember(context.Background(), tc.Channel.Id, user.Id)
			if tc.ExpectedError == "" {
				require.NoError(t, err)
			} else {
				CheckErrorID(t, err, tc.ExpectedError)
			}
		})
	}
}

func TestRemoveChannelMember(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	user1 := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})
	bot := th.CreateBotWithSystemAdminClient(t)
	_, _, appErr := th.App.AddUserToTeam(th.Context, team.Id, bot.UserId, "")
	require.Nil(t, appErr)

	_, err := client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser2.Id)
	require.NoError(t, err)

	resp, err := client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, "junk")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, model.NewId())
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	resp, err = client.RemoveUserFromChannel(context.Background(), model.NewId(), th.BasicUser2.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	th.LoginBasic2(t)
	resp, err = client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	t.Run("success", func(t *testing.T) {
		// Setup the system administrator to listen for websocket events from the channels.
		th.LinkUserToTeam(t, th.SystemAdminUser, th.BasicTeam)
		_, appErr = th.App.AddUserToChannel(th.Context, th.SystemAdminUser, th.BasicChannel, false)
		require.Nil(t, appErr)
		_, appErr = th.App.AddUserToChannel(th.Context, th.SystemAdminUser, th.BasicChannel2, false)
		require.Nil(t, appErr)
		props := map[string]string{}
		props[model.DesktopNotifyProp] = model.ChannelNotifyAll
		_, err = th.SystemAdminClient.UpdateChannelNotifyProps(context.Background(), th.BasicChannel.Id, th.SystemAdminUser.Id, props)
		require.NoError(t, err)
		_, err = th.SystemAdminClient.UpdateChannelNotifyProps(context.Background(), th.BasicChannel2.Id, th.SystemAdminUser.Id, props)
		require.NoError(t, err)

		wsClient := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)

		// requirePost listens for websocket events and tries to find the post matching
		// the expected post's channel and message.
		requirePost := func(expectedPost *model.Post) {
			t.Helper()
			for {
				select {
				case event := <-wsClient.EventChannel:
					postData, ok := event.GetData()["post"]
					if !ok {
						continue
					}

					var post model.Post
					err = json.Unmarshal([]byte(postData.(string)), &post)
					require.NoError(t, err)
					if post.ChannelId == expectedPost.ChannelId && post.Message == expectedPost.Message {
						return
					}
				case <-time.After(5 * time.Second):
					require.FailNow(t, "failed to find expected post after 5 seconds")
					return
				}
			}
		}

		_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser2, th.BasicChannel, false)
		require.Nil(t, appErr)
		_, err2 := client.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser2.Id)
		require.NoError(t, err2)

		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s left the channel.", th.BasicUser2.Username),
			ChannelId: th.BasicChannel.Id,
		})

		_, err2 = client.RemoveUserFromChannel(context.Background(), th.BasicChannel2.Id, th.BasicUser.Id)
		require.NoError(t, err2)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel2.Id,
		})

		_, err2 = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
		require.NoError(t, err2)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel.Id,
		})
	})

	// Leave deleted channel
	th.LoginBasic(t)
	deletedChannel := th.CreatePublicChannel(t)
	_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser, deletedChannel, false)
	require.Nil(t, appErr)
	_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser2, deletedChannel, false)
	require.Nil(t, appErr)

	appErr = th.App.DeleteChannel(th.Context, deletedChannel, "")
	require.Nil(t, appErr)

	_, err = client.RemoveUserFromChannel(context.Background(), deletedChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)

	th.LoginBasic(t)
	private := th.CreatePrivateChannel(t)
	_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser2, private, false)
	require.Nil(t, appErr)

	_, err = client.RemoveUserFromChannel(context.Background(), private.Id, th.BasicUser2.Id)
	require.NoError(t, err)

	th.LoginBasic2(t)
	resp, err = client.RemoveUserFromChannel(context.Background(), private.Id, th.BasicUser.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser, private, false)
		require.Nil(t, appErr)
		_, err = client.RemoveUserFromChannel(context.Background(), private.Id, th.BasicUser.Id)
		require.NoError(t, err)
	})

	th.LoginBasic(t)
	th.UpdateUserToNonTeamAdmin(t, user1, team)
	appErr = th.App.Srv().InvalidateAllCaches()
	require.Nil(t, appErr)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer func() {
		th.RestoreDefaultRolePermissions(t, defaultRolePermissions)
	}()

	th.AddPermissionToRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelUserRoleId)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// Check that a regular channel user can remove other users.
		privateChannel := th.CreateChannelWithClient(t, client, model.ChannelTypePrivate)
		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user1.Id)
		require.NoError(t, err)
		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user2.Id)
		require.NoError(t, err)

		_, err = client.RemoveUserFromChannel(context.Background(), privateChannel.Id, user2.Id)
		require.NoError(t, err)
	})

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelAdminRoleId)
	th.RemovePermissionFromRole(t, model.PermissionManagePrivateChannelMembers.Id, model.ChannelUserRoleId)

	privateChannel := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypePrivate)
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), privateChannel.Id, user1.Id)
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), privateChannel.Id, user2.Id)
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), privateChannel.Id, bot.UserId)
	require.NoError(t, err)

	resp, err = client.RemoveUserFromChannel(context.Background(), privateChannel.Id, user2.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.MakeUserChannelAdmin(t, user1, privateChannel)
	appErr = th.App.Srv().InvalidateAllCaches()
	require.Nil(t, appErr)

	_, err = client.RemoveUserFromChannel(context.Background(), privateChannel.Id, user2.Id)
	require.NoError(t, err)

	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), privateChannel.Id, th.SystemAdminUser.Id)
	require.NoError(t, err)

	// If the channel is group-constrained the user cannot be removed
	privateChannel.GroupConstrained = new(true)
	_, appErr = th.App.UpdateChannel(th.Context, privateChannel)
	require.Nil(t, appErr)
	_, err = client.RemoveUserFromChannel(context.Background(), privateChannel.Id, user2.Id)
	CheckErrorID(t, err, "api.channel.remove_member.group_constrained.app_error")

	// If the channel is group-constrained user can remove self
	_, err = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), privateChannel.Id, th.SystemAdminUser.Id)
	require.NoError(t, err)

	// Test on preventing removal of user from a direct channel
	directChannel, _, err := client.CreateDirectChannel(context.Background(), user1.Id, user2.Id)
	require.NoError(t, err)

	// If the channel is group-constrained a user can remove a bot
	_, err = client.RemoveUserFromChannel(context.Background(), privateChannel.Id, bot.UserId)
	require.NoError(t, err)

	resp, err = client.RemoveUserFromChannel(context.Background(), directChannel.Id, user1.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = client.RemoveUserFromChannel(context.Background(), directChannel.Id, user2.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	resp, err = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), directChannel.Id, user1.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// Test on preventing removal of user from a group channel
	user3 := th.CreateUser(t)
	groupChannel, _, err := client.CreateGroupChannel(context.Background(), []string{user1.Id, user2.Id, user3.Id})
	require.NoError(t, err)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		resp, err = client.RemoveUserFromChannel(context.Background(), groupChannel.Id, user1.Id)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestAutocompleteChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	// A private channel to make sure private channels are used.
	ptown, _, err := th.Client.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.ChannelTypePrivate,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	tower, _, err := th.Client.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Tower",
		Name:        "tower",
		Type:        model.ChannelTypeOpen,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	defer func() {
		_, err = th.Client.DeleteChannel(context.Background(), ptown.Id)
		require.NoError(t, err)
		_, err = th.Client.DeleteChannel(context.Background(), tower.Id)
		require.NoError(t, err)
	}()

	for _, tc := range []struct {
		description      string
		teamId           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square", "town"},
			[]string{"off-topic", "tower"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "tower"},
		},
		{
			"Basic town square and off topic",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "tower", "town"},
			[]string{"off-topic"},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, _, err := th.Client.AutocompleteChannelsForTeam(context.Background(), tc.teamId, tc.fragment)
			require.NoError(t, err)
			names := make([]string, len(channels))
			for i, c := range channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestAutocompleteChannelsForSearch(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	th.LoginSystemAdminWithClient(t, th.SystemAdminClient)
	th.LoginBasicWithClient(t, th.Client)

	u1 := th.CreateUserWithClient(t, th.SystemAdminClient)
	defer func() {
		appErr := th.App.PermanentDeleteUser(th.Context, u1)
		require.Nil(t, appErr)
	}()
	u2 := th.CreateUserWithClient(t, th.SystemAdminClient)
	defer func() {
		appErr := th.App.PermanentDeleteUser(th.Context, u2)
		require.Nil(t, appErr)
	}()
	u3 := th.CreateUserWithClient(t, th.SystemAdminClient)
	defer func() {
		appErr := th.App.PermanentDeleteUser(th.Context, u3)
		require.Nil(t, appErr)
	}()
	u4 := th.CreateUserWithClient(t, th.SystemAdminClient)
	defer func() {
		appErr := th.App.PermanentDeleteUser(th.Context, u4)
		require.Nil(t, appErr)
	}()

	// A private channel to make sure private channels are not used
	ptown, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.ChannelTypePrivate,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	defer func() {
		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), ptown.Id)
		require.NoError(t, err)
	}()

	mypriv, _, err := th.Client.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.ChannelTypePrivate,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	defer func() {
		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), mypriv.Id)
		require.NoError(t, err)
	}()

	dc1, _, err := th.Client.CreateDirectChannel(context.Background(), th.BasicUser.Id, u1.Id)
	require.NoError(t, err)
	dc2, _, err := th.SystemAdminClient.CreateDirectChannel(context.Background(), u2.Id, u3.Id)
	require.NoError(t, err)

	gc1, _, err := th.Client.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, u2.Id, u3.Id})
	require.NoError(t, err)
	gc2, _, err := th.SystemAdminClient.CreateGroupChannel(context.Background(), []string{u2.Id, u3.Id, u4.Id})
	require.NoError(t, err)

	for _, tc := range []struct {
		description      string
		teamID           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "townpriv"},
		},
		{
			"Basic town square and townpriv",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Direct and group messages",
			th.BasicTeam.Id,
			"fakeuser",
			[]string{dc1.Name, gc1.Name},
			[]string{dc2.Name, gc2.Name},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, _, err := th.Client.AutocompleteChannelsForTeamForSearch(context.Background(), tc.teamID, tc.fragment)
			require.NoError(t, err)
			names := make([]string, len(channels))
			for i, c := range channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestAutocompleteChannelsForSearchGuestUsers(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	u1 := th.CreateUserWithClient(t, th.SystemAdminClient)
	defer func() {
		appErr := th.App.PermanentDeleteUser(th.Context, u1)
		require.Nil(t, appErr)
	}()

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guestPassword := model.NewTestPassword()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      guestPassword,
		EmailVerified: true,
	}
	guest, appErr := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, appErr)

	th.LoginSystemAdminWithClient(t, th.SystemAdminClient)

	_, _, err := th.SystemAdminClient.AddTeamMember(context.Background(), th.BasicTeam.Id, guest.Id)
	require.NoError(t, err)

	// A private channel to make sure private channels are not used
	town, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.ChannelTypeOpen,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	defer func() {
		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), town.Id)
		require.NoError(t, err)
	}()
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), town.Id, guest.Id)
	require.NoError(t, err)

	mypriv, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.ChannelTypePrivate,
		TeamId:      th.BasicTeam.Id,
	})
	require.NoError(t, err)
	defer func() {
		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), mypriv.Id)
		require.NoError(t, err)
	}()
	_, _, err = th.SystemAdminClient.AddChannelMember(context.Background(), mypriv.Id, guest.Id)
	require.NoError(t, err)

	dc1, _, err := th.SystemAdminClient.CreateDirectChannel(context.Background(), th.BasicUser.Id, guest.Id)
	require.NoError(t, err)
	dc2, _, err := th.SystemAdminClient.CreateDirectChannel(context.Background(), th.BasicUser.Id, th.BasicUser2.Id)
	require.NoError(t, err)

	gc1, _, err := th.SystemAdminClient.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, guest.Id})
	require.NoError(t, err)
	gc2, _, err := th.SystemAdminClient.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, u1.Id})
	require.NoError(t, err)

	_, _, err = th.Client.Login(context.Background(), guest.Username, guestPassword)
	require.NoError(t, err)

	for _, tc := range []struct {
		description      string
		teamID           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Should return those channel where is member",
			th.BasicTeam.Id,
			"town",
			[]string{"town", "townpriv"},
			[]string{"town-square", "off-topic"},
		},
		{
			"Should return empty if not member of the searched channels",
			th.BasicTeam.Id,
			"off-to",
			[]string{},
			[]string{"off-topic", "town-square", "town", "townpriv"},
		},
		{
			"Should return direct and group messages",
			th.BasicTeam.Id,
			"fakeuser",
			[]string{dc1.Name, gc1.Name},
			[]string{dc2.Name, gc2.Name},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, _, err := th.Client.AutocompleteChannelsForTeamForSearch(context.Background(), tc.teamID, tc.fragment)
			require.NoError(t, err)
			names := make([]string, len(channels))
			for i, c := range channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestUpdateChannelScheme(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	th.App.Srv().SetLicense(model.NewTestLicense(""))

	err := th.App.SetPhase2PermissionsMigrationStatus(true)
	require.NoError(t, err)

	team, _, err := th.SystemAdminClient.CreateTeam(context.Background(), &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteId:        "inviteid0",
		Name:            "z-z-" + model.NewId() + "a",
		Email:           "success+" + model.NewId() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	})
	require.NoError(t, err)

	channel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewId() + "a",
		Type:        model.ChannelTypeOpen,
		TeamId:      team.Id,
	})
	require.NoError(t, err)

	channelScheme, _, err := th.SystemAdminClient.CreateScheme(context.Background(), &model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SchemeScopeChannel,
	})
	require.NoError(t, err)

	teamScheme, _, err := th.SystemAdminClient.CreateScheme(context.Background(), &model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SchemeScopeTeam,
	})
	require.NoError(t, err)

	// Test the setup/base case.
	_, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), channel.Id, channelScheme.Id)
	require.NoError(t, err)

	// Test various invalid channel and scheme id combinations.
	resp, err := th.SystemAdminClient.UpdateChannelScheme(context.Background(), channel.Id, "x")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	resp, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), "x", channelScheme.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	resp, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), "x", "x")
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// Test that permissions are required.
	resp, err = th.Client.UpdateChannelScheme(context.Background(), channel.Id, channelScheme.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// Test that a license is required.
	th.App.Srv().SetLicense(nil)
	resp, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), channel.Id, channelScheme.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	th.App.Srv().SetLicense(model.NewTestLicense(""))

	// Test an invalid scheme scope.
	resp, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), channel.Id, teamScheme.Id)
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	// Test that an unauthenticated user gets rejected.
	_, err = th.SystemAdminClient.Logout(context.Background())
	require.NoError(t, err)
	resp, err = th.SystemAdminClient.UpdateChannelScheme(context.Background(), channel.Id, channelScheme.Id)
	require.Error(t, err)
	CheckUnauthorizedStatus(t, resp)
}

func TestGetChannelMembersTimezones(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, _, err := client.UpdateUser(context.Background(), user)
	require.NoError(t, err)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, _, err = th.SystemAdminClient.UpdateUser(context.Background(), user2)
	require.NoError(t, err)

	timezone, _, err := client.GetChannelMembersTimezones(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	require.Len(t, timezone, 2, "should return 2 timezones")

	// both users have same timezone
	user2.Timezone["automaticTimezone"] = "XOXO/BLABLA"
	_, _, err = th.SystemAdminClient.UpdateUser(context.Background(), user2)
	require.NoError(t, err)

	timezone, _, err = client.GetChannelMembersTimezones(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	require.Len(t, timezone, 1, "should return 1 timezone")

	// no timezone set should return empty
	user2.Timezone["automaticTimezone"] = ""
	_, _, err = th.SystemAdminClient.UpdateUser(context.Background(), user2)
	require.NoError(t, err)

	user.Timezone["manualTimezone"] = ""
	_, _, err = client.UpdateUser(context.Background(), user)
	require.NoError(t, err)

	timezone, _, err = client.GetChannelMembersTimezones(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	require.Empty(t, timezone, "should return 0 timezone")
}

func TestChannelMembersMinusGroupMembers(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	user1 := th.BasicUser
	user2 := th.BasicUser2

	channel := th.CreatePrivateChannel(t)

	_, appErr := th.App.AddChannelMember(th.Context, user1.Id, channel, app.ChannelMemberOpts{})
	require.Nil(t, appErr)
	_, appErr = th.App.AddChannelMember(th.Context, user2.Id, channel, app.ChannelMemberOpts{})
	require.Nil(t, appErr)

	channel.GroupConstrained = new(true)
	channel, appErr = th.App.UpdateChannel(th.Context, channel)
	require.Nil(t, appErr)

	group1 := th.CreateGroup(t)
	group2 := th.CreateGroup(t)

	_, appErr = th.App.UpsertGroupMember(group1.Id, user1.Id)
	require.Nil(t, appErr)
	_, appErr = th.App.UpsertGroupMember(group2.Id, user2.Id)
	require.Nil(t, appErr)

	// No permissions
	_, _, _, err := th.Client.ChannelMembersMinusGroupMembers(context.Background(), channel.Id, []string{group1.Id, group2.Id}, 0, 100, "")
	CheckErrorID(t, err, "api.context.permissions.app_error")

	testCases := map[string]struct {
		groupIDs        []string
		page            int
		perPage         int
		length          int
		count           int
		otherAssertions func([]*model.UserWithGroups)
	}{
		"All groups, expect no users removed": {
			groupIDs: []string{group1.Id, group2.Id},
			page:     0,
			perPage:  100,
			length:   0,
			count:    0,
		},
		"Some nonexistent group, page 0": {
			groupIDs: []string{model.NewId()},
			page:     0,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"Some nonexistent group, page 1": {
			groupIDs: []string{model.NewId()},
			page:     1,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"One group, expect one user removed": {
			groupIDs: []string{group1.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user2.Id)
			},
		},
		"Other group, expect other user removed": {
			groupIDs: []string{group2.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user1.Id)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uwg, count, _, err := th.SystemAdminClient.ChannelMembersMinusGroupMembers(context.Background(), channel.Id, tc.groupIDs, tc.page, tc.perPage, "")
			require.NoError(t, err)
			require.Len(t, uwg, tc.length)
			require.Equal(t, tc.count, int(count))
			if tc.otherAssertions != nil {
				tc.otherAssertions(uwg)
			}
		})
	}
}

func TestGetChannelModerations(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	channel := th.BasicChannel
	team := th.BasicTeam

	err := th.App.SetPhase2PermissionsMigrationStatus(true)
	require.NoError(t, err)

	t.Run("Errors without a license", func(t *testing.T) {
		_, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		CheckErrorID(t, err, "api.channel.get_channel_moderations.license.error")
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, _, err := th.Client.GetChannelModerations(context.Background(), channel.Id, "")
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with default roles", func(t *testing.T) {
		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in higher scoped scheme when no channel scheme present", func(t *testing.T) {
		scheme := th.SetupTeamScheme(t)
		team.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)

		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns value false and enabled true for permissions that are not present in channel scheme but present in team scheme", func(t *testing.T) {
		scheme := th.SetupChannelScheme(t)
		channel.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateChannelScheme(th.Context, channel)
		require.Nil(t, appErr)

		th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)

		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in channel & team scheme", func(t *testing.T) {
		teamScheme := th.SetupTeamScheme(t)
		team.SchemeId = &teamScheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		scheme := th.SetupChannelScheme(t)
		channel.SchemeId = &scheme.Id
		_, appErr = th.App.UpdateChannelScheme(th.Context, channel)
		require.Nil(t, appErr)

		th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)
		th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, teamScheme.DefaultChannelGuestRole)

		defer th.AddPermissionToRole(t, model.PermissionCreatePost.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(t, model.PermissionCreatePost.Id, teamScheme.DefaultChannelGuestRole)

		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns the correct value for manage_members depending on whether the channel is public or private", func(t *testing.T) {
		scheme := th.SetupTeamScheme(t)
		team.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelMembers.Id, scheme.DefaultChannelUserRole)
		defer th.AddPermissionToRole(t, model.PermissionManagePublicChannelMembers.Id, scheme.DefaultChannelUserRole)

		// public channel does not have the permission
		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Equal(t, moderation.Roles.Members.Value, false)
			}
		}

		// private channel does have the permission
		moderations, _, err = th.SystemAdminClient.GetChannelModerations(context.Background(), th.BasicPrivateChannel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Equal(t, moderation.Roles.Members.Value, true)
			}
		}
	})

	t.Run("Returns the correct value for manage_bookmarks depending on whether the channel is public or private", func(t *testing.T) {
		scheme := th.SetupTeamScheme(t)
		team.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		bookmarkPublicPermissions := []string{
			model.PermissionAddBookmarkPublicChannel.Id,
			model.PermissionEditBookmarkPublicChannel.Id,
			model.PermissionDeleteBookmarkPublicChannel.Id,
			model.PermissionOrderBookmarkPublicChannel.Id,
		}

		for _, p := range bookmarkPublicPermissions {
			th.RemovePermissionFromRole(t, p, scheme.DefaultChannelUserRole)
		}

		defer func() {
			for _, p := range bookmarkPublicPermissions {
				th.AddPermissionToRole(t, p, scheme.DefaultChannelUserRole)
			}
		}()

		// public channel does not have the permissions
		moderations, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == "manage_bookmarks" {
				require.Equal(t, moderation.Roles.Members.Value, false)
			}
		}

		// private channel does have the permissions
		moderations, _, err = th.SystemAdminClient.GetChannelModerations(context.Background(), th.BasicPrivateChannel.Id, "")
		require.NoError(t, err)
		for _, moderation := range moderations {
			if moderation.Name == "manage_bookmarks" {
				require.Equal(t, moderation.Roles.Members.Value, true)
			}
		}
	})

	t.Run("Does not return an error if the team scheme has a blank DefaultChannelGuestRole field", func(t *testing.T) {
		scheme := th.SetupTeamScheme(t)
		scheme.DefaultChannelGuestRole = ""

		// Restore the real store so helper cleanup (cache invalidation, license reload)
		// doesn't run against the partial mock.
		originalStore := th.App.Srv().Store()
		t.Cleanup(func() { th.App.Srv().SetStore(originalStore) })

		mockStore := mocks.Store{}

		// Playbooks DB job requires a plugin mock
		pluginStore := mocks.PluginStore{}
		pluginStore.On("List", mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
		pluginStore.On("ListWithOptions", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
		mockStore.On("Plugin").Return(&pluginStore)

		mockSchemeStore := mocks.SchemeStore{}
		mockSchemeStore.On("Get", mock.Anything).Return(scheme, nil)
		mockStore.On("Scheme").Return(&mockSchemeStore)
		mockStore.On("Team").Return(th.App.Srv().Store().Team())
		mockStore.On("Channel").Return(th.App.Srv().Store().Channel())
		mockStore.On("User").Return(th.App.Srv().Store().User())
		mockStore.On("Post").Return(th.App.Srv().Store().Post())
		mockStore.On("FileInfo").Return(th.App.Srv().Store().FileInfo())
		mockStore.On("Webhook").Return(th.App.Srv().Store().Webhook())
		mockStore.On("DeliveryTracking").Return(th.App.Srv().Store().DeliveryTracking())
		mockStore.On("System").Return(th.App.Srv().Store().System())
		mockStore.On("License").Return(th.App.Srv().Store().License())
		mockStore.On("Role").Return(th.App.Srv().Store().Role())
		mockStore.On("Close").Return(nil)
		th.App.Srv().SetStore(&mockStore)

		team.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		_, _, err := th.SystemAdminClient.GetChannelModerations(context.Background(), channel.Id, "")
		require.NoError(t, err)
	})
}

func TestPatchChannelDisableJoinLeaveMessages(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	t.Run("patch only disable_join_leave_messages succeeds", func(t *testing.T) {
		disableJoinLeave := true
		patch := &model.ChannelPatch{
			DisableJoinLeaveMessages: &disableJoinLeave,
		}

		channel, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.True(t, channel.DisableJoinLeaveMessages)
	})

	t.Run("patch can re-enable join/leave messages", func(t *testing.T) {
		disableJoinLeave := false
		patch := &model.ChannelPatch{
			DisableJoinLeaveMessages: &disableJoinLeave,
		}

		channel, resp, err := client.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.False(t, channel.DisableJoinLeaveMessages)
	})

	t.Run("cannot patch on direct channel", func(t *testing.T) {
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)

		disableJoinLeave := true
		patch := &model.ChannelPatch{
			DisableJoinLeaveMessages: &disableJoinLeave,
		}

		_, resp, err := client.PatchChannel(context.Background(), dm.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("cannot patch on group channel", func(t *testing.T) {
		user3 := th.CreateUser(t)
		gm, _, gmErr := th.SystemAdminClient.CreateGroupChannel(context.Background(), []string{th.BasicUser.Id, th.BasicUser2.Id, user3.Id})
		require.NoError(t, gmErr)

		disableJoinLeave := true
		patch := &model.ChannelPatch{
			DisableJoinLeaveMessages: &disableJoinLeave,
		}

		_, resp, err := client.PatchChannel(context.Background(), gm.Id, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("non-member cannot patch", func(t *testing.T) {
		// manage_public_channel_properties is in the default channel_user role,
		// so any channel member can change this setting. Test with a non-member.
		nonMember := th.CreateUser(t)
		nonMemberClient := th.CreateClient()
		_, _, loginErr := nonMemberClient.Login(context.Background(), nonMember.Email, nonMember.Password)
		require.NoError(t, loginErr)

		disableJoinLeave := true
		patch := &model.ChannelPatch{
			DisableJoinLeaveMessages: &disableJoinLeave,
		}

		_, resp, err := nonMemberClient.PatchChannel(context.Background(), th.BasicChannel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestPatchChannelModerations(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	channel := th.BasicChannel

	emptyPatch := []*model.ChannelModerationPatch{}

	createPosts := model.ChannelModeratedPermissions[0]

	err := th.App.SetPhase2PermissionsMigrationStatus(true)
	require.NoError(t, err)

	t.Run("Errors without a license", func(t *testing.T) {
		_, _, err := th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, emptyPatch)
		CheckErrorID(t, err, "api.channel.patch_channel_moderations.license.error")
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, _, err := th.Client.PatchChannelModerations(context.Background(), channel.Id, emptyPatch)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with empty patch", func(t *testing.T) {
		moderations, _, err := th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, emptyPatch)
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		require.Nil(t, channel.SchemeId)
	})

	t.Run("Creates a scheme and returns the updated channel moderations when patching an existing permission", func(t *testing.T) {
		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: new(false)},
			},
		}

		moderations, _, err := th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			if moderation.Name == createPosts {
				require.Equal(t, moderation.Roles.Members.Value, false)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			} else {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			}
		}
		var appErr *model.AppError
		channel, appErr = th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.NotNil(t, channel.SchemeId)
	})

	t.Run("Removes the existing scheme when moderated permissions are set back to higher scoped values", func(t *testing.T) {
		var appErr *model.AppError
		channel, appErr = th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		schemeId := channel.SchemeId

		scheme, appErr := th.App.GetScheme(*schemeId)
		require.Nil(t, appErr)
		require.Equal(t, scheme.DeleteAt, int64(0))

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: new(true)},
			},
		}

		moderations, _, err := th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		channel, appErr = th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.Nil(t, channel.SchemeId)

		scheme, appErr = th.App.GetScheme(*schemeId)
		require.Nil(t, appErr)
		require.NotEqual(t, scheme.DeleteAt, int64(0))
	})

	t.Run("Does not return an error if the team scheme has a blank DefaultChannelGuestRole field", func(t *testing.T) {
		team := th.BasicTeam
		scheme := th.SetupTeamScheme(t)
		scheme.DefaultChannelGuestRole = ""

		// Restore the real store so helper cleanup (cache invalidation, license reload)
		// doesn't run against the partial mock.
		originalStore := th.App.Srv().Store()
		t.Cleanup(func() { th.App.Srv().SetStore(originalStore) })

		mockStore := mocks.Store{}

		// Playbooks DB job requires a plugin mock
		pluginStore := mocks.PluginStore{}
		pluginStore.On("List", mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
		pluginStore.On("ListWithOptions", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
		mockStore.On("Plugin").Return(&pluginStore)

		mockSchemeStore := mocks.SchemeStore{}
		mockSchemeStore.On("Get", mock.Anything).Return(scheme, nil)
		mockSchemeStore.On("Save", mock.Anything).Return(scheme, nil)
		mockSchemeStore.On("Delete", mock.Anything).Return(scheme, nil)
		mockStore.On("Scheme").Return(&mockSchemeStore)
		mockStore.On("Team").Return(th.App.Srv().Store().Team())
		mockStore.On("Channel").Return(th.App.Srv().Store().Channel())
		mockStore.On("User").Return(th.App.Srv().Store().User())
		mockStore.On("Post").Return(th.App.Srv().Store().Post())
		mockStore.On("FileInfo").Return(th.App.Srv().Store().FileInfo())
		mockStore.On("Webhook").Return(th.App.Srv().Store().Webhook())
		mockStore.On("DeliveryTracking").Return(th.App.Srv().Store().DeliveryTracking())
		mockStore.On("System").Return(th.App.Srv().Store().System())
		mockStore.On("License").Return(th.App.Srv().Store().License())
		mockStore.On("Role").Return(th.App.Srv().Store().Role())
		mockStore.On("Close").Return(nil)
		th.App.Srv().SetStore(&mockStore)

		team.SchemeId = &scheme.Id
		_, appErr := th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		moderations, _, err := th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, emptyPatch)
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: new(true)},
			},
		}

		moderations, _, err = th.SystemAdminClient.PatchChannelModerations(context.Background(), channel.Id, patch)
		require.NoError(t, err)
		require.Equal(t, len(moderations), 5)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" || moderation.Name == "manage_bookmarks" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}
	})
}

func TestGetChannelMemberCountsByGroup(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	channel := th.BasicChannel
	t.Run("Errors without a license", func(t *testing.T) {
		_, _, err := th.SystemAdminClient.GetChannelMemberCountsByGroup(context.Background(), channel.Id, false, "")
		CheckErrorID(t, err, "api.channel.channel_member_counts_by_group.license.error")
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors without read permission to the channel", func(t *testing.T) {
		_, _, err := th.Client.GetChannelMemberCountsByGroup(context.Background(), model.NewId(), false, "")
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("Returns empty for a channel with no members or groups", func(t *testing.T) {
		memberCounts, _, err := th.SystemAdminClient.GetChannelMemberCountsByGroup(context.Background(), channel.Id, false, "")
		require.NoError(t, err)
		require.Equal(t, []*model.ChannelMemberCountByGroup{}, memberCounts)
	})

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, appErr := th.App.UpsertGroupMember(th.Group.Id, user.Id)
	require.Nil(t, appErr)
	_, _, err := th.SystemAdminClient.UpdateUser(context.Background(), user)
	require.NoError(t, err)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, appErr = th.App.UpsertGroupMember(th.Group.Id, user2.Id)
	require.Nil(t, appErr)
	_, _, err = th.SystemAdminClient.UpdateUser(context.Background(), user2)
	require.NoError(t, err)

	t.Run("Returns users in group without timezones", func(t *testing.T) {
		memberCounts, _, err := th.SystemAdminClient.GetChannelMemberCountsByGroup(context.Background(), channel.Id, false, "")
		require.NoError(t, err)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	t.Run("Returns users in group with timezones", func(t *testing.T) {
		memberCounts, _, err := th.SystemAdminClient.GetChannelMemberCountsByGroup(context.Background(), channel.Id, true, "")
		require.NoError(t, err)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 2,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	id := model.NewId()
	group := &model.Group{
		DisplayName: "dn_" + id,
		Name:        new("name" + id),
		Source:      model.GroupSourceLdap,
		RemoteId:    new(model.NewId()),
	}

	_, appErr = th.App.CreateGroup(group)
	require.Nil(t, appErr)
	_, appErr = th.App.UpsertGroupMember(group.Id, user.Id)
	require.Nil(t, appErr)

	t.Run("Returns multiple groups with users in group with timezones", func(t *testing.T) {
		memberCounts, _, err := th.SystemAdminClient.GetChannelMemberCountsByGroup(context.Background(), channel.Id, true, "")
		require.NoError(t, err)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     group.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 1,
			},
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 2,
			},
		}
		require.ElementsMatch(t, expectedMemberCounts, memberCounts)
	})
}

func TestGetChannelsMemberCount(t *testing.T) {
	mainHelper.Parallel(t)
	// Setup
	th := Setup(t).InitBasic(t)

	client := th.Client

	channel1 := th.CreatePublicChannel(t)
	channel2 := th.CreatePrivateChannel(t)
	channel3 := th.CreatePrivateChannel(t)
	th.RemoveUserFromChannel(t, th.BasicUser, channel3)

	user1 := th.CreateUser(t)
	user2 := th.CreateUser(t)
	user3 := th.CreateUser(t)

	th.LinkUserToTeam(t, user1, th.BasicTeam)
	th.LinkUserToTeam(t, user2, th.BasicTeam)
	th.LinkUserToTeam(t, user3, th.BasicTeam)

	th.AddUserToChannel(t, user1, channel1)
	th.AddUserToChannel(t, user2, channel1)
	th.AddUserToChannel(t, user3, channel1)
	th.AddUserToChannel(t, user2, channel2)

	t.Run("Should return correct member count", func(t *testing.T) {
		// Create a request with channel IDs
		channelIDs := []string{channel1.Id, channel2.Id}
		channelsMemberCount, _, err := client.GetChannelsMemberCount(context.Background(), channelIDs)
		require.NoError(t, err)

		// Verify the member counts
		require.Contains(t, channelsMemberCount, channel1.Id)
		require.Contains(t, channelsMemberCount, channel2.Id)
		require.Equal(t, int64(4), channelsMemberCount[channel1.Id])
		require.Equal(t, int64(2), channelsMemberCount[channel2.Id])
	})

	t.Run("Should return empty object when empty array is passed", func(t *testing.T) {
		channelsMemberCount, _, err := client.GetChannelsMemberCount(context.Background(), []string{})
		require.NoError(t, err)
		require.Equal(t, 0, len(channelsMemberCount))
	})

	t.Run("Should fail due to permissions", func(t *testing.T) {
		_, resp, err := client.GetChannelsMemberCount(context.Background(), []string{channel3.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("Should fail due to expired session when logged out", func(t *testing.T) {
		_, err := client.Logout(context.Background())
		require.NoError(t, err)
		channelIDs := []string{channel1.Id, channel2.Id}
		_, resp, err := client.GetChannelsMemberCount(context.Background(), channelIDs)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
		CheckErrorID(t, err, "api.context.session_expired.app_error")
	})

	t.Run("Should fail due to expired session when logged out", func(t *testing.T) {
		th.LoginBasic2(t)
		channelIDs := []string{channel1.Id, channel2.Id}
		_, resp, err := client.GetChannelsMemberCount(context.Background(), channelIDs)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("Should not fail for public channels that the user is not a member of", func(t *testing.T) {
		th.LoginBasic2(t)
		channelIDs := []string{channel1.Id}
		_, _, err := client.GetChannelsMemberCount(context.Background(), channelIDs)
		require.NoError(t, err)
	})

	t.Run("Should fail for private channels that the user is not a member of", func(t *testing.T) {
		th.LoginBasic2(t)
		channelIDs := []string{channel2.Id}
		_, _, err := client.GetChannelsMemberCount(context.Background(), channelIDs)
		require.Error(t, err)
	})
}

func TestMoveChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	client := th.Client
	team1 := th.BasicTeam
	team2 := th.CreateTeam(t)

	t.Run("Should move channel", func(t *testing.T) {
		publicChannel := th.CreatePublicChannel(t)
		ch, _, err := th.SystemAdminClient.MoveChannel(context.Background(), publicChannel.Id, team2.Id, false)
		require.NoError(t, err)
		require.Equal(t, team2.Id, ch.TeamId)
	})

	t.Run("Should return custom error with repeated channel", func(t *testing.T) {
		channelT1 := &model.Channel{
			DisplayName: "repeated",
			Name:        "repeated",
			Type:        model.ChannelTypePrivate,
			TeamId:      team1.Id,
		}
		channelT1, _, err := th.Client.CreateChannel(context.Background(), channelT1)
		require.NoError(t, err)

		channelT2 := &model.Channel{
			DisplayName: "repeated",
			Name:        "repeated",
			Type:        model.ChannelTypePrivate,
			TeamId:      team2.Id,
		}
		_, _, err = th.Client.CreateChannel(context.Background(), channelT2)
		require.NoError(t, err)

		_, _, err = th.SystemAdminClient.MoveChannel(context.Background(), channelT1.Id, team2.Id, false)
		require.EqualError(t, err, "A channel with that name already exists on the same team.")
	})

	t.Run("Should move private channel", func(t *testing.T) {
		channel := th.CreatePrivateChannel(t)
		ch, _, err := th.SystemAdminClient.MoveChannel(context.Background(), channel.Id, team1.Id, false)
		require.NoError(t, err)
		require.Equal(t, team1.Id, ch.TeamId)
	})

	t.Run("Should fail when trying to move a DM channel", func(t *testing.T) {
		user := th.CreateUser(t)
		dmChannel := th.CreateDmChannel(t, user)
		_, _, err := client.MoveChannel(context.Background(), dmChannel.Id, team1.Id, false)
		require.Error(t, err)
		CheckErrorID(t, err, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail when trying to move a group channel", func(t *testing.T) {
		user := th.CreateUser(t)

		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.SystemAdminUser.Id, th.TeamAdminUser.Id}, user.Id)
		require.Nil(t, appErr)
		_, _, err := client.MoveChannel(context.Background(), gmChannel.Id, team1.Id, false)
		require.Error(t, err)
		CheckErrorID(t, err, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail due to permissions", func(t *testing.T) {
		publicChannel := th.CreatePublicChannel(t)
		_, _, err := client.MoveChannel(context.Background(), publicChannel.Id, team1.Id, false)
		require.Error(t, err)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel(t)
		user := th.BasicUser

		_, err := client.RemoveTeamMember(context.Background(), team2.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.AddChannelMember(context.Background(), publicChannel.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.MoveChannel(context.Background(), publicChannel.Id, team2.Id, false)
		require.Error(t, err)
		CheckErrorID(t, err, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move public channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel(t)
		user := th.BasicUser

		_, err := client.RemoveTeamMember(context.Background(), team2.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.MoveChannel(context.Background(), privateChannel.Id, team2.Id, false)
		require.Error(t, err)
		CheckErrorID(t, err, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move private channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel(t)
		user := th.BasicUser

		_, err := client.RemoveTeamMember(context.Background(), team2.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.AddChannelMember(context.Background(), publicChannel.Id, user.Id)
		require.NoError(t, err)

		newChannel, _, err := client.MoveChannel(context.Background(), publicChannel.Id, team2.Id, true)
		require.NoError(t, err)
		require.Equal(t, team2.Id, newChannel.TeamId)
	}, "Should be able to (force) move public channel by a member that is not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel(t)
		user := th.BasicUser

		_, err := client.RemoveTeamMember(context.Background(), team2.Id, user.Id)
		require.NoError(t, err)

		_, _, err = client.AddChannelMember(context.Background(), privateChannel.Id, user.Id)
		require.NoError(t, err)

		newChannel, _, err := client.MoveChannel(context.Background(), privateChannel.Id, team2.Id, true)
		require.NoError(t, err)
		require.Equal(t, team2.Id, newChannel.TeamId)
	}, "Should be able to (force) move private channel by a member that is not member of target team")
}

func TestRootMentionsCount(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	// initially, MentionCountRoot is 0 in the database
	channelMember, err := th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(0), channelMember.MentionCountRoot)
	require.Equal(t, int64(0), channelMember.MentionCount)

	// mention the user in a root post
	post1, _, err := th.SystemAdminClient.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "hey @" + user.Username})
	require.NoError(t, err)
	// mention the user in a reply post
	post2 := &model.Post{ChannelId: channel.Id, Message: "reply at @" + user.Username, RootId: post1.Id}
	_, _, err = th.SystemAdminClient.CreatePost(context.Background(), post2)
	require.NoError(t, err)

	// this should perform lazy migration and populate the field
	channelUnread, _, err := client.GetChannelUnread(context.Background(), channel.Id, user.Id)
	require.NoError(t, err)
	// reply post is not counted, so we should have one root mention
	require.EqualValues(t, int64(1), channelUnread.MentionCountRoot)
	// regular count stays the same
	require.Equal(t, int64(2), channelUnread.MentionCount)
	// validate that DB is updated
	channelMember, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
	require.NoError(t, err)
	require.EqualValues(t, int64(1), channelMember.MentionCountRoot)

	// validate that Team level counts are calculated
	counts, appErr := th.App.GetTeamUnread(channel.TeamId, user.Id)
	require.Nil(t, appErr)
	require.Equal(t, int64(1), counts.MentionCountRoot)
	require.Equal(t, int64(2), counts.MentionCount)
}

func TestViewChannelWithoutCollapsedThreads(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	client := th.Client
	user := th.BasicUser
	team := th.BasicTeam
	channel := th.BasicChannel

	// mention the user in a root post
	post1, _, err := th.SystemAdminClient.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "hey @" + user.Username})
	require.NoError(t, err)
	// mention the user in a reply post
	post2 := &model.Post{ChannelId: channel.Id, Message: "reply at @" + user.Username, RootId: post1.Id}
	_, _, err = th.SystemAdminClient.CreatePost(context.Background(), post2)
	require.NoError(t, err)

	threads, _, err := client.GetUserThreads(context.Background(), user.Id, team.Id, model.GetUserThreadsOpts{})
	require.NoError(t, err)
	require.EqualValues(t, int64(1), threads.TotalUnreadMentions)

	// simulate opening the channel from an old client
	_, _, err = client.ViewChannel(context.Background(), user.Id, &model.ChannelView{
		ChannelId:                 channel.Id,
		PrevChannelId:             "",
		CollapsedThreadsSupported: false,
	})
	require.NoError(t, err)

	threads, _, err = client.GetUserThreads(context.Background(), user.Id, team.Id, model.GetUserThreadsOpts{})
	require.NoError(t, err)
	require.Zero(t, threads.TotalUnreadMentions)
}

func TestChannelMemberSanitization(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	channel := th.CreatePublicChannel(t)

	// Add second user to channel
	_, _, err := client.AddChannelMember(context.Background(), channel.Id, user2.Id)
	require.NoError(t, err)

	// Give the current user a real, non-zero last_viewed_at: user2 posts a
	// message so the channel is unread for the current user, who then views it.
	// This keeps the current-user assertions realistic; the requester's own
	// timestamps are never sanitized, so a genuine 0 would still be serialized.
	user2Client := th.CreateClient()
	_, _, err = user2Client.Login(context.Background(), user2.Email, user2.Password)
	require.NoError(t, err)
	_, _, err = user2Client.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "unread message"})
	require.NoError(t, err)

	_, _, err = client.ViewChannel(context.Background(), user.Id, &model.ChannelView{ChannelId: channel.Id})
	require.NoError(t, err)

	// decodeRawMembers reads the raw JSON body of a channel member response so the
	// test can assert whether the timestamp fields are present or omitted, which a
	// typed model.ChannelMember cannot distinguish from a zero value.
	decodeRawMembers := func(resp *http.Response, err error) []map[string]json.RawMessage {
		require.NoError(t, err)
		defer resp.Body.Close()

		var raw json.RawMessage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

		var members []map[string]json.RawMessage
		if decodeErr := json.Unmarshal(raw, &members); decodeErr != nil {
			var single map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &single))
			members = []map[string]json.RawMessage{single}
		}
		return members
	}

	// decodeNDJSONMembers reads a newline-delimited JSON stream, as returned by
	// getChannelMembersForUser when page=-1.
	decodeNDJSONMembers := func(resp *http.Response, err error) []map[string]json.RawMessage {
		require.NoError(t, err)
		defer resp.Body.Close()

		var members []map[string]json.RawMessage
		decoder := json.NewDecoder(resp.Body)
		for {
			var member map[string]json.RawMessage
			decodeErr := decoder.Decode(&member)
			if decodeErr == io.EOF {
				break
			}
			require.NoError(t, decodeErr)
			members = append(members, member)
		}
		return members
	}

	userIDOf := func(t *testing.T, member map[string]json.RawMessage) string {
		t.Helper()
		var id string
		require.NoError(t, json.Unmarshal(member["user_id"], &id))
		return id
	}

	// assertTimestamps verifies that the current user's memberships expose valid
	// timestamps while other users' timestamps are omitted entirely.
	assertTimestamps := func(t *testing.T, members []map[string]json.RawMessage) {
		t.Helper()
		for _, member := range members {
			rawLastViewedAt, hasLastViewedAt := member["last_viewed_at"]
			rawLastUpdateAt, hasLastUpdateAt := member["last_update_at"]

			if userIDOf(t, member) == user.Id {
				require.True(t, hasLastViewedAt, "Current user should see their last_viewed_at")
				require.True(t, hasLastUpdateAt, "Current user should see their last_update_at")

				var lastViewedAt, lastUpdateAt int64
				require.NoError(t, json.Unmarshal(rawLastViewedAt, &lastViewedAt))
				require.NoError(t, json.Unmarshal(rawLastUpdateAt, &lastUpdateAt))
				assert.GreaterOrEqual(t, lastViewedAt, int64(0), "Current user's last_viewed_at should be a valid timestamp, not the sentinel")
				assert.GreaterOrEqual(t, lastUpdateAt, int64(0), "Current user's last_update_at should be a valid timestamp, not the sentinel")
			} else {
				assert.False(t, hasLastViewedAt, "Other users' last_viewed_at should be omitted, not returned as an invalid value")
				assert.False(t, hasLastUpdateAt, "Other users' last_update_at should be omitted, not returned as an invalid value")
			}
		}
	}

	t.Run("getChannelMembers omits last_viewed_at and last_update_at for other users", func(t *testing.T) {
		members := decodeRawMembers(client.DoAPIGet(context.Background(), "/channels/"+channel.Id+"/members?page=0&per_page=60", ""))
		require.Len(t, members, 2)
		assertTimestamps(t, members)
	})

	t.Run("getChannelMember omits timestamps for other users but keeps them for the current user", func(t *testing.T) {
		otherMembers := decodeRawMembers(client.DoAPIGet(context.Background(), "/channels/"+channel.Id+"/members/"+user2.Id, ""))
		require.Len(t, otherMembers, 1)
		_, hasLastViewedAt := otherMembers[0]["last_viewed_at"]
		_, hasLastUpdateAt := otherMembers[0]["last_update_at"]
		assert.False(t, hasLastViewedAt, "Other user's last_viewed_at should be omitted")
		assert.False(t, hasLastUpdateAt, "Other user's last_update_at should be omitted")

		currentMembers := decodeRawMembers(client.DoAPIGet(context.Background(), "/channels/"+channel.Id+"/members/"+user.Id, ""))
		require.Len(t, currentMembers, 1)
		assertTimestamps(t, currentMembers)
	})

	t.Run("getChannelMembersByIds omits timestamps for other users", func(t *testing.T) {
		members := decodeRawMembers(client.DoAPIPostJSON(context.Background(), "/channels/"+channel.Id+"/members/ids", []string{user.Id, user2.Id}))
		require.Len(t, members, 2)
		assertTimestamps(t, members)
	})

	assertOtherUserMembersOmitted := func(t *testing.T, members []map[string]json.RawMessage, expectTeamData bool) {
		t.Helper()
		require.NotEmpty(t, members)
		for _, member := range members {
			assert.Equal(t, user2.Id, userIDOf(t, member))
			assert.NotContains(t, member, "last_viewed_at", "Other user's last_viewed_at should be omitted")
			assert.NotContains(t, member, "last_update_at", "Other user's last_update_at should be omitted")
			if expectTeamData {
				assert.Contains(t, member, "team_name", "Team data should still be present")
			}
		}
	}

	t.Run("getChannelMembersForUser (paginated) omits timestamps for other users", func(t *testing.T) {
		// Querying another user's channel members requires the edit_other_users
		// permission, so use the system admin client.
		members := decodeRawMembers(th.SystemAdminClient.DoAPIGet(context.Background(), "/users/"+user2.Id+"/channel_members?page=0", ""))
		assertOtherUserMembersOmitted(t, members, true)
	})

	t.Run("getChannelMembersForUser (NDJSON stream) omits timestamps for other users", func(t *testing.T) {
		// page=-1 switches the endpoint to the newline-delimited streaming path,
		// which sanitizes members through a separate code path.
		members := decodeNDJSONMembers(th.SystemAdminClient.DoAPIGet(context.Background(), "/users/"+user2.Id+"/channel_members?page=-1", ""))
		assertOtherUserMembersOmitted(t, members, true)
	})

	t.Run("getChannelMembersForTeamForUser omits timestamps for other users", func(t *testing.T) {
		// Querying another user's memberships requires manage_system, so use the
		// system admin client.
		members := decodeRawMembers(th.SystemAdminClient.DoAPIGet(context.Background(), "/users/"+user2.Id+"/teams/"+th.BasicTeam.Id+"/channels/members", ""))
		assertOtherUserMembersOmitted(t, members, false)
	})

	t.Run("addChannelMember omits timestamps in the returned member data", func(t *testing.T) {
		newUser := th.CreateUser(t)
		th.LinkUserToTeam(t, newUser, th.BasicTeam)

		members := decodeRawMembers(client.DoAPIPostJSON(context.Background(), "/channels/"+channel.Id+"/members", map[string]string{"user_id": newUser.Id}))
		require.Len(t, members, 1)

		assert.NotContains(t, members[0], "last_viewed_at", "Returned member last_viewed_at should be omitted")
		assert.NotContains(t, members[0], "last_update_at", "Returned member last_update_at should be omitted")
		assert.Equal(t, newUser.Id, userIDOf(t, members[0]), "UserId should be preserved")

		var channelID string
		require.NoError(t, json.Unmarshal(members[0]["channel_id"], &channelID))
		assert.Equal(t, channel.Id, channelID, "ChannelId should be preserved")
	})
}

func TestChannelEndpointsRejectBoards(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupBoardTest(t)
	client := th.Client
	ctx := context.Background()

	boardChannel, _, err := client.CreateBoard(ctx, &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Test Board",
		Name:        "board-" + model.NewId(),
		Type:        model.ChannelTypeOpenBoard,
	})
	require.NoError(t, err)
	require.NotNil(t, boardChannel)

	// --- WRITE operations ---

	t.Run("createChannel rejects board type", func(t *testing.T) {
		_, resp, err := client.CreateChannel(ctx, &model.Channel{
			DisplayName: "New Board",
			Name:        "board-new-" + model.NewId(),
			Type:        model.ChannelTypeOpenBoard,
			TeamId:      th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannel not found for board", func(t *testing.T) {
		_, resp, err := client.UpdateChannel(ctx, boardChannel)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("patchChannel not found for board", func(t *testing.T) {
		newName := "patched"
		_, resp, err := client.PatchChannel(ctx, boardChannel.Id, &model.ChannelPatch{
			DisplayName: &newName,
		})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("deleteChannel not found for board", func(t *testing.T) {
		resp, err := client.DeleteChannel(ctx, boardChannel.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("updateChannelPrivacy not found for board", func(t *testing.T) {
		_, resp, err := client.UpdateChannelPrivacy(ctx, boardChannel.Id, model.ChannelTypePrivate)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("restoreChannel not found for board", func(t *testing.T) {
		_, resp, err := client.RestoreChannel(ctx, boardChannel.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("moveChannel not found for board", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.MoveChannel(ctx, boardChannel.Id, th.BasicTeam.Id, false)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("addChannelMember not found for board", func(t *testing.T) {
		_, resp, err := client.AddChannelMember(ctx, boardChannel.Id, th.BasicUser2.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("removeChannelMember not found for board", func(t *testing.T) {
		resp, err := client.RemoveUserFromChannel(ctx, boardChannel.Id, th.BasicUser.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("updateChannelMemberRoles rejects board", func(t *testing.T) {
		resp, err := client.UpdateChannelRoles(ctx, boardChannel.Id, th.BasicUser.Id, model.ChannelUserRoleId)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannelScheme rejects board", func(t *testing.T) {
		resp, err := th.SystemAdminClient.UpdateChannelScheme(ctx, boardChannel.Id, model.NewId())
		require.Error(t, err)
		CheckForbiddenStatus(t, resp) // license check fires before channel fetch
	})

	t.Run("updateChannelMemberNotifyProps rejects board", func(t *testing.T) {
		resp, err := client.UpdateChannelNotifyProps(ctx, boardChannel.Id, th.BasicUser.Id, map[string]string{model.DesktopNotifyProp: model.UserNotifyAll})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannelMemberAutotranslation rejects board", func(t *testing.T) {
		resp, err := client.UpdateChannelMemberAutotranslation(ctx, boardChannel.Id, th.BasicUser.Id, true)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp) // feature-availability check fires before channel fetch
	})

	t.Run("viewChannel rejects board", func(t *testing.T) {
		_, resp, err := client.ViewChannel(ctx, th.BasicUser.Id, &model.ChannelView{ChannelId: boardChannel.Id})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	// --- READ operations: boards are invisible (404) via store-level filter ---

	t.Run("getChannel not found for board", func(t *testing.T) {
		_, resp, err := client.GetChannel(ctx, boardChannel.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("getChannelStats not found for board", func(t *testing.T) {
		_, resp, err := client.GetChannelStats(ctx, boardChannel.Id, "", false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getChannelMembers not found for board", func(t *testing.T) {
		_, resp, err := client.GetChannelMembers(ctx, boardChannel.Id, 0, 60, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getPinnedPosts not found for board", func(t *testing.T) {
		_, resp, err := client.GetPinnedPosts(ctx, boardChannel.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})
}

func TestChannelEndpointsExcludeBoards(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupBoardTest(t)
	client := th.Client

	openBoard, _, err := th.Client.CreateBoard(context.Background(), &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Open Board",
		Name:        "open-board-" + model.NewId(),
		Type:        model.ChannelTypeOpenBoard,
	})
	require.NoError(t, err)
	require.NotNil(t, openBoard)

	privateBoard, _, err := th.Client.CreateBoard(context.Background(), &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Private Board",
		Name:        "private-board-" + model.NewId(),
		Type:        model.ChannelTypePrivateBoard,
	})
	require.NoError(t, err)
	require.NotNil(t, privateBoard)

	// Helper to check board IDs are not in a channel list
	assertNoBoardsInList := func(t *testing.T, channels []*model.Channel) {
		t.Helper()
		for _, ch := range channels {
			assert.False(t, ch.IsBoard(), "board channel %s (type %s) should not appear in channel list", ch.Id, ch.Type)
		}
	}

	ctx := context.Background()

	t.Run("getPublicChannelsForTeam excludes boards", func(t *testing.T) {
		channels, _, err := client.GetPublicChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("getDeletedChannelsForTeam excludes boards", func(t *testing.T) {
		// Soft-delete the open board so it appears in deleted list
		nErr := th.App.Srv().Store().Channel().Delete(openBoard.Id, model.GetMillis())
		require.NoError(t, nErr)

		channels, _, err := th.SystemAdminClient.GetDeletedChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("getPrivateChannelsForTeam excludes boards", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.GetPrivateChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("searchChannelsForTeam excludes boards", func(t *testing.T) {
		channels, _, err := client.SearchChannels(ctx, th.BasicTeam.Id, &model.ChannelSearch{Term: "board"})
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("autocompleteChannelsForTeam excludes boards", func(t *testing.T) {
		channels, _, err := client.AutocompleteChannelsForTeam(ctx, th.BasicTeam.Id, "board")
		require.NoError(t, err)
		assertNoBoardsInList(t, []*model.Channel(channels))
	})

	t.Run("searchAllChannels excludes boards", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.SearchAllChannels(ctx, &model.ChannelSearch{Term: "board"})
		require.NoError(t, err)
		for _, ch := range channels {
			assert.False(t, ch.IsBoard(), "board channel %s (type %s) should not appear in searchAllChannels results", ch.Id, ch.Type)
		}
	})

	t.Run("getAllChannels excludes boards", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.GetAllChannels(ctx, 0, 100, "")
		require.NoError(t, err)
		for _, ch := range channels {
			assert.False(t, ch.IsBoard(), "board channel %s (type %s) should not appear in getAllChannels results", ch.Id, ch.Type)
		}
	})

	t.Run("getChannelsForTeamForUser excludes boards", func(t *testing.T) {
		channels, _, err := client.GetChannelsForTeamForUser(ctx, th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("getChannelsForUser excludes boards", func(t *testing.T) {
		channels, _, err := client.GetChannelsForUserWithLastDeleteAt(ctx, th.BasicUser.Id, 0)
		require.NoError(t, err)
		assertNoBoardsInList(t, channels)
	})

	t.Run("getChannelsMemberCount excludes boards", func(t *testing.T) {
		counts, _, err := client.GetChannelsMemberCount(ctx, []string{th.BasicChannel.Id, openBoard.Id, privateBoard.Id})
		require.NoError(t, err)
		_, hasRegular := counts[th.BasicChannel.Id]
		assert.True(t, hasRegular, "regular channel should be in member count results")
		_, hasOpenBoard := counts[openBoard.Id]
		assert.False(t, hasOpenBoard, "open board should not be in member count results")
		_, hasPrivateBoard := counts[privateBoard.Id]
		assert.False(t, hasPrivateBoard, "private board should not be in member count results")
	})

	t.Run("getChannelByName 404s for board", func(t *testing.T) {
		_, resp, err := client.GetChannelByName(ctx, privateBoard.Name, th.BasicTeam.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("getChannelByNameIncludeDeleted 404s for board", func(t *testing.T) {
		_, resp, err := client.GetChannelByNameIncludeDeleted(ctx, privateBoard.Name, th.BasicTeam.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("getChannelByNameForTeamName 404s for board", func(t *testing.T) {
		_, resp, err := client.GetChannelByNameForTeamName(ctx, privateBoard.Name, th.BasicTeam.Name, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("searchAllChannelsForUser excludes boards", func(t *testing.T) {
		channels, _, err := client.SearchAllChannelsForUser(ctx, "board")
		require.NoError(t, err)
		for _, ch := range channels {
			assert.False(t, ch.IsBoard(), "board channel %s (type %s) should not appear in searchAllChannelsForUser results", ch.Id, ch.Type)
		}
	})
}

// createTestSpaceChannel creates a ChannelTypeSpace ("S") backing channel directly through the
// store, since spaces have no dedicated REST creation endpoint in this slice.
func createTestSpaceChannel(t *testing.T, th *TestHelper) *model.Channel {
	t.Helper()
	space := &model.Channel{
		TeamId:      th.BasicTeam.Id,
		DisplayName: "Space",
		Name:        "space-" + model.NewId(),
		Type:        model.ChannelTypeSpace,
	}
	space, err := th.App.Srv().Store().Channel().Save(th.Context, space, -1)
	require.NoError(t, err)
	return space
}

// addUserToSpaceChannel makes a user a member of a space backing channel. Membership is written
// directly through the store, since this slice has no app/REST path for adding members to a
// space, and the per-user authorization cache is invalidated so SessionHasPermissionToChannel
// sees the new membership.
func addUserToSpaceChannel(t *testing.T, th *TestHelper, space *model.Channel, userID string) {
	t.Helper()
	_, err := th.App.Srv().Store().Channel().SaveMember(th.Context, &model.ChannelMember{
		ChannelId:   space.Id,
		UserId:      userID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeUser:  true,
	})
	require.NoError(t, err)
	th.App.Srv().Store().Channel().InvalidateAllChannelMembersForUser(userID)
}

// TestChannelEndpointsExcludeSpaces mirrors TestChannelEndpointsExcludeBoards for the space ("S")
// backing-channel type: it 404s on every /channels endpoint and never leaks into any list, search,
// or by-name surface, except where rejectSpaceChannelByID guards a member-mutation endpoint or
// view-state marking no-ops through the store-level type filter.
func TestChannelEndpointsExcludeSpaces(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client
	ctx := context.Background()

	space := createTestSpaceChannel(t, th)

	// BasicUser is a member of the space backing channel (e.g. a page collaborator).
	addUserToSpaceChannel(t, th, space, th.BasicUser.Id)

	assertNoSpacesInList := func(t *testing.T, channels []*model.Channel) {
		t.Helper()
		for _, ch := range channels {
			assert.NotEqual(t, model.ChannelTypeSpace, ch.Type, "space channel %s should not appear in channel list", ch.Id)
		}
	}

	// --- Space backing channels never leak into list/search surfaces ---

	t.Run("getPublicChannelsForTeam excludes spaces", func(t *testing.T) {
		channels, _, err := client.GetPublicChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("getPrivateChannelsForTeam excludes spaces", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.GetPrivateChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("getDeletedChannelsForTeam excludes spaces", func(t *testing.T) {
		deletedSpace := createTestSpaceChannel(t, th)
		nErr := th.App.Srv().Store().Channel().Delete(deletedSpace.Id, model.GetMillis())
		require.NoError(t, nErr)

		channels, _, err := th.SystemAdminClient.GetDeletedChannelsForTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("searchChannels excludes spaces", func(t *testing.T) {
		channels, _, err := client.SearchChannels(ctx, th.BasicTeam.Id, &model.ChannelSearch{Term: "space"})
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("autocompleteChannelsForTeam excludes spaces", func(t *testing.T) {
		channels, _, err := client.AutocompleteChannelsForTeam(ctx, th.BasicTeam.Id, "space")
		require.NoError(t, err)
		assertNoSpacesInList(t, []*model.Channel(channels))
	})

	t.Run("searchAllChannels excludes spaces", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.SearchAllChannels(ctx, &model.ChannelSearch{Term: "space"})
		require.NoError(t, err)
		for _, ch := range channels {
			assert.NotEqual(t, model.ChannelTypeSpace, ch.Type, "space channel %s should not appear in searchAllChannels results", ch.Id)
		}
	})

	t.Run("getAllChannels excludes spaces", func(t *testing.T) {
		channels, _, err := th.SystemAdminClient.GetAllChannels(ctx, 0, 100, "")
		require.NoError(t, err)
		for _, ch := range channels {
			assert.NotEqual(t, model.ChannelTypeSpace, ch.Type, "space channel %s should not appear in getAllChannels results", ch.Id)
		}
	})

	t.Run("getChannelsForTeamForUser excludes spaces", func(t *testing.T) {
		channels, _, err := client.GetChannelsForTeamForUser(ctx, th.BasicTeam.Id, th.BasicUser.Id, false, "")
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("getChannelsForUser excludes spaces even for a member", func(t *testing.T) {
		channels, _, err := client.GetChannelsForUserWithLastDeleteAt(ctx, th.BasicUser.Id, 0)
		require.NoError(t, err)
		assertNoSpacesInList(t, channels)
	})

	t.Run("getChannelsMemberCount excludes spaces", func(t *testing.T) {
		counts, _, err := client.GetChannelsMemberCount(ctx, []string{th.BasicChannel.Id, space.Id})
		require.NoError(t, err)
		_, hasRegular := counts[th.BasicChannel.Id]
		assert.True(t, hasRegular, "regular channel should be in member count results")
		_, hasSpace := counts[space.Id]
		assert.False(t, hasSpace, "space backing channel should not be in member count results")
	})

	t.Run("getChannelByName 404s for space", func(t *testing.T) {
		_, resp, err := client.GetChannelByName(ctx, space.Name, th.BasicTeam.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("getChannelByNameForTeamName 404s for space", func(t *testing.T) {
		_, resp, err := client.GetChannelByNameForTeamName(ctx, space.Name, th.BasicTeam.Name, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("searchAllChannelsForUser excludes spaces", func(t *testing.T) {
		channels, _, err := client.SearchAllChannelsForUser(ctx, "space")
		require.NoError(t, err)
		for _, ch := range channels {
			assert.NotEqual(t, model.ChannelTypeSpace, ch.Type, "space channel %s should not appear in searchAllChannelsForUser results", ch.Id)
		}
	})

	// --- Like boards, a space backing channel is excluded from the generic getChannel and
	// --- 404s for everyone; it resolves only through the dedicated space APIs. ---

	t.Run("getChannel 404s a space for a member", func(t *testing.T) {
		_, resp, err := client.GetChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("getChannel 404s a space for system admin", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	// --- Generic destructive/conversion endpoints 404 (Get-resolving) or reject (member-mutation) spaces ---

	t.Run("createChannel rejects a space", func(t *testing.T) {
		_, resp, err := client.CreateChannel(ctx, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			DisplayName: "Space",
			Name:        "space-" + model.NewId(),
			Type:        model.ChannelTypeSpace,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("deleteChannel 404s a space", func(t *testing.T) {
		resp, err := th.SystemAdminClient.DeleteChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("updateChannelPrivacy 404s a space", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.UpdateChannelPrivacy(ctx, space.Id, model.ChannelTypeOpen)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("restoreChannel 404s a space", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.RestoreChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("updateChannelScheme rejects a space", func(t *testing.T) {
		originalLicense := th.App.Srv().License()
		th.App.Srv().SetLicense(model.NewTestLicense(""))
		defer th.App.Srv().SetLicense(originalLicense)

		err := th.App.SetPhase2PermissionsMigrationStatus(true)
		require.NoError(t, err)

		channelScheme, _, err := th.SystemAdminClient.CreateScheme(ctx, &model.Scheme{
			DisplayName: "DisplayName",
			Name:        model.NewId(),
			Description: "Some description",
			Scope:       model.SchemeScopeChannel,
		})
		require.NoError(t, err)

		resp, err := th.SystemAdminClient.UpdateChannelScheme(ctx, space.Id, channelScheme.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	// --- Generic member/view-state mutation endpoints reject or no-op spaces (managed by the spaces feature) ---

	t.Run("viewChannel silently no-ops a space", func(t *testing.T) {
		// Spaces carry no chat read-state: GetChannelsWithUnreadsAndWithMentions filters
		// them out, so viewing one succeeds without marking anything viewed — the same
		// behavior as a nonexistent channel ID.
		viewResp, resp, err := client.ViewChannel(ctx, th.BasicUser.Id, &model.ChannelView{ChannelId: space.Id})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotContains(t, viewResp.LastViewedAtTimes, space.Id)
	})

	t.Run("updateChannelMemberRoles rejects a space", func(t *testing.T) {
		resp, err := th.SystemAdminClient.UpdateChannelRoles(ctx, space.Id, th.BasicUser.Id, "channel_user")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannelMemberSchemeRoles rejects a space", func(t *testing.T) {
		resp, err := th.SystemAdminClient.UpdateChannelMemberSchemeRoles(ctx, space.Id, th.BasicUser.Id, &model.SchemeRoles{SchemeUser: true})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannelMemberNotifyProps rejects a space", func(t *testing.T) {
		resp, err := client.UpdateChannelNotifyProps(ctx, space.Id, th.BasicUser.Id, map[string]string{model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("updateChannelMemberAutotranslation rejects a space", func(t *testing.T) {
		mockAutotranslation := &einterfacesmocks.AutoTranslationInterface{}
		mockAutotranslation.On("IsFeatureAvailable").Return(true)
		originalAutoTranslation := th.Server.AutoTranslation
		th.Server.AutoTranslation = mockAutotranslation
		defer func() {
			th.Server.AutoTranslation = originalAutoTranslation
		}()

		resp, err := client.UpdateChannelMemberAutotranslation(ctx, space.Id, th.BasicUser.Id, true)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("addChannelMember 404s a space", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.AddChannelMember(ctx, space.Id, th.BasicUser2.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("setChannelMembers 404s a space", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, space.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser2.Id}}, 0, 0)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("moveChannel 404s a space", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.MoveChannel(ctx, space.Id, th.BasicTeam.Id, false)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	// --- Generic read endpoints that resolve the channel via GetChannel return 403 for spaces,
	// --- because GetChannel returns not-found for space channels, which SessionHasPermissionToChannel
	// --- treats as no permission. No explicit space guard is needed here. ---

	t.Run("getChannelStats rejects a space", func(t *testing.T) {
		_, resp, err := client.GetChannelStats(ctx, space.Id, "", false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getChannelMembers rejects a space", func(t *testing.T) {
		_, resp, err := client.GetChannelMembers(ctx, space.Id, 0, 100, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getChannelMembersTimezones rejects a space", func(t *testing.T) {
		_, resp, err := client.GetChannelMembersTimezones(ctx, space.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getChannelMembersByIds rejects a space", func(t *testing.T) {
		_, resp, err := client.GetChannelMembersByIds(ctx, space.Id, []string{th.BasicUser.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("getChannelMember rejects a space", func(t *testing.T) {
		_, resp, err := client.GetChannelMember(ctx, space.Id, th.BasicUser.Id, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channelMemberCountsByGroup rejects a space", func(t *testing.T) {
		originalLicense := th.App.Srv().License()
		th.App.Srv().SetLicense(model.NewTestLicense())
		defer th.App.Srv().SetLicense(originalLicense)

		_, resp, err := client.GetChannelMemberCountsByGroup(ctx, space.Id, false, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channelMembersMinusGroupMembers rejects a space", func(t *testing.T) {
		_, _, resp, err := th.SystemAdminClient.ChannelMembersMinusGroupMembers(ctx, space.Id, []string{model.NewId()}, 0, 100, "")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("getChannelAccessControlAttributes rejects a space", func(t *testing.T) {
		r, err := client.DoAPIGet(ctx, "/channels/"+space.Id+"/access_control/attributes", "")
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	t.Run("convertGroupMessageToChannel rejects a space", func(t *testing.T) {
		resp, err := client.DoAPIPost(ctx, "/channels/"+space.Id+"/convert_to_channel", "{}")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// --- rejectSpaceChannelByID passes through a not-found result (a nonexistent id is definitely
	// --- not a space), so the endpoint's own not-found/permission handling surfaces normally ---

	t.Run("updateChannelMemberRoles on a nonexistent channel behaves as before", func(t *testing.T) {
		resp, err := th.SystemAdminClient.UpdateChannelRoles(ctx, model.NewId(), th.BasicUser.Id, "channel_user")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

// TestLocalChannelEndpoints404Spaces verifies the local (admin socket) channel endpoints 404 a
// space, the same as /api/v4: space channels are excluded from the generic GetChannel these
// handlers resolve through, so they never reach the mutation.
func TestLocalChannelEndpoints404Spaces(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	space := createTestSpaceChannel(t, th)

	t.Run("local deleteChannel 404s a space", func(t *testing.T) {
		resp, err := th.LocalClient.DeleteChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("local restoreChannel 404s a space", func(t *testing.T) {
		_, resp, err := th.LocalClient.RestoreChannel(ctx, space.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("local addChannelMember 404s a space", func(t *testing.T) {
		_, resp, err := th.LocalClient.AddChannelMember(ctx, space.Id, th.BasicUser2.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})
}

func TestSetChannelMembers(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	t.Run("basic reconcile", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		user2 := th.BasicUser2

		// Add user2 to channel first
		_, _, err := th.SystemAdminClient.AddChannelMember(ctx, channel.Id, user2.Id)
		require.NoError(t, err)

		// Create a new user on the team
		user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
		th.LinkUserToTeam(t, user3, th.BasicTeam)

		// Set members to [BasicUser, user3] — user2 should be removed, user3 added
		results, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id, user3.Id}}, 0, 0)
		require.NoError(t, err)
		assert.Equal(t, "application/x-ndjson", resp.Header.Get("Content-Type"))

		// Collect all results
		var allAdded, allRemoved []string
		var allErrors []model.SetChannelMembersError
		for _, r := range results {
			allAdded = append(allAdded, r.Added...)
			allRemoved = append(allRemoved, r.Removed...)
			allErrors = append(allErrors, r.Errors...)
		}
		assert.Contains(t, allRemoved, user2.Id)
		assert.Contains(t, allAdded, user3.Id)
		assert.NotContains(t, allAdded, th.BasicUser.Id, "existing member should not be re-added")
		assert.NotContains(t, allRemoved, th.BasicUser.Id, "existing member should not be removed")
		assert.Empty(t, allErrors, "should have no errors")

		// Verify final channel membership matches what was requested
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, channel.Id, 0, 100, "")
		require.NoError(t, err)
		memberIDs := make(map[string]bool)
		for _, m := range members {
			memberIDs[m.UserId] = true
		}
		assert.Len(t, members, 2, "channel should have exactly 2 members")
		assert.True(t, memberIDs[th.BasicUser.Id], "BasicUser should be a member")
		assert.True(t, memberIDs[user3.Id], "user3 should be a member")
		assert.False(t, memberIDs[user2.Id], "user2 should not be a member")
	})

	t.Run("idempotent", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		user2 := th.BasicUser2

		// First call performs a real mutation — add user2
		results1, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id, user2.Id}}, 0, 0)
		require.NoError(t, err)
		var firstAdded []string
		for _, r := range results1 {
			firstAdded = append(firstAdded, r.Added...)
		}
		assert.Contains(t, firstAdded, user2.Id, "first call should add user2")

		// Second call with same list — should be all no-ops
		results2, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id, user2.Id}}, 0, 0)
		require.NoError(t, err)
		for _, r := range results2 {
			assert.Empty(t, r.Added)
			assert.Empty(t, r.Removed)
			assert.Empty(t, r.Errors)
		}

		// Verify membership unchanged
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, channel.Id, 0, 100, "")
		require.NoError(t, err)
		assert.Len(t, members, 2)
	})

	t.Run("empty list removes all removable members", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// Set to empty list
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{}}, 0, 0)
		require.NoError(t, err)

		var allRemoved []string
		for _, r := range results {
			allRemoved = append(allRemoved, r.Removed...)
		}
		assert.Contains(t, allRemoved, th.BasicUser.Id)

		// Verify channel has no members
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, channel.Id, 0, 100, "")
		require.NoError(t, err)
		assert.Empty(t, members)
	})

	t.Run("empty private channel rejected", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel(t)
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, privateChannel.Id, &model.SetChannelMembersRequest{Members: []string{}}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		// Verify members are unchanged
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, privateChannel.Id, 0, 100, "")
		require.NoError(t, err)
		assert.NotEmpty(t, members, "private channel should still have members")
	})

	t.Run("private channel can be orphaned when all desired users are invalid", func(t *testing.T) {
		// Known limitation: if the desired list is non-empty but all users are
		// invalid (e.g. not on the team), removals succeed first and then all
		// adds fail, leaving the private channel empty. The sysadmin can recover
		// by adding members back via the API.
		privateChannel := th.CreatePrivateChannel(t)

		// Create a user NOT on the team
		otherUser := th.CreateUserWithClient(t, th.SystemAdminClient)

		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, privateChannel.Id, &model.SetChannelMembersRequest{Members: []string{otherUser.Id}}, 0, 0)
		require.NoError(t, err)

		// The add should have failed
		var allErrors []model.SetChannelMembersError
		for _, r := range results {
			allErrors = append(allErrors, r.Errors...)
		}
		assert.NotEmpty(t, allErrors, "invalid user should appear in errors")

		// The channel is now empty — this is the known limitation
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, privateChannel.Id, 0, 100, "")
		require.NoError(t, err)
		assert.Empty(t, members, "private channel is orphaned (known limitation)")
	})

	t.Run("non-sysadmin returns 403", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		_, resp, err := th.Client.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 0)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("DM channel rejected", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, dmChannel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("GM channel rejected", func(t *testing.T) {
		user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
		th.LinkUserToTeam(t, user3, th.BasicTeam)
		gmChannel, _, err := th.SystemAdminClient.CreateGroupChannel(ctx, []string{th.BasicUser.Id, th.BasicUser2.Id, user3.Id})
		require.NoError(t, err)

		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, gmChannel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("group-constrained channel rejected", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		channel.GroupConstrained = new(true)
		_, appErr := th.App.UpdateChannel(th.Context, channel)
		require.Nil(t, appErr)

		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("policy-enforced channel rejected", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// The gate rejects bulk membership edits only when the channel's
		// policy actually governs membership. We declare the `membership`
		// action here so PolicyActions[membership]=true is hydrated on
		// subsequent reads — a non-membership action (e.g. `view`) would
		// no longer trigger this path after the Phase 2 migration, which
		// is intentional and covered by the permission-only test below.
		policy := &model.AccessControlPolicy{
			Type:    model.AccessControlPolicyTypeChannel,
			ID:      channel.Id,
			Name:    "Test Policy",
			Version: model.AccessControlPolicyVersionV0_2,
			Rules: []model.AccessControlPolicyRule{
				{
					Actions:    []string{model.AccessControlPolicyActionMembership},
					Expression: "user.attributes.team == \"test\"",
				},
			},
		}
		_, storeErr := th.App.Srv().Store().AccessControlPolicy().Save(th.Context, policy)
		require.NoError(t, storeErr)
		t.Cleanup(func() {
			_ = th.App.Srv().Store().AccessControlPolicy().Delete(th.Context, channel.Id)
		})
		th.App.Srv().Store().Channel().InvalidateChannel(channel.Id)

		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("town-square non-guests cannot be removed", func(t *testing.T) {
		// Find town-square
		townSquare, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, th.BasicTeam.Id, false)
		require.Nil(t, appErr)

		// Set to empty list, non-guest members cannot be removed and should appear in errors
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, townSquare.Id, &model.SetChannelMembersRequest{Members: []string{}}, 0, 0)
		require.NoError(t, err)

		var allErrors []model.SetChannelMembersError
		for _, r := range results {
			allErrors = append(allErrors, r.Errors...)
		}
		assert.NotEmpty(t, allErrors, "non-guest members should appear in errors")

		// Verify members are still in town-square (removal was blocked)
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, townSquare.Id, 0, 100, "")
		require.NoError(t, err)
		assert.NotEmpty(t, members, "town-square should still have members")
	})

	t.Run("user not on team appears in errors", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// Create a user NOT on the team
		otherUser := th.CreateUserWithClient(t, th.SystemAdminClient)

		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id, otherUser.Id}}, 0, 0)
		require.NoError(t, err)

		var allErrors []model.SetChannelMembersError
		for _, r := range results {
			allErrors = append(allErrors, r.Errors...)
		}
		found := false
		for _, e := range allErrors {
			if e.UserID == otherUser.Id {
				found = true
				break
			}
		}
		assert.True(t, found, "user not on team should appear in errors")
	})

	t.Run("null members rejected", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		// nil Members field serializes as JSON null for the members array
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{}, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid batch_size rejected", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// batch_size out of range (too large)
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 9999, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid batch_delay_ms rejected", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// batch_delay_ms out of range (above max 10000)
		_, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: []string{th.BasicUser.Id}}, 0, 99999)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("batching with large list", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// Create 5 users and add them to the team
		var userIDs []string
		userIDs = append(userIDs, th.BasicUser.Id)
		for range 5 {
			u := th.CreateUserWithClient(t, th.SystemAdminClient)
			th.LinkUserToTeam(t, u, th.BasicTeam)
			userIDs = append(userIDs, u.Id)
		}

		// Use batch_size=2 to force multiple batches
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{Members: userIDs}, 2, 0)
		require.NoError(t, err)

		// BasicUser is already a member, so 5 to add in batches of 2 = 3 batches
		assert.Len(t, results, 3, "should have exactly 3 batches for 5 adds with batch_size=2")

		var allAdded []string
		for _, r := range results {
			assert.LessOrEqual(t, len(r.Added), 2, "each batch should have at most 2 additions")
			allAdded = append(allAdded, r.Added...)
		}
		assert.Len(t, allAdded, 5, "should have added 5 users total")

		// Verify final membership matches the requested list
		members, _, err := th.SystemAdminClient.GetChannelMembers(ctx, channel.Id, 0, 100, "")
		require.NoError(t, err)
		memberIDs := make(map[string]bool)
		for _, m := range members {
			memberIDs[m.UserId] = true
		}
		for _, id := range userIDs {
			assert.True(t, memberIDs[id], "user %s should be a member", id)
		}
		assert.Len(t, members, len(userIDs), "channel should have exactly the requested members")
	})

	t.Run("channel_admins sets admin roles", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		user2 := th.BasicUser2
		user3 := th.CreateUserWithClient(t, th.SystemAdminClient)
		th.LinkUserToTeam(t, user3, th.BasicTeam)

		// BasicUser is already channel admin (as creator). Promote user2 instead.
		adminIDs := []string{th.BasicUser.Id, user2.Id}
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id, user2.Id, user3.Id},
				ChannelAdmins: &adminIDs,
			}, 0, 0)
		require.NoError(t, err)

		var allPromoted []string
		for _, r := range results {
			allPromoted = append(allPromoted, r.Promoted...)
		}
		assert.Contains(t, allPromoted, user2.Id, "user2 should be promoted to admin")

		// Verify SchemeAdmin is set for both admins
		member, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		assert.True(t, member.SchemeAdmin, "BasicUser should be channel admin")

		member2, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user2.Id, "")
		require.NoError(t, err)
		assert.True(t, member2.SchemeAdmin, "user2 should be channel admin")

		// user3 should not be admin
		member3, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user3.Id, "")
		require.NoError(t, err)
		assert.False(t, member3.SchemeAdmin, "user3 should not be channel admin")
	})

	t.Run("channel_admins omitted preserves existing admins", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// First, set BasicUser as admin
		adminIDs := []string{th.BasicUser.Id}
		_, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id},
				ChannelAdmins: &adminIDs,
			}, 0, 0)
		require.NoError(t, err)

		// Verify admin is set
		member, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.True(t, member.SchemeAdmin)

		// Now call again without channel_admins (nil). Admin role should be preserved.
		user2 := th.BasicUser2
		_, _, err = th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members: []string{th.BasicUser.Id, user2.Id},
			}, 0, 0)
		require.NoError(t, err)

		member, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		assert.True(t, member.SchemeAdmin, "admin role should be preserved when channel_admins is omitted")
	})

	t.Run("channel_admins demotes existing admins", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		user2 := th.BasicUser2

		// Set both users as members with BasicUser as admin
		adminIDs := []string{th.BasicUser.Id}
		_, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id, user2.Id},
				ChannelAdmins: &adminIDs,
			}, 0, 0)
		require.NoError(t, err)

		// Verify BasicUser is admin
		member, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.True(t, member.SchemeAdmin)

		// Now set user2 as admin instead, demoting BasicUser
		newAdminIDs := []string{user2.Id}
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id, user2.Id},
				ChannelAdmins: &newAdminIDs,
			}, 0, 0)
		require.NoError(t, err)

		var allPromoted, allDemoted []string
		for _, r := range results {
			allPromoted = append(allPromoted, r.Promoted...)
			allDemoted = append(allDemoted, r.Demoted...)
		}
		assert.Contains(t, allPromoted, user2.Id, "user2 should be promoted")
		assert.Contains(t, allDemoted, th.BasicUser.Id, "BasicUser should be demoted")

		member, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		assert.False(t, member.SchemeAdmin, "BasicUser should no longer be admin")

		member2, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user2.Id, "")
		require.NoError(t, err)
		assert.True(t, member2.SchemeAdmin, "user2 should be admin")
	})

	t.Run("channel_admins adds user only in admins list as member and admin", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		user2 := th.BasicUser2

		// user2 is only in channel_admins, not in members. Should be added and made admin.
		adminIDs := []string{user2.Id}
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id},
				ChannelAdmins: &adminIDs,
			}, 0, 0)
		require.NoError(t, err)

		var allAdded, allPromoted []string
		for _, r := range results {
			allAdded = append(allAdded, r.Added...)
			allPromoted = append(allPromoted, r.Promoted...)
		}
		assert.Contains(t, allAdded, user2.Id, "user2 should be added as member")
		assert.Contains(t, allPromoted, user2.Id, "user2 should be promoted to admin")

		member, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user2.Id, "")
		require.NoError(t, err)
		assert.True(t, member.SchemeAdmin, "user2 should be channel admin")
	})

	t.Run("empty channel_admins clears all admin roles", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		// First, set BasicUser as admin
		adminIDs := []string{th.BasicUser.Id}
		_, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id},
				ChannelAdmins: &adminIDs,
			}, 0, 0)
		require.NoError(t, err)

		member, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		require.True(t, member.SchemeAdmin)

		// Set channel_admins to empty slice, should demote everyone
		emptyAdmins := []string{}
		results, _, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id,
			&model.SetChannelMembersRequest{
				Members:       []string{th.BasicUser.Id},
				ChannelAdmins: &emptyAdmins,
			}, 0, 0)
		require.NoError(t, err)

		var allDemoted []string
		for _, r := range results {
			allDemoted = append(allDemoted, r.Demoted...)
		}
		assert.Contains(t, allDemoted, th.BasicUser.Id, "BasicUser should be demoted")

		member, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser.Id, "")
		require.NoError(t, err)
		assert.False(t, member.SchemeAdmin, "BasicUser should no longer be admin")
	})

	t.Run("permission-only policy does NOT reject bulk membership edits (bug fix)", func(t *testing.T) {
		// The original `policy_enforced` rejection misfired for channels
		// carrying only a permission policy (e.g. file upload
		// restriction). After Phase 2 the gate reads
		// PolicyActions[membership] specifically, so the endpoint must
		// succeed for these channels.
		channel := th.CreatePrivateChannel(t)
		user2 := th.BasicUser2

		_, err := th.App.Srv().Store().AccessControlPolicy().Save(th.Context, &model.AccessControlPolicy{
			ID:       channel.Id,
			Type:     model.AccessControlPolicyTypeChannel,
			Active:   true,
			Revision: 1,
			Version:  model.AccessControlPolicyVersionV0_2,
			Imports:  []string{},
			Rules: []model.AccessControlPolicyRule{
				{Actions: []string{model.AccessControlPolicyActionUploadFileAttachment}, Expression: "true"},
			},
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = th.App.Srv().Store().AccessControlPolicy().Delete(th.Context, channel.Id)
		})
		th.App.Srv().Store().Channel().InvalidateChannel(channel.Id)

		// Sanity check: the channel reads as PolicyEnforced=true (any
		// policy attached) but PolicyActions[membership]=false. Without
		// this distinction the test below would still pass due to the
		// rejection path simply being dead code, defeating the purpose
		// of the regression test.
		ch, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.True(t, ch.PolicyEnforced, "channel must report PolicyEnforced=true so we know the bug-prone path is reachable")
		require.False(t, ch.HasMembershipPolicyAction(), "channel must NOT carry the membership action — that's the bug-fix invariant")

		results, resp, err := th.SystemAdminClient.SetChannelMembers(ctx, channel.Id, &model.SetChannelMembersRequest{
			Members: []string{th.BasicUser.Id, user2.Id},
		}, 0, 0)
		require.NoError(t, err)
		require.NotNil(t, resp)
		var allAdded []string
		for _, r := range results {
			allAdded = append(allAdded, r.Added...)
		}
		assert.Contains(t, allAdded, user2.Id, "user2 should have been added since the gate must not reject permission-only channels")
	})
}

func TestGetManagedCategories(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)
	client := th.Client

	t.Run("should return 501 without enterprise license", func(t *testing.T) {
		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.Error(t, err)
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("should return empty map when no managed categories exist", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mappings))
		assert.Empty(t, mappings)
	})

	t.Run("should return managed category mappings for the user's channels", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		appErr := th.App.SetChannelManagedCategory(th.Context, th.BasicChannel.Id, "Operations")
		require.Nil(t, appErr)
		defer func() {
			_ = th.App.ClearChannelManagedCategory(th.Context, th.BasicChannel.Id)
		}()

		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mappings))
		assert.Equal(t, "Operations", mappings[th.BasicChannel.Id])
	})
}

func TestGetManagedCategoriesFeatureFlagDisabled(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("route is not registered when feature flag is off at startup", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		defer func() {
			appErr := th.App.Srv().RemoveLicense()
			require.Nil(t, appErr)
		}()

		resp, err := th.Client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.Error(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestPatchChannelManagedCategory(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)
	th.ConfigStore.SetReadOnlyFF(false)
	t.Cleanup(func() {
		th.ConfigStore.SetReadOnlyFF(true)
	})
	client := th.Client

	enableManagedCategories := func() {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ManagedChannelCategories = true })
	}
	disableManagedCategories := func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ManagedChannelCategories = false })
	}
	removeLicense := func() {
		appErr := th.App.Srv().RemoveLicense()
		require.Nil(t, appErr)
	}

	t.Run("non-admin should get 403 when feature is enabled", func(t *testing.T) {
		enableManagedCategories()
		defer removeLicense()

		channel := th.CreatePublicChannel(t)

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		th.AddUserToChannel(t, user, channel)
		userClient := th.CreateClient()
		_, _, err := userClient.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)

		categoryName := "Operations"
		patch := &model.ChannelPatch{ManagedCategoryName: &categoryName}
		_, resp, err := userClient.PatchChannel(context.Background(), channel.Id, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("should silently ignore when no enterprise license", func(t *testing.T) {
		removeLicense()

		channel := th.CreatePublicChannel(t)

		categoryName := "Operations"
		patch := &model.ChannelPatch{ManagedCategoryName: &categoryName}
		_, _, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)

		enableManagedCategories()
		defer removeLicense()

		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mappings))
		_, exists := mappings[channel.Id]
		assert.False(t, exists, "managed category should not be set without license")
	})

	t.Run("should silently ignore when feature is disabled", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))
		disableManagedCategories()
		defer removeLicense()

		channel := th.CreatePublicChannel(t)

		categoryName := "Operations"
		patch := &model.ChannelPatch{ManagedCategoryName: &categoryName}
		_, _, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)

		enableManagedCategories()

		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mappings))
		_, exists := mappings[channel.Id]
		assert.False(t, exists, "managed category should not be set when feature is disabled")
	})

	t.Run("happy path: set and clear managed category", func(t *testing.T) {
		enableManagedCategories()
		defer removeLicense()

		channel := th.CreatePublicChannel(t)

		categoryName := "Operations"
		patch := &model.ChannelPatch{ManagedCategoryName: &categoryName}
		_, _, err := client.PatchChannel(context.Background(), channel.Id, patch)
		require.NoError(t, err)

		resp, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()

		var mappings map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mappings))
		assert.Equal(t, "Operations", mappings[channel.Id])

		emptyName := ""
		clearPatch := &model.ChannelPatch{ManagedCategoryName: &emptyName}
		_, _, err = client.PatchChannel(context.Background(), channel.Id, clearPatch)
		require.NoError(t, err)

		resp2, err := client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", th.BasicTeam.Id), "")
		require.NoError(t, err)
		defer resp2.Body.Close()

		var mappings2 map[string]string
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&mappings2))
		_, exists := mappings2[channel.Id]
		assert.False(t, exists, "managed category should be cleared")
	})
}
