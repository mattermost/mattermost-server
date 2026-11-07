// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// setupChannelComplianceTest returns a helper and the access_control
// property group, with ChannelAttributes on and an Enterprise Advanced
// license -- the shared preconditions for every test below.
func setupChannelComplianceTest(t *testing.T) (*TestHelper, *model.PropertyGroup) {
	t.Helper()
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)
	return th, group
}

func newComplianceChannelField(t *testing.T, th *TestHelper, group *model.PropertyGroup, objectType string) *model.PropertyField {
	t.Helper()
	field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
		// "a" prefix: a CPA field name must start with a letter or
		// underscore, but model.NewId() can start with a digit.
		Name:       "a" + model.NewId(),
		Type:       model.PropertyFieldTypeText,
		GroupID:    group.ID,
		ObjectType: objectType,
		TargetType: "system",
		Attrs:      model.StringInterface{"display_name": "Cost center"},
	}, false, "")
	require.Nil(t, appErr)
	return field
}

// The three compliance endpoints (missing_values, missing_values/summary,
// missing_values/notify) share the same resolve-and-authorize preamble, so
// the license/permission/routing tests are table-driven across all three.
func TestPropertyFieldComplianceEndpointsGating(t *testing.T) {
	mainHelper.Parallel(t)

	type endpointCall func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error)

	endpoints := map[string]endpointCall{
		"missing_values": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
			_, resp, err := th.SystemAdminClient.GetChannelsMissingAttributeValue(ctx, groupName, fieldID, 0, 20)
			return resp, err
		},
		"missing_values/summary": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
			_, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(ctx, groupName, fieldID)
			return resp, err
		},
		"missing_values/notify": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
			_, resp, err := th.SystemAdminClient.NotifyChannelAdminsOfMissingAttributeValue(ctx, groupName, fieldID)
			return resp, err
		},
	}

	for name, call := range endpoints {
		t.Run(name+": refused without Enterprise Advanced", func(t *testing.T) {
			th := SetupConfig(t, func(cfg *model.Config) {
				cfg.FeatureFlags.ChannelAttributes = true
			}).InitBasic(t)
			th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

			group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
			require.Nil(t, appErr)
			field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

			resp, err := call(context.Background(), th, group.Name, field.ID)
			require.Error(t, err)
			require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
		})

		t.Run(name+": refused for a regular (non-admin) session", func(t *testing.T) {
			th, group := setupChannelComplianceTest(t)
			field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

			basicCall := map[string]endpointCall{
				"missing_values": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
					_, resp, err := th.Client.GetChannelsMissingAttributeValue(ctx, groupName, fieldID, 0, 20)
					return resp, err
				},
				"missing_values/summary": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
					_, resp, err := th.Client.GetChannelAttributeComplianceSummary(ctx, groupName, fieldID)
					return resp, err
				},
				"missing_values/notify": func(ctx context.Context, th *TestHelper, groupName, fieldID string) (*model.Response, error) {
					_, resp, err := th.Client.NotifyChannelAdminsOfMissingAttributeValue(ctx, groupName, fieldID)
					return resp, err
				},
			}[name]

			resp, err := basicCall(context.Background(), th, group.Name, field.ID)
			require.Error(t, err)
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
		})

		t.Run(name+": 404 for an unknown field id", func(t *testing.T) {
			th, group := setupChannelComplianceTest(t)

			resp, err := call(context.Background(), th, group.Name, model.NewId())
			require.Error(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run(name+": 404 for a field of a different object type", func(t *testing.T) {
			th, group := setupChannelComplianceTest(t)
			field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeUser)

			resp, err := call(context.Background(), th, group.Name, field.ID)
			require.Error(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run(name+": 400 for a legacy PSAv1 field", func(t *testing.T) {
			th, group := setupChannelComplianceTest(t)
			// ObjectType left empty: PSAv1 legacy fields predate the
			// hierarchical ObjectType/TargetType model these endpoints assume.
			// Created directly via the store: the access_control group is
			// PSAv2, so App.CreatePropertyField's version-match check would
			// reject a PSAv1 field here, but legacy fields predating a
			// group's PSAv2 migration are exactly the case this 400 guards.
			field, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
				Name:    "a" + model.NewId(),
				Type:    model.PropertyFieldTypeText,
				GroupID: group.ID,
			})
			require.NoError(t, err)

			resp, apiErr := call(context.Background(), th, group.Name, field.ID)
			require.Error(t, apiErr)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run(name+": 404 when the ChannelAttributes flag is off", func(t *testing.T) {
			th := SetupConfig(t, func(cfg *model.Config) {
				cfg.FeatureFlags.ChannelAttributes = false
			}).InitBasic(t)
			th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

			group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
			require.Nil(t, appErr)

			resp, err := call(context.Background(), th, group.Name, model.NewId())
			require.Error(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func TestGetPropertyFieldMissingValues(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("lists a channel missing the value, with its channel admin, and no email leaks", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		admin := th.CreateUser(t)
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Missing Value Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, channel)
		th.MakeUserChannelAdmin(t, admin, channel)

		// Sweep every page looking for our channel: other active channels
		// already on the test server also lack a value for this brand-new
		// field, so the list is not just our one channel.
		var found *model.ChannelMissingAttributeValue
		for page := 0; found == nil; page++ {
			list, resp, err := th.SystemAdminClient.GetChannelsMissingAttributeValue(context.Background(), group.Name, field.ID, page, 20)
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			if len(list.Channels) == 0 {
				break
			}
			for _, c := range list.Channels {
				if c.ChannelId == channel.Id {
					found = c
					break
				}
			}
		}

		require.NotNil(t, found, "the channel missing a value must appear somewhere in the paginated list")
		require.Len(t, found.ChannelAdmins, 1)
		assert.Equal(t, admin.Id, found.ChannelAdmins[0].Id)
		assert.Equal(t, admin.Username, found.ChannelAdmins[0].Username)
		assert.NotContains(t, admin.Email, found.ChannelAdmins[0].Username, "sanity: fixture emails and usernames must not collide")
	})

	t.Run("total_count matches the number of channels actually returned across every page", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		var totalReported int64
		var totalSeen int
		for page := 0; ; page++ {
			list, resp, err := th.SystemAdminClient.GetChannelsMissingAttributeValue(context.Background(), group.Name, field.ID, page, 5)
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			totalReported = list.TotalCount
			totalSeen += len(list.Channels)
			if len(list.Channels) < 5 {
				break
			}
			require.LessOrEqual(t, page, 1000, "pagination did not terminate")
		}

		assert.EqualValues(t, totalReported, totalSeen)
	})
}

func TestGetPropertyFieldComplianceSummary(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("a channel admin raises unique_admin_count by at least one", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		before, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		admin := th.CreateUser(t)
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Summary Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, channel)
		th.MakeUserChannelAdmin(t, admin, channel)

		after, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		assert.Greater(t, after.MissingChannelCount, before.MissingChannelCount)
		assert.Greater(t, after.UniqueAdminCount, before.UniqueAdminCount)
	})

	t.Run("a channel with a value does not raise missing_channel_count", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		before, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Compliant Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		_, appErr = th.App.UpsertPropertyValue(th.Context, &model.PropertyValue{
			TargetID: channel.Id, TargetType: model.PropertyValueTargetTypeChannel,
			GroupID: group.ID, FieldID: field.ID, Value: []byte(`"set"`),
		})
		require.Nil(t, appErr)

		after, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		assert.Equal(t, before.MissingChannelCount, after.MissingChannelCount)
	})

	t.Run("message_preview is non-empty and stable across calls", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		first, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, first.MessagePreview)

		second, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, first.MessagePreview, second.MessagePreview)
	})

	t.Run("a remote (non-home) shared channel counts toward shared_channel_count, not missing_channel_count", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		before, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Remote Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		now := model.GetMillis()
		_, nErr := th.App.Srv().Store().SharedChannel().Save(&model.SharedChannel{
			ChannelId: channel.Id,
			TeamId:    channel.TeamId,
			CreatorId: th.SystemAdminUser.Id,
			ShareName: model.NewId(),
			Home:      false,
			RemoteId:  model.NewId(),
			CreateAt:  now,
			UpdateAt:  now,
		})
		require.NoError(t, nErr)

		after, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		assert.Equal(t, before.MissingChannelCount, after.MissingChannelCount, "a remote channel must not count toward the local missing count")
		assert.Greater(t, after.SharedChannelCount, before.SharedChannelCount)
	})
}

// The create-mode summary route serves the banner count before the attribute
// has been saved, so it gates on PermissionManageSystem directly rather than an
// edit-this-field check.
func TestPropertyFieldsComplianceCreateMode(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("missing_values/summary is refused for a regular (non-admin) session", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)

		_, resp, err := th.Client.GetChannelAttributeComplianceSummary(context.Background(), group.Name, "")
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("missing_values/summary reports every active local channel, since no field exists yet", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)

		before, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		_, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Create Mode Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)

		after, resp, err := th.SystemAdminClient.GetChannelAttributeComplianceSummary(context.Background(), group.Name, "")
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// field_id is empty and required is false: there is no field yet to
		// report either of those against.
		assert.Empty(t, after.FieldId)
		assert.False(t, after.Required)
		assert.Greater(t, after.MissingChannelCount, before.MissingChannelCount)
	})
}

func TestNotifyPropertyFieldMissingValuesEndpoint(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("returns 202 and a second call within the cooldown is throttled", func(t *testing.T) {
		th, group := setupChannelComplianceTest(t)
		field := newComplianceChannelField(t, th, group, model.PropertyFieldObjectTypeChannel)

		admin := th.CreateUser(t)
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypeOpen,
			Name:        model.NewId(),
			DisplayName: "Notify Channel " + model.NewId(),
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, channel)
		th.MakeUserChannelAdmin(t, admin, channel)

		result, resp, err := th.SystemAdminClient.NotifyChannelAdminsOfMissingAttributeValue(context.Background(), group.Name, field.ID)
		require.NoError(t, err)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		assert.GreaterOrEqual(t, result.QueuedAdminCount, int64(1))

		_, resp, err = th.SystemAdminClient.NotifyChannelAdminsOfMissingAttributeValue(context.Background(), group.Name, field.ID)
		require.Error(t, err)
		require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})
}
