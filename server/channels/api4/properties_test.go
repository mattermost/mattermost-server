// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

func TestSessionAttributesFieldEditing(t *testing.T) {
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.SessionAttributes = true
	}).InitBasic(t)

	groupName := model.SessionAttributesPropertyGroupName
	objectType := model.PropertyFieldObjectTypeSession

	t.Run("requires an Enterprise Advanced license", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, objectType, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			PerPage:    100,
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.session_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	fields, _, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, objectType, model.PropertyFieldSearch{
		TargetType: string(model.PropertyFieldTargetLevelSystem),
		PerPage:    100,
	})
	require.NoError(t, err)
	require.NotEmpty(t, fields)

	var field *model.PropertyField
	for _, f := range fields {
		if f.Name == model.SessionAttributesPropertyFieldVPNActive {
			field = f
			break
		}
	}
	require.NotNil(t, field)
	require.False(t, field.Protected, "seeded session attribute fields must not be protected")

	t.Run("can enable and tune ttl/grace", func(t *testing.T) {
		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, objectType, field.ID, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				model.SAAttrEnabled:            true,
				model.SAAttrTTLSeconds:         30,
				model.SAAttrGracePeriodSeconds: 30,
			},
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, true, patched.Attrs[model.SAAttrEnabled])
	})

	t.Run("cannot rename", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, objectType, field.ID, &model.PropertyFieldPatch{
			Name: model.NewPointer("renamed_field"),
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "app.session_attributes.field_immutable.app_error")
	})

	t.Run("cannot change non-tunable attrs", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, objectType, field.ID, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.SAAttrDisplayName: "Hacked"},
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("cannot be deleted", func(t *testing.T) {
		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), groupName, objectType, field.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestSessionAttributesFeatureFlagGate(t *testing.T) {
	// Routes are registered through the ClassificationMarkings flag so the
	// generic Properties API is reachable while the SessionAttributes feature
	// flag stays off.
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.SessionAttributes = false
		cfg.FeatureFlags.ClassificationMarkings = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	groupName := model.SessionAttributesPropertyGroupName
	objectType := model.PropertyFieldObjectTypeSession
	search := model.PropertyFieldSearch{
		TargetType: string(model.PropertyFieldTargetLevelSystem),
		PerPage:    100,
	}

	t.Run("admin read returns 501 when feature flag is off", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, objectType, search)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.session_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("non-admin read returns 501 when feature flag is off", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.GetPropertyFields(context.Background(), groupName, objectType, search)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.session_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})
}

func TestPropertyRoutesWithClassificationMarkingsFlag(t *testing.T) {
	mainHelper.Parallel(t)

	// Routes should be available when ClassificationMarkings=true even with IntegratedBoards=false
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = false
		cfg.FeatureFlags.ClassificationMarkings = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{
		Name:    "classification_test",
		Version: model.PropertyGroupVersionV2,
	})
	require.Nil(t, err)
	require.NotNil(t, group)

	t.Run("create field should succeed with ClassificationMarkings flag", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotEmpty(t, createdField.ID)
	})

	t.Run("get fields should succeed with ClassificationMarkings flag", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}

func TestPropertyRoutesWithPostAttributesFlag(t *testing.T) {
	mainHelper.Parallel(t)

	// Routes should be available when PostAttributes=true alone, with every
	// other peer property flag off. The post_attributes group itself is
	// registered unconditionally at startup (see RegisterBuiltinGroups in
	// server.go); only the generic Properties API route registration is
	// gated by the OR condition in InitProperties.
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = false
		cfg.FeatureFlags.ManagedChannelCategories = false
		cfg.FeatureFlags.ClassificationMarkings = false
		cfg.FeatureFlags.SessionAttributes = false
		cfg.FeatureFlags.PostAttributes = true
	}).InitBasic(t)

	groupName := model.PostAttributesPropertyGroupName

	t.Run("create field should succeed with PostAttributes flag", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), groupName, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotEmpty(t, createdField.ID)
	})

	t.Run("get fields should succeed with PostAttributes flag", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, "post", model.PropertyFieldSearch{TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}

// This is the request the sidebar makes first: the webapp loads the managed
// category field before it asks for a team's channel-to-category mappings, so a
// group version the endpoint will not serve hides managed categories entirely
// even though the mappings are intact (MM-71093).
func TestManagedCategoryFieldsRequireGroupAtV2(t *testing.T) {
	mainHelper.Parallel(t)

	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)

	groupName := model.ManagedCategoryPropertyGroupName
	search := model.PropertyFieldSearch{TargetType: "system"}

	// The state server startup leaves behind.
	fields, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, model.PropertyValueTargetTypeChannel, search)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	require.Len(t, fields, 1)
	require.Equal(t, model.ManagedCategoryPropertyFieldName, fields[0].Name)

	// A version the endpoint will not serve, as an affected server booted into.
	// That was version 3, but the endpoint serves V3 now, so use one past it.
	require.NoError(t, th.App.Srv().Store().PropertyGroup().SetVersion(groupName, model.PropertyGroupVersionV3+1))

	_, resp, err = th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, model.PropertyValueTargetTypeChannel, search)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)
}

// The gate is an OR across every Properties feature flag, so the off case is
// only meaningful with all of them disabled — ClassificationMarkings defaults to
// true. This is the half that backs "off by default exposes nothing".
func TestPropertyRoutesAbsentWithoutGateFlags(t *testing.T) {
	mainHelper.Parallel(t)

	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = false
		cfg.FeatureFlags.ManagedChannelCategories = false
		cfg.FeatureFlags.ClassificationMarkings = false
		cfg.FeatureFlags.SessionAttributes = false
		cfg.FeatureFlags.PostAttributes = false
		cfg.FeatureFlags.ChannelAttributes = false
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{
		Name:    "channel_attributes_off_test",
		Version: model.PropertyGroupVersionV2,
	})
	require.Nil(t, appErr)

	_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), group.Name, "channel", model.PropertyFieldSearch{TargetType: "system"})
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	field := &model.PropertyField{
		Name:       model.NewId(),
		Type:       model.PropertyFieldTypeText,
		TargetType: "system",
	}
	_, resp, err = th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "channel", field)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	// Patch a field that genuinely exists, created through the app layer since
	// the create route is gated off (asserted above). A random field ID would
	// 404 on its own and prove nothing about the route being absent.
	memberLevel := model.PermissionLevelMember
	existingField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "channel",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}, false, "")
	require.Nil(t, appErr)

	_, resp, err = th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, []model.PropertyValuePatchItem{
		{FieldID: existingField.ID, Value: json.RawMessage(`"x"`)},
	})
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)
}

func TestPropertyRoutesWithChannelAttributesFlag(t *testing.T) {
	mainHelper.Parallel(t)

	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = false
		cfg.FeatureFlags.ManagedChannelCategories = false
		cfg.FeatureFlags.ClassificationMarkings = false
		cfg.FeatureFlags.SessionAttributes = false
		cfg.FeatureFlags.PostAttributes = false
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{
		Name:    "channel_attributes_test",
		Version: model.PropertyGroupVersionV2,
	})
	require.Nil(t, appErr)
	require.NotNil(t, group)

	field := &model.PropertyField{
		Name:       model.NewId(),
		Type:       model.PropertyFieldTypeText,
		TargetType: "system",
	}
	createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "channel", field)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	require.NotEmpty(t, createdField.ID)

	fields, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), group.Name, "channel", model.PropertyFieldSearch{TargetType: "system"})
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	require.Len(t, fields, 1)
	require.Equal(t, createdField.ID, fields[0].ID)
}

func TestCreatePropertyField(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	// Register a property group for testing
	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		_, resp, err := client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("create protected field should fail", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
			Protected:  true,
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("non-admin should have permissions forced to member", func(t *testing.T) {
		th.LoginBasic(t)

		sysadminLevel := model.PermissionLevelSysadmin
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}

		createdField, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// Non-admin should have permissions forced to member
		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionOptions)
	})

	t.Run("admin should get sysadmin defaults on a system target when not specified", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// On a system target the member level means "any authenticated user"
		// for the definition slots, so those default to sysadmin: a globally
		// scoped field must not be renameable, retypeable or deletable by
		// everyone. Values are the exception -- they are gated by the value's
		// own target rather than the field's, so they keep the member default
		// and stay writable by the people the value's target admits.
		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionOptions)
	})

	t.Run("admin should get member defaults on a channel target when not specified", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// A narrower target keeps the object type default on all three slots:
		// the member level there resolves to channel membership, not everyone.
		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionOptions)
	})

	t.Run("explicit permission levels are not upgraded on a system target", func(t *testing.T) {
		memberLevel := model.PermissionLevelMember
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			TargetType:        "system",
			PermissionOptions: &memberLevel,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// A sysadmin who deliberately pins a slot keeps it: the schema default
		// only fills slots the caller left nil.
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionOptions)
		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionField)
	})

	t.Run("admin should keep custom permissions when specified", func(t *testing.T) {
		sysadminLevel := model.PermissionLevelSysadmin
		memberLevel := model.PermissionLevelMember
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &sysadminLevel,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// Admin should keep the custom permissions
		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionOptions)
	})

	t.Run("admin can set permission level=admin on a channel-target field", func(t *testing.T) {
		adminLevel := model.PermissionLevelAdmin
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelAdmin, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelAdmin, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelAdmin, *createdField.PermissionOptions)
	})

	t.Run("scope admin may pin permission levels", func(t *testing.T) {
		// promote th.BasicUser to be a channel admin of th.BasicChannel
		_, appErr := th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id,
			model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		require.Nil(t, appErr)
		t.Cleanup(func() {
			_, _ = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id, model.ChannelUserRoleId)
		})
		th.LoginBasic(t)

		adminLevel := model.PermissionLevelAdmin
		memberLevel := model.PermissionLevelMember
		sysadminLevel := model.PermissionLevelSysadmin
		noneLevel := model.PermissionLevelNone

		channelField := func() *model.PropertyField {
			return &model.PropertyField{
				Name:       model.NewId(),
				Type:       model.PropertyFieldTypeText,
				TargetType: "channel",
				TargetID:   th.BasicChannel.Id,
			}
		}

		t.Run("channel admin pins admin on a channel-scoped field", func(t *testing.T) {
			field := channelField()
			field.PermissionField = &adminLevel
			field.PermissionValues = &memberLevel
			field.PermissionOptions = &adminLevel

			created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)

			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
			require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)
			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionOptions)
		})

		t.Run("omitted slots are default-filled", func(t *testing.T) {
			field := channelField()
			field.PermissionField = &adminLevel

			created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)

			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
			require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)
			require.Equal(t, model.PermissionLevelMember, *created.PermissionOptions)
		})

		t.Run("pinning sysadmin on permission_field is refused", func(t *testing.T) {
			field := channelField()
			field.PermissionField = &sysadminLevel

			_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
			CheckErrorID(t, err, "api.property_field.create.creator_cannot_edit.app_error")
		})

		t.Run("sysadmin and none are allowed on the values and options slots", func(t *testing.T) {
			for name, tc := range map[string]struct {
				values  *model.PermissionLevel
				options *model.PermissionLevel
			}{
				"values sysadmin":  {values: &sysadminLevel},
				"options sysadmin": {options: &sysadminLevel},
				"values none":      {values: &noneLevel},
				"options none":     {options: &noneLevel},
			} {
				t.Run(name, func(t *testing.T) {
					field := channelField()
					field.PermissionField = &adminLevel
					field.PermissionValues = tc.values
					field.PermissionOptions = tc.options

					created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
					require.NoError(t, err)
					CheckCreatedStatus(t, resp)

					if tc.values != nil {
						require.Equal(t, *tc.values, *created.PermissionValues)
					}
					if tc.options != nil {
						require.Equal(t, *tc.options, *created.PermissionOptions)
					}
				})
			}
		})

		t.Run("permission_field none is refused", func(t *testing.T) {
			// none means nobody can edit, so the anti-lockout check catches it
			// and returns 403.
			field := channelField()
			field.PermissionField = &noneLevel

			_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
			CheckErrorID(t, err, "api.property_field.create.creator_cannot_edit.app_error")
		})

		t.Run("sysadmin submitting permission_field none is refused the same way", func(t *testing.T) {
			// Pins the status-code change for system admins too.
			field := channelField()
			field.TargetType = "system"
			field.TargetID = ""
			field.PermissionField = &noneLevel

			_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
			CheckErrorID(t, err, "api.property_field.create.creator_cannot_edit.app_error")
		})
	})

	t.Run("plain channel member still has levels silently defaulted", func(t *testing.T) {
		// th.BasicUser2 is a plain member of th.BasicChannel and is never
		// promoted, so this pins the unchanged non-scope-admin behaviour.
		th.LoginBasic2(t)
		t.Cleanup(func() { th.LoginBasic(t) })

		sysadminLevel := model.PermissionLevelSysadmin
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}

		created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PermissionLevelMember, *created.PermissionField)
		require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *created.PermissionOptions)
	})

	t.Run("team admin may pin on a team-scoped field", func(t *testing.T) {
		th.LoginTeamAdmin(t)
		t.Cleanup(func() { th.LoginBasic(t) })

		adminLevel := model.PermissionLevelAdmin
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "team",
			TargetID:          th.BasicTeam.Id,
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		}

		created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionValues)
		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionOptions)
	})

	t.Run("DM participants may pin on a DM-scoped field", func(t *testing.T) {
		th.LoginBasic(t)

		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)

		adminLevel := model.PermissionLevelAdmin
		sysadminLevel := model.PermissionLevelSysadmin

		t.Run("admin is honoured", func(t *testing.T) {
			field := &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				TargetType:        "channel",
				TargetID:          dm.Id,
				PermissionField:   &adminLevel,
				PermissionValues:  &adminLevel,
				PermissionOptions: &adminLevel,
			}

			created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)
			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
		})

		t.Run("sysadmin is still refused", func(t *testing.T) {
			field := &model.PropertyField{
				Name:            model.NewId(),
				Type:            model.PropertyFieldTypeText,
				TargetType:      "channel",
				TargetID:        dm.Id,
				PermissionField: &sysadminLevel,
			}

			_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
			CheckErrorID(t, err, "api.property_field.create.creator_cannot_edit.app_error")
		})
	})

	t.Run("linked fields take the source template's definition levels and the caller's values level", func(t *testing.T) {
		// A linked field's permission_field and permission_options come from
		// the source template, so the submitted ones never take effect. That is
		// why linked fields are exempt from the anti-lockout check: validating
		// the caller against a level that will be discarded is meaningless.
		//
		// permission_values is the exception, added with channel attributes: a
		// caller pin wins over the template there, so a linked field can be
		// writable at a lower level than the template it takes its schema
		// from. Channel classification depends on it -- a sysadmin-governed
		// template with a member-writable channel field. See callerPinnedValues
		// in app/properties/property_field.go.
		//
		// th.BasicUser must be a channel admin here: the anti-lockout check
		// only runs for callers who may pin, so a plain member would exercise
		// the silent-default path instead of the exemption.
		_, appErr := th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id,
			model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		require.Nil(t, appErr)
		t.Cleanup(func() {
			_, _ = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id, model.ChannelUserRoleId)
		})
		th.LoginBasic(t)

		adminLevel := model.PermissionLevelAdmin
		memberLevel := model.PermissionLevelMember
		sysadminLevel := model.PermissionLevelSysadmin

		// Templates are created through the App layer, which applies no pin
		// logic, so the source's levels are exactly as specified here.
		newTemplate := func(t *testing.T, field, values, options model.PermissionLevel) string {
			t.Helper()
			source, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				GroupID:           group.ID,
				ObjectType:        model.PropertyFieldObjectTypeTemplate,
				TargetType:        "channel",
				TargetID:          th.BasicChannel.Id,
				PermissionField:   &field,
				PermissionValues:  &values,
				PermissionOptions: &options,
			}, false, "")
			require.Nil(t, appErr)
			return source.ID
		}

		t.Run("a pin the caller cannot satisfy is accepted, not refused", func(t *testing.T) {
			// sysadmin is a level this channel admin cannot satisfy, so a 201
			// here can only mean the anti-lockout check was skipped.
			sourceID := newTemplate(t, sysadminLevel, sysadminLevel, sysadminLevel)
			created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				TargetType:        "channel",
				TargetID:          th.BasicChannel.Id,
				LinkedFieldID:     &sourceID,
				PermissionField:   &sysadminLevel,
				PermissionValues:  &memberLevel,
				PermissionOptions: &memberLevel,
			})
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)

			// The creator is locked out of the definition by design: they
			// cannot edit or delete the field they just created.
			require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)
			// Values keep the caller's pin rather than the template's sysadmin.
			require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)
			require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionOptions)
		})

		t.Run("definition slots come from the source, values comes from the request", func(t *testing.T) {
			// The three source levels are distinct from each other and each
			// differs from what the request submits, so this fails if a
			// definition slot is left as submitted, if values is overwritten by
			// the template, or if any slot is copied from the wrong one or
			// collapsed to a single value.
			sourceID := newTemplate(t, adminLevel, sysadminLevel, memberLevel)
			created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				TargetType:        "channel",
				TargetID:          th.BasicChannel.Id,
				LinkedFieldID:     &sourceID,
				PermissionField:   &memberLevel,
				PermissionValues:  &adminLevel,
				PermissionOptions: &sysadminLevel,
			})
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)

			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
			require.Equal(t, model.PermissionLevelAdmin, *created.PermissionValues)
			require.Equal(t, model.PermissionLevelMember, *created.PermissionOptions)
		})
	})

	t.Run("admin can set permission level=creator on a post-object field", func(t *testing.T) {
		creatorLevel := model.PermissionLevelCreator
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &creatorLevel,
			PermissionOptions: &creatorLevel,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelCreator, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelCreator, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelCreator, *createdField.PermissionOptions)
	})

	t.Run("permission level=creator on a user-object field should fail", func(t *testing.T) {
		// User-object fields have no entity creator, so the level is rejected
		// rather than silently resolving to admin.
		creatorLevel := model.PermissionLevelCreator
		memberLevel := model.PermissionLevelMember
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &memberLevel,
			PermissionValues:  &creatorLevel,
			PermissionOptions: &memberLevel,
		}

		_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "user", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid group name should fail", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		// Invalid group name with uppercase — route regex won't match, returns 404
		_, resp, err := th.Client.CreatePropertyField(context.Background(), "Invalid", "post", field)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("nonexistent group name should fail", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), "nonexistent_group", "post", field)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("successful creation should return 201", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
		}

		createdField, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotEmpty(t, createdField.ID)
		require.Equal(t, field.Name, createdField.Name)
		require.Equal(t, "post", createdField.ObjectType)
		require.Equal(t, group.ID, createdField.GroupID)
		require.Equal(t, th.BasicUser.Id, createdField.CreatedBy)
	})

	t.Run("websocket event should be fired on field creation", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
		}

		createdField, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		var receivedField model.PropertyField
		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyFieldCreated {
					fieldData, ok := event.GetData()["property_field"].(string)
					require.True(t, ok)
					require.NoError(t, json.Unmarshal([]byte(fieldData), &receivedField))
					require.Equal(t, "post", event.GetData()["object_type"])
					require.Equal(t, th.BasicChannel.Id, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, createdField.ID, receivedField.ID)
		require.Equal(t, createdField.Name, receivedField.Name)
	})

	t.Run("group_id in body should be overridden by group_name from URL", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			GroupID:    model.NewId(), // Try to set a different group ID in body
		}

		createdField, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// The group ID should come from the URL's group_name resolution, not the body
		require.Equal(t, group.ID, createdField.GroupID)
	})

	t.Run("system target_type requires ManageSystem permission", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		// Non-admin should be rejected for system-scoped fields
		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should succeed
		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, "system", createdField.TargetType)
	})

	t.Run("channel target_type requires CreatePost permission", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
		}

		// BasicUser is a member of BasicChannel and can post — should succeed
		createdField, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, "channel", createdField.TargetType)
		require.Equal(t, th.BasicChannel.Id, createdField.TargetID)
	})

	t.Run("channel target_type with inaccessible channel should fail", func(t *testing.T) {
		th.LoginBasic(t)

		// Create a channel BasicUser is not a member of
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		otherChannel := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, otherTeam.Id)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
			TargetID:   otherChannel.Id,
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel target_type without target_id should fail", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "channel",
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("team target_type requires ManageTeam permission", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "team",
			TargetID:   th.BasicTeam.Id,
		}

		// BasicUser is a member but not a team admin — should fail
		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// SystemAdmin should succeed
		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, "team", createdField.TargetType)
		require.Equal(t, th.BasicTeam.Id, createdField.TargetID)
	})

	t.Run("team target_type without target_id should fail", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "team",
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("unknown target_type should fail with bad request", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "unknown",
			TargetID:   model.NewId(),
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("empty target_type should fail with bad request", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name: model.NewId(),
			Type: model.PropertyFieldTypeText,
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("v3 group with flag off falls back to v2 and returns 201", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_create", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_create", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), v1Group.Name, "post", field)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("create with permissions against a group not serving v3 is rejected", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					Field: model.WriteOnly{Write: model.PermissionLevelMember},
				},
			},
		}

		_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("create with permissions against a v3 group with the flag on is stored, with no field.write to check", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_create_permissions", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = false
		})

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					// There is no existing field yet to hold field.write on, so a
					// scope admin setting their own new field's permissions is
					// legal regardless of what the restrictions say.
					Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
				},
			},
		}

		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.Permissions)
	})

	t.Run("create with permissions in a v3 group requires administering the field's scope", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_create_permissions_scope", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = false
		})

		// field.write none with a grant naming only the creator: stored, it
		// would leave no administrator able to edit or delete the field.
		lockingField := func(creatorID string) *model.PropertyField {
			return &model.PropertyField{
				Name:       model.NewId(),
				Type:       model.PropertyFieldTypeText,
				TargetType: "channel",
				TargetID:   th.BasicChannel.Id,
				Permissions: &model.Permissions{
					Restrictions: &model.Restrictions{
						Field: model.WriteOnly{Write: model.PermissionLevelNone},
					},
					Grants: []model.Grant{{
						Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: creatorID},
						Allow:    []string{model.PropertyActionFieldWrite},
					}},
				},
			}
		}

		t.Run("a plain channel member is refused and nothing is stored", func(t *testing.T) {
			th.LoginBasic(t)

			field := lockingField(th.BasicUser.Id)
			_, resp, err := th.Client.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			stored, appErr := th.App.SearchPropertyFields(th.Context, v3Group.ID, model.PropertyFieldSearchOpts{
				ObjectTypes: []string{"post"},
				TargetIDs:   []string{th.BasicChannel.Id},
				PerPage:     100,
			})
			require.Nil(t, appErr)
			for _, f := range stored {
				require.NotEqual(t, field.Name, f.Name)
			}
		})

		t.Run("the same member can still create the field without one", func(t *testing.T) {
			th.LoginBasic(t)

			field := lockingField(th.BasicUser.Id)
			field.Permissions = nil
			created, resp, err := th.Client.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)
			// Every new field gets an object converted from the default levels,
			// so none of the refused locking object reaches the store.
			require.NotNil(t, created.Permissions)
			require.NotEqual(t, model.PermissionLevelNone, created.Permissions.Restrictions.TierFor(model.PropertyActionFieldWrite))
			for _, grant := range created.Permissions.Grants {
				require.NotEqual(t, model.PropertyOwnerTypeUser, grant.Type)
			}
		})

		t.Run("a channel admin may submit one", func(t *testing.T) {
			channelAdmin := th.CreateUser(t)
			th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
			_, appErr := th.App.AddUserToChannel(th.Context, channelAdmin, th.BasicChannel, false)
			require.Nil(t, appErr)
			_, appErr = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, channelAdmin.Id,
				model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
			require.Nil(t, appErr)

			client := th.CreateClient()
			_, _, err := client.Login(context.Background(), channelAdmin.Email, channelAdmin.Password)
			require.NoError(t, err)

			// field.write none is refused even from a scope admin: no
			// permissions object submitted through the API may leave a field
			// that no system admin can edit.
			_, resp, err := client.CreatePropertyField(context.Background(), v3Group.Name, "post", lockingField(channelAdmin.Id))
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)

			field := lockingField(channelAdmin.Id)
			field.Permissions.Restrictions.Field.Write = model.PermissionLevelAdmin
			created, resp, err := client.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)
			require.NotNil(t, created.Permissions)
			require.Equal(t, model.PermissionLevelAdmin, created.Permissions.Restrictions.Field.Write)
		})
	})

	t.Run("a non-scope-admin cannot pin the creator permission level", func(t *testing.T) {
		// Only a caller who administers the field's scope may configure
		// permission levels; everyone else has all three pinned to the object
		// type's default, so a submitted creator level is discarded rather
		// than honoured. th.BasicUser is a plain channel_user in
		// th.BasicChannel here.
		th.LoginBasic(t)

		creatorLevel := model.PermissionLevelCreator
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &creatorLevel,
			PermissionOptions: &creatorLevel,
		}

		created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PermissionLevelMember, *created.PermissionField)
		require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *created.PermissionOptions)
	})

	t.Run("channel admin can create an admin-managed field with creator-set values", func(t *testing.T) {
		// A channel admin defines a channel-scoped field whose definition and
		// options only the channel's admins may manage, and whose values each
		// object's creator may set on their own object.
		channelAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
		_, appErr := th.App.AddUserToChannel(th.Context, channelAdmin, th.BasicChannel, false)
		require.Nil(t, appErr)
		_, appErr = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, channelAdmin.Id,
			model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), channelAdmin.Email, channelAdmin.Password)
		require.NoError(t, err)

		adminLevel := model.PermissionLevelAdmin
		creatorLevel := model.PermissionLevelCreator
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &adminLevel,
			PermissionValues:  &creatorLevel,
			PermissionOptions: &adminLevel,
		}

		created, resp, err := client.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
		require.Equal(t, model.PermissionLevelCreator, *created.PermissionValues)
		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionOptions)

		// Re-read: the pin happens before the store write, so echoing the
		// request body back would look identical to a genuine persist.
		fields, resp, err := client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			PerPage:    200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		var fetched *model.PropertyField
		for _, f := range fields {
			if f.ID == created.ID {
				fetched = f
				break
			}
		}
		require.NotNil(t, fetched, "the created field should be readable back")
		require.Equal(t, model.PermissionLevelAdmin, *fetched.PermissionField)
		require.Equal(t, model.PermissionLevelCreator, *fetched.PermissionValues)
		require.Equal(t, model.PermissionLevelAdmin, *fetched.PermissionOptions)
	})

	t.Run("system admin can create an admin-managed field with creator-set values", func(t *testing.T) {
		// The same shape provisioned by a system admin: definition and options
		// managed by the channel's admins, values settable by each object's
		// creator.
		adminLevel := model.PermissionLevelAdmin
		creatorLevel := model.PermissionLevelCreator
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{
					{"name": "confidential"},
					{"name": "public"},
				},
			},
			PermissionField:   &adminLevel,
			PermissionValues:  &creatorLevel,
			PermissionOptions: &adminLevel,
		}

		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)
		require.Equal(t, model.PermissionLevelCreator, *created.PermissionValues)
		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionOptions)
	})
}

func TestServePropertyFieldPermissionsPayload(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
		cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
	}).InitBasic(t)

	v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_permissions_payload", Version: model.PropertyGroupVersionV3})
	require.Nil(t, appErr)
	require.NotNil(t, v3Group)

	v2Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v2_permissions_payload", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)
	require.NotNil(t, v2Group)

	newFieldWithPermissions := func(t *testing.T, groupID string) *model.PropertyField {
		t.Helper()
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			GroupID:    groupID,
			ObjectType: "post",
			TargetType: "system",
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
				},
				Grants: []model.Grant{
					{
						Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()},
						Allow:    []string{model.PropertyActionValueWrite},
					},
				},
			},
		}
		created, err := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, err)
		require.NotNil(t, created.Permissions)
		return created
	}

	t.Run("v2 group carries no permissions key", func(t *testing.T) {
		field := newFieldWithPermissions(t, v2Group.ID)

		th.LoginBasic(t)
		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), v2Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"post"},
			TargetType:  "system",
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		found := findPropertyFieldByID(fields, field.ID)
		require.NotNil(t, found)
		require.Nil(t, found.Permissions)
	})

	t.Run("v2 group reports a linked field's access mode from its template", func(t *testing.T) {
		restrictions := func() *model.Restrictions {
			return &model.Restrictions{
				Field:  model.WriteOnly{Write: model.PermissionLevelNone},
				Value:  model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelNone},
				Option: model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelNone},
			}
		}
		template, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:     v2Group.ID,
			Name:        model.NewId(),
			Type:        model.PropertyFieldTypeText,
			ObjectType:  model.PropertyFieldObjectTypeTemplate,
			TargetType:  "system",
			Permissions: &model.Permissions{Restrictions: restrictions(), Masking: &model.Masking{}},
		})
		require.NoError(t, err)
		linked, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:       v2Group.ID,
			Name:          model.NewId(),
			Type:          model.PropertyFieldTypeText,
			ObjectType:    "post",
			TargetType:    "system",
			LinkedFieldID: &template.ID,
			Permissions:   &model.Permissions{Restrictions: restrictions()},
		})
		require.NoError(t, err)

		fields, resp, err := th.SystemAdminClient.SearchPropertyFields(context.Background(), v2Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"post"},
			TargetType:  "system",
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		found := findPropertyFieldByID(fields, linked.ID)
		require.NotNil(t, found)
		require.Equal(t, model.PropertyAccessModeSharedOnly, found.Attrs[model.PropertyAttrsAccessMode])
		require.Equal(t, true, found.Attrs[model.PropertyAttrsProtected])
	})

	t.Run("v3 group with the flag off carries no permissions key", func(t *testing.T) {
		field := newFieldWithPermissions(t, v3Group.ID)

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = false
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
		})

		th.LoginBasic(t)
		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), v3Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"post"},
			TargetType:  "system",
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		found := findPropertyFieldByID(fields, field.ID)
		require.NotNil(t, found)
		require.Nil(t, found.Permissions)
	})

	t.Run("v3 group with the flag on serves the full object to an admin and a stripped one to a member", func(t *testing.T) {
		field := newFieldWithPermissions(t, v3Group.ID)

		adminFields, resp, err := th.SystemAdminClient.SearchPropertyFields(context.Background(), v3Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"post"},
			TargetType:  "system",
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		adminView := findPropertyFieldByID(adminFields, field.ID)
		require.NotNil(t, adminView)
		require.NotNil(t, adminView.Permissions)
		require.False(t, adminView.Permissions.Filtered)
		require.Len(t, adminView.Permissions.Grants, 1)

		th.LoginBasic(t)
		memberFields, resp, err := th.Client.SearchPropertyFields(context.Background(), v3Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"post"},
			TargetType:  "system",
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		memberView := findPropertyFieldByID(memberFields, field.ID)
		require.NotNil(t, memberView)
		require.NotNil(t, memberView.Permissions)
		require.True(t, memberView.Permissions.Filtered)
		require.Empty(t, memberView.Permissions.Grants)
	})

	t.Run("create and patch responses carry the shaped object too", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			ObjectType: "post",
			TargetType: "system",
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
				},
				Grants: []model.Grant{},
			},
		}
		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), v3Group.Name, "post", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.Permissions)
		require.False(t, created.Permissions.Filtered)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}
		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3Group.Name, "post", created.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, patched.Permissions)
		require.False(t, patched.Permissions.Filtered)
	})

	t.Run("websocket event carries no permissions at all", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			GroupID:    v3Group.ID,
			ObjectType: model.PropertyFieldObjectTypeUser,
			TargetType: "system",
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
				},
				Grants: []model.Grant{
					{Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}, Allow: []string{model.PropertyActionValueWrite}},
				},
				Masking: &model.Masking{
					Except: []model.Identity{{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}},
				},
			},
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), v3Group.Name, model.PropertyFieldObjectTypeUser, field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, createdField.Permissions)

		var receivedField model.PropertyField
		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyFieldCreated {
					fieldData, ok := event.GetData()["property_field"].(string)
					require.True(t, ok)
					require.NoError(t, json.Unmarshal([]byte(fieldData), &receivedField))
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, createdField.ID, receivedField.ID)
		require.Nil(t, receivedField.Permissions)
	})
}

func findPropertyFieldByID(fields []*model.PropertyField, id string) *model.PropertyField {
	for _, f := range fields {
		if f.ID == id {
			return f
		}
	}
	return nil
}

func TestGetPropertyFields(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	// Register property groups for testing
	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_get", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	otherGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_get_other", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, otherGroup)

	memberLevel := model.PermissionLevelMember

	// Create a field in the main group
	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)
	require.NotNil(t, createdField)

	// Create a field in a different group
	otherField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           otherGroup.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdOtherField, appErr := th.App.CreatePropertyField(th.Context, otherField, false, "")
	require.Nil(t, appErr)
	require.NotNil(t, createdOtherField)

	// Hierarchical-scope fixtures live under a dedicated group so length
	// assertions in those subtests can be exact — independent of any fields
	// created by the unrelated subtests in `group` above.
	hierGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_get_hier", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, hierGroup)

	mkField := func(t *testing.T, targetType model.PropertyFieldTargetLevel, targetID string) *model.PropertyField {
		t.Helper()
		f := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           hierGroup.ID,
			ObjectType:        "post",
			TargetType:        string(targetType),
			TargetID:          targetID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		created, cerr := th.App.CreatePropertyField(th.Context, f, false, "")
		require.Nil(t, cerr)
		return created
	}

	// Fixtures: 1 system, 1 team-A (BasicTeam), 1 team-B (other team),
	// 2 channel-X (BasicChannel, in team-A), 1 channel-Y (in team-A).
	otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
	channelY := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, th.BasicTeam.Id)
	// BasicUser is *not* a member of channelY by default.

	sysField := mkField(t, model.PropertyFieldTargetLevelSystem, "")
	teamAField := mkField(t, model.PropertyFieldTargetLevelTeam, th.BasicTeam.Id)
	chanX1Field := mkField(t, model.PropertyFieldTargetLevelChannel, th.BasicChannel.Id)
	chanX2Field := mkField(t, model.PropertyFieldTargetLevelChannel, th.BasicChannel.Id)
	// Side-effect-only fixtures: their DB presence proves the hierarchical
	// filter actively excludes them. ElementsMatch on the expected ID set
	// handles the exclusion check.
	_ = mkField(t, model.PropertyFieldTargetLevelTeam, otherTeam.Id)   // team-B field
	_ = mkField(t, model.PropertyFieldTargetLevelChannel, channelY.Id) // channel-Y field (BasicUser has no access)

	// A dedicated group holding a single system-object-type field
	// (ObjectType=system, TargetType=system). Used by the DWIM subtests
	// to assert that requests with object_type=system collapse to the
	// system scope regardless of any channel/team/target params passed.
	systemObjGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_get_system_obj", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	sysObjField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           systemObjGroup.ID,
		ObjectType:        model.PropertyFieldObjectTypeSystem,
		TargetType:        string(model.PropertyFieldTargetLevelSystem),
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdSysObjField, appErr := th.App.CreatePropertyField(th.Context, sysObjField, false, "")
	require.Nil(t, appErr)

	fieldIDs := func(fields []*model.PropertyField) []string {
		ids := make([]string, len(fields))
		for i, f := range fields {
			ids[i] = f.ID
		}
		return ids
	}

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		_, resp, err := client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{PerPage: 60})
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("nonexistent group name should fail", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), "nonexistent_group", "post", model.PropertyFieldSearch{PerPage: 60})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("successful get should return fields", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{PerPage: 60, TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.NotEmpty(t, fields)
		found := false
		for _, f := range fields {
			if f.ID == createdField.ID {
				found = true
				break
			}
		}
		require.True(t, found, "Created field should be in the response")
	})

	t.Run("cursor pagination should return subsequent pages", func(t *testing.T) {
		th.LoginBasic(t)

		// Create additional fields to have enough for pagination
		for range 3 {
			f := &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				GroupID:           group.ID,
				ObjectType:        "post",
				TargetType:        "system",
				PermissionField:   &memberLevel,
				PermissionValues:  &memberLevel,
				PermissionOptions: &memberLevel,
			}
			_, appErr := th.App.CreatePropertyField(th.Context, f, false, "")
			require.Nil(t, appErr)
		}

		// First request without cursor
		page0, resp, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{PerPage: 2, TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, page0, 2)

		// Use last item from first page as cursor
		lastField := page0[len(page0)-1]
		page1, resp, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{
			PerPage:        2,
			TargetType:     "system",
			CursorID:       lastField.ID,
			CursorCreateAt: lastField.CreateAt,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, page1)

		// Pages should contain different fields
		page0IDs := map[string]bool{}
		for _, f := range page0 {
			page0IDs[f.ID] = true
		}
		for _, f := range page1 {
			require.False(t, page0IDs[f.ID], "Second page should not contain fields from first page")
		}
	})

	t.Run("invalid cursor should return 400", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{
			PerPage:        2,
			TargetType:     "system",
			CursorID:       "not-a-valid-id",
			CursorCreateAt: 12345,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("get should only return fields from the specified group", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{PerPage: 60, TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// All returned fields must belong to the queried group
		for _, f := range fields {
			require.Equal(t, group.ID, f.GroupID, "All returned fields should belong to the queried group")
		}

		// The field from the other group should not be present
		for _, f := range fields {
			require.NotEqual(t, createdOtherField.ID, f.ID, "Field from other group should not be returned")
		}
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		th.LoginBasic(t)

		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_get_fields", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), v3Group.Name, "post", model.PropertyFieldSearch{PerPage: 60, TargetType: "system"})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_get_fields", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), v1Group.Name, "post", model.PropertyFieldSearch{PerPage: 60})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("no scope (no target_type, channel_id or team_id) returns 400 scope_required", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("object_type=system without scope returns system-level rows", func(t *testing.T) {
		// System-object-type fields can only live at the system scope, so
		// the GET endpoint defaults to target_type=system when the caller
		// omits the scope.
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), systemObjGroup.Name, model.PropertyFieldObjectTypeSystem, model.PropertyFieldSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{createdSysObjField.ID}, fieldIDs(fields))
	})

	t.Run("object_type=system collapses to system scope regardless of channel_id (DWIM)", func(t *testing.T) {
		// System-object fields can only live at the system scope by
		// invariant, so the channel_id filter is a semantic no-op. The
		// endpoint accepts the request and returns the same rows as the
		// unscoped call rather than 400-ing on scope_conflict.
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), systemObjGroup.Name, model.PropertyFieldObjectTypeSystem, model.PropertyFieldSearch{
			ChannelID: th.BasicChannel.Id,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{createdSysObjField.ID}, fieldIDs(fields))
	})

	t.Run("object_type=system collapses to system scope regardless of team_id (DWIM)", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), systemObjGroup.Name, model.PropertyFieldObjectTypeSystem, model.PropertyFieldSearch{
			TeamID: th.BasicTeam.Id,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{createdSysObjField.ID}, fieldIDs(fields))
	})

	t.Run("object_type=system collapses to system scope regardless of target_type=channel (DWIM)", func(t *testing.T) {
		// Even a confused single-target request like target_type=channel
		// + target_id=<channel> is reduced to system scope when the
		// object_type is system. The non-system filter values are
		// dropped before the conflict check runs.
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), systemObjGroup.Name, model.PropertyFieldObjectTypeSystem, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelChannel),
			TargetID:   th.BasicChannel.Id,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{createdSysObjField.ID}, fieldIDs(fields))
	})

	t.Run("channel_id combined with target_type returns 400 scope_conflict", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID:  th.BasicChannel.Id,
			TargetType: string(model.PropertyFieldTargetLevelChannel),
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("team_id combined with target_id returns 400 scope_conflict", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID:   th.BasicTeam.Id,
			TargetID: model.NewId(),
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("channel_id pointing at non-existent channel returns 403", func(t *testing.T) {
		// Permission is checked before existence — non-existent channels are
		// indistinguishable from inaccessible ones, by design.
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: model.NewId(),
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel_id where user has no access returns 403", func(t *testing.T) {
		th.LoginBasic(t)

		// BasicUser is not a member of channelY.
		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: channelY.Id,
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("team_id where user has no access returns 403", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID: otherTeam.Id,
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("team_id alone returns system + team rows only", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID:  th.BasicTeam.Id,
			PerPage: 200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 2, "should return exactly sysField + teamAField")
		require.ElementsMatch(t, []string{sysField.ID, teamAField.ID}, fieldIDs(fields))
	})

	t.Run("channel_id + team_id returns system + team + channel rows", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: th.BasicChannel.Id,
			TeamID:    th.BasicTeam.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 4, "should return exactly sysField + teamAField + 2 channel-X fields")
		require.ElementsMatch(t, []string{sysField.ID, teamAField.ID, chanX1Field.ID, chanX2Field.ID}, fieldIDs(fields))
	})

	t.Run("channel_id without team_id returns system + team + channel rows", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: th.BasicChannel.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 4, "should return exactly sysField + teamAField + 2 channel-X fields")
		require.ElementsMatch(t, []string{sysField.ID, teamAField.ID, chanX1Field.ID, chanX2Field.ID}, fieldIDs(fields))
	})

	t.Run("DM channel returns system + channel rows (no team in the hierarchy)", func(t *testing.T) {
		// DM channels have no parent team, so the hierarchy collapses
		// to system → channel. teamAField must not leak in even though
		// the BasicUser shares team-A with other channels.
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		dmField := mkField(t, model.PropertyFieldTargetLevelChannel, dmChannel.Id)

		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: dmChannel.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{sysField.ID, dmField.ID}, fieldIDs(fields))
	})

	t.Run("GM channel returns system + channel rows (no team in the hierarchy)", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		gmField := mkField(t, model.PropertyFieldTargetLevelChannel, gmChannel.Id)

		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: gmChannel.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.ElementsMatch(t, []string{sysField.ID, gmField.ID}, fieldIDs(fields))
	})

	t.Run("a bad team_id is overwritten by the channel's team_id when channel_id is present and correctly returns system + team + channel rows", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			ChannelID: th.BasicChannel.Id,
			TeamID:    otherTeam.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 4, "should return exactly sysField + teamAField + 2 channel-X fields")
		require.ElementsMatch(t, []string{sysField.ID, teamAField.ID, chanX1Field.ID, chanX2Field.ID}, fieldIDs(fields))
	})

	t.Run("since=-1 is treated as no filter", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetType:    string(model.PropertyFieldTargetLevelSystem),
			SinceUpdateAt: -1,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 1, "should return exactly sysField — the only system row in hierGroup at this point")
		require.ElementsMatch(t, []string{sysField.ID}, fieldIDs(fields))
	})

	t.Run("since returns soft-deleted rows", func(t *testing.T) {
		// Create a dedicated field, delete it, then verify it shows up in a since query.
		toDelete := mkField(t, model.PropertyFieldTargetLevelSystem, "")

		// Snapshot a since cutoff strictly less than UpdateAt of the delete.
		// CreatePropertyField sets UpdateAt to CreateAt; use it - 1.
		cutoff := toDelete.UpdateAt - 1

		// Soft-delete via the App layer.
		require.Nil(t, th.App.DeletePropertyField(th.Context, hierGroup.ID, toDelete.ID, false, ""))

		// Use sysadmin to avoid any read-permission masking on tombstones.
		fields, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID:        th.BasicTeam.Id,
			SinceUpdateAt: cutoff,
			PerPage:       200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		var found *model.PropertyField
		for _, f := range fields {
			if f.ID == toDelete.ID {
				found = f
				break
			}
		}
		require.NotNil(t, found, "soft-deleted field should be returned in since-delta")
		require.Greater(t, found.DeleteAt, int64(0), "returned field should be tombstoned")
	})

	t.Run("cursor_create_at while since>0 returns 400", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetType:     string(model.PropertyFieldTargetLevelSystem),
			SinceUpdateAt:  1,
			CursorID:       model.NewId(),
			CursorCreateAt: 12345,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("cursor_update_at while since absent returns 400", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetType:     string(model.PropertyFieldTargetLevelSystem),
			CursorID:       model.NewId(),
			CursorUpdateAt: 12345,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("cursor with both create_at and update_at set returns 400", func(t *testing.T) {
		// Cursor.IsValid() requires exactly one of the two timestamps.
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetType:     string(model.PropertyFieldTargetLevelSystem),
			SinceUpdateAt:  1,
			CursorID:       model.NewId(),
			CursorCreateAt: 12345,
			CursorUpdateAt: 12345,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("target_id alone without scope returns 400", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetID: model.NewId(),
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("delta-mode cursor paginates correctly across multiple updates", func(t *testing.T) {
		// Round-trip the dual-mode cursor: create N fresh fields after a
		// cutoff, then page through them in delta mode with per_page < N.
		// Use a dedicated team so the count is exactly predictable.
		paginationTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		cutoff := model.GetMillis()
		// CreatePropertyField stamps UpdateAt = GetMillis(); sleep to ensure
		// all rows are strictly greater than `cutoff` even at ms precision.
		time.Sleep(2 * time.Millisecond)

		fresh := make([]*model.PropertyField, 0, 3)
		for range 3 {
			fresh = append(fresh, mkField(t, model.PropertyFieldTargetLevelTeam, paginationTeam.Id))
		}

		// Page 1: per_page=2, no cursor. Use sysadmin to side-step team membership.
		page1, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID:        paginationTeam.Id,
			SinceUpdateAt: cutoff,
			PerPage:       2,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, page1, 2)

		// Page 2: cursor from last of page 1, using update_at (delta mode).
		last := page1[len(page1)-1]
		page2, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TeamID:         paginationTeam.Id,
			SinceUpdateAt:  cutoff,
			CursorID:       last.ID,
			CursorUpdateAt: last.UpdateAt,
			PerPage:        2,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Combined pages should match the freshly-created set with no dups or skips.
		combined := append(append([]string{}, fieldIDs(page1)...), fieldIDs(page2)...)
		expected := []string{fresh[0].ID, fresh[1].ID, fresh[2].ID}
		require.ElementsMatch(t, expected, combined, "delta-mode cursor must paginate the full set with no dups or skips")
	})

	t.Run("single-target target_type=team returns only that team's rows", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.GetPropertyFields(context.Background(), hierGroup.Name, "post", model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelTeam),
			TargetID:   th.BasicTeam.Id,
			PerPage:    100,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Len(t, fields, 1, "should return exactly teamAField")
		require.ElementsMatch(t, []string{teamAField.ID}, fieldIDs(fields))
	})
}

func TestGetPropertyFieldsScopeAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_scope", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	baseURL := "/properties/groups/" + group.Name + "/post/fields"

	// target_type=system without target_id should succeed (system needs no resource scope)
	t.Run("system target_type without target_id should succeed", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=system", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=system with target_id should succeed
	t.Run("system target_type with target_id should succeed", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=system&target_id="+model.NewId(), "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=channel without target_id should fail with 400
	t.Run("channel target_type without target_id should fail", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=channel", "")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=channel with target_id the user has access to should succeed
	t.Run("channel target_type with accessible channel should succeed", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=channel&target_id="+th.BasicChannel.Id, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=channel with target_id the user does NOT have access to should fail with 403
	t.Run("channel target_type with inaccessible channel should fail", func(t *testing.T) {
		th.LoginBasic(t)

		// Create a channel in a team the basic user is not a member of
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		otherChannel := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, otherTeam.Id)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=channel&target_id="+otherChannel.Id, "")
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=team without target_id should fail with 400
	t.Run("team target_type without target_id should fail", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=team", "")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=team with target_id the user has access to should succeed
	t.Run("team target_type with accessible team should succeed", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=team&target_id="+th.BasicTeam.Id, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	// target_type=team with target_id the user does NOT have access to should fail with 403
	t.Run("team target_type with inaccessible team should fail", func(t *testing.T) {
		th.LoginBasic(t)

		// Create a team the basic user is not a member of
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=team&target_id="+otherTeam.Id, "")
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		resp.Body.Close()
	})

	// No target_type at all should fail with bad request
	t.Run("no target_type should fail", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL, "")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})
}

func TestGetPropertyFieldsFiltering(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_filter", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	memberLevel := model.PermissionLevelMember

	// Create fields with different target_type/target_id combinations
	systemField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdSystemField, appErr := th.App.CreatePropertyField(th.Context, systemField, false, "")
	require.Nil(t, appErr)

	channelField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "channel",
		TargetID:          th.BasicChannel.Id,
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdChannelField, appErr := th.App.CreatePropertyField(th.Context, channelField, false, "")
	require.Nil(t, appErr)

	otherChannelField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "channel",
		TargetID:          th.BasicChannel2.Id,
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdOtherChannelField, appErr := th.App.CreatePropertyField(th.Context, otherChannelField, false, "")
	require.Nil(t, appErr)

	baseURL := "/properties/groups/" + group.Name + "/post/fields"

	decodeFields := func(t *testing.T, resp *http.Response) []*model.PropertyField {
		t.Helper()
		var fields []*model.PropertyField
		err := json.NewDecoder(resp.Body).Decode(&fields)
		require.NoError(t, err)
		return fields
	}

	fieldIDs := func(fields []*model.PropertyField) map[string]bool {
		ids := make(map[string]bool, len(fields))
		for _, f := range fields {
			ids[f.ID] = true
		}
		return ids
	}

	t.Run("no target_type returns bad request", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL, "")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("filter by target_type=system returns only system fields", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=system", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		ids := fieldIDs(decodeFields(t, resp))
		resp.Body.Close()

		require.True(t, ids[createdSystemField.ID], "system field should be present")
		require.False(t, ids[createdChannelField.ID], "channel field should not be present")
		require.False(t, ids[createdOtherChannelField.ID], "other channel field should not be present")
	})

	t.Run("filter by target_type=channel and target_id returns only matching field", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=channel&target_id="+th.BasicChannel.Id, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		ids := fieldIDs(decodeFields(t, resp))
		resp.Body.Close()

		require.True(t, ids[createdChannelField.ID], "channel field should be present")
		require.False(t, ids[createdSystemField.ID], "system field should not be present")
		require.False(t, ids[createdOtherChannelField.ID], "other channel field should not be present")
	})

	t.Run("filter by target_type=channel and different target_id returns different field", func(t *testing.T) {
		th.LoginBasic(t)

		resp, err := th.Client.DoAPIGet(context.Background(), baseURL+"?target_type=channel&target_id="+th.BasicChannel2.Id, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		ids := fieldIDs(decodeFields(t, resp))
		resp.Body.Close()

		require.True(t, ids[createdOtherChannelField.ID], "other channel field should be present")
		require.False(t, ids[createdSystemField.ID], "system field should not be present")
		require.False(t, ids[createdChannelField.ID], "first channel field should not be present")
	})
}

func TestSearchPropertyFields(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_search", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	memberLevel := model.PermissionLevelMember
	mkField := func(t *testing.T, objectType string, targetType model.PropertyFieldTargetLevel, targetID string) *model.PropertyField {
		t.Helper()
		f := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        objectType,
			TargetType:        string(targetType),
			TargetID:          targetID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		created, cerr := th.App.CreatePropertyField(th.Context, f, false, "")
		require.Nil(t, cerr)
		return created
	}

	otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)

	// Fixtures across multiple object types, all in team-A / channel-X scope.
	postSysField := mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelSystem, "")
	postTeamAField := mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelTeam, th.BasicTeam.Id)
	postChanXField := mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelChannel, th.BasicChannel.Id)
	chanSysField := mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelSystem, "")
	chanTeamAField := mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelTeam, th.BasicTeam.Id)
	userSysField := mkField(t, model.PropertyFieldObjectTypeUser, model.PropertyFieldTargetLevelSystem, "")
	sysObjField := mkField(t, model.PropertyFieldObjectTypeSystem, model.PropertyFieldTargetLevelSystem, "")
	// Out-of-scope rows that must NOT leak into hierarchical results.
	_ = mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelTeam, otherTeam.Id) // team-B
	_ = mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelTeam, otherTeam.Id)

	fieldIDs := func(fields []*model.PropertyField) []string {
		ids := make([]string, len(fields))
		for i, f := range fields {
			ids[i] = f.ID
		}
		return ids
	}

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)
		_, resp, err := client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TeamID:      th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("nonexistent group should return 404", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), "nonexistent_group", model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TeamID:      th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("missing object_types returns 400", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			TeamID: th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid object_types returns 400", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{"garbage"},
			TeamID:      th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("no scope (no target_type, channel_id or team_id) returns 400 scope_required", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("object_types=[system] without scope returns system-object rows", func(t *testing.T) {
		// System-object fields can only live at the system scope, so the
		// endpoint defaults to target_type=system when object_types is
		// exactly [system]. This mirrors the GET endpoint's shortcut.
		th.LoginBasic(t)
		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypeSystem},
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Contains(t, fieldIDs(fields), sysObjField.ID)
	})

	t.Run("object_types=[system] collapses to system scope regardless of channel_id (DWIM)", func(t *testing.T) {
		// Any channel/team/target filter is a semantic no-op when
		// object_types is exactly [system]. The endpoint must return
		// the same rows as the unscoped call rather than 400-ing.
		th.LoginBasic(t)
		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypeSystem},
			ChannelID:   th.BasicChannel.Id,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Contains(t, fieldIDs(fields), sysObjField.ID)
	})

	t.Run("object_types=[system, post] without scope returns 400 (shortcut requires exactly [system])", func(t *testing.T) {
		// The system shortcut is only safe when every requested object
		// type lives at the system scope. Mixing system with another
		// object type without an explicit scope would silently drop the
		// non-system rows under target_type=system, so we reject it.
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{
				model.PropertyFieldObjectTypeSystem,
				model.PropertyFieldObjectTypePost,
			},
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("scope conflict (channel_id + target_type) returns 400", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			ChannelID:   th.BasicChannel.Id,
			TargetType:  string(model.PropertyFieldTargetLevelSystem),
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("team_id unauthorized returns 403", func(t *testing.T) {
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TeamID:      otherTeam.Id,
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("multi-OT hierarchical scope returns all matching rows across types", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{
				model.PropertyFieldObjectTypePost,
				model.PropertyFieldObjectTypeChannel,
				model.PropertyFieldObjectTypeUser,
			},
			ChannelID: th.BasicChannel.Id,
			TeamID:    th.BasicTeam.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// For each requested object_type, the hierarchical scope (system +
		// team-A + channel-X) yields the matching rows:
		//   post    -> postSysField, postTeamAField, postChanXField
		//   channel -> chanSysField, chanTeamAField
		//   user    -> userSysField
		expected := []string{
			postSysField.ID, postTeamAField.ID, postChanXField.ID,
			chanSysField.ID, chanTeamAField.ID,
			userSysField.ID,
		}
		require.Len(t, fields, len(expected))
		require.ElementsMatch(t, expected, fieldIDs(fields))
	})

	t.Run("DM channel returns multi-OT system + channel rows (no team in the hierarchy)", func(t *testing.T) {
		// DM channels have no parent team — the hierarchy collapses to
		// system → channel across every requested object_type.
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		dmPostField := mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelChannel, dmChannel.Id)
		dmChannelField := mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelChannel, dmChannel.Id)

		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{
				model.PropertyFieldObjectTypePost,
				model.PropertyFieldObjectTypeChannel,
				model.PropertyFieldObjectTypeUser,
			},
			ChannelID: dmChannel.Id,
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// system rows for every requested OT + the DM-scoped fields. No
		// team rows (postTeamAField / chanTeamAField) since the DM has
		// no parent team.
		require.ElementsMatch(t, []string{
			postSysField.ID, chanSysField.ID, userSysField.ID,
			dmPostField.ID, dmChannelField.ID,
		}, fieldIDs(fields))
	})

	t.Run("GM channel returns multi-OT system + channel rows (no team in the hierarchy)", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		gmChannelField := mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelChannel, gmChannel.Id)

		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypeChannel, model.PropertyFieldObjectTypeUser},
			ChannelID:   gmChannel.Id,
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.ElementsMatch(t, []string{
			chanSysField.ID, userSysField.ID, gmChannelField.ID,
		}, fieldIDs(fields))
	})

	t.Run("a bad team_id is overwritten by the channel's team_id when channel_id is present and correctly returns multi-OT rows", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{
				model.PropertyFieldObjectTypePost,
				model.PropertyFieldObjectTypeChannel,
				model.PropertyFieldObjectTypeUser,
			},
			ChannelID: th.BasicChannel.Id,
			TeamID:    otherTeam.Id, // wrong team — server should ignore it
			PerPage:   200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Exactly the same set as the "channel_id + team_id" case above, even
		// though the request specified the wrong team_id.
		expected := []string{
			postSysField.ID, postTeamAField.ID, postChanXField.ID,
			chanSysField.ID, chanTeamAField.ID,
			userSysField.ID,
		}
		require.Len(t, fields, len(expected))
		require.ElementsMatch(t, expected, fieldIDs(fields))
	})

	t.Run("single object_type behaves like singular endpoint", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypeChannel},
			TeamID:      th.BasicTeam.Id,
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// team_id scope for object_type=channel: chanSysField + chanTeamAField.
		require.ElementsMatch(t, []string{chanSysField.ID, chanTeamAField.ID}, fieldIDs(fields))
	})

	t.Run("single-target scope returns only that exact slice", func(t *testing.T) {
		th.LoginBasic(t)

		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost, model.PropertyFieldObjectTypeChannel},
			TargetType:  string(model.PropertyFieldTargetLevelTeam),
			TargetID:    th.BasicTeam.Id,
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Single-target team-A across post + channel object types: only the
		// team-A rows, no system or channel rows.
		require.ElementsMatch(t, []string{postTeamAField.ID, chanTeamAField.ID}, fieldIDs(fields))
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		th.LoginBasic(t)

		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_search_fields", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		_, resp, err := th.Client.SearchPropertyFields(context.Background(), v3Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TeamID:      th.BasicTeam.Id,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("v1 group returns 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_search_fields", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		_, resp, err := th.Client.SearchPropertyFields(context.Background(), v1Group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TeamID:      th.BasicTeam.Id,
		})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("channel_id pointing at non-existent channel returns 403", func(t *testing.T) {
		// Mirrors the singular endpoint: permission is checked first, so
		// non-existent and inaccessible channels are indistinguishable.
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			ChannelID:   model.NewId(),
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel_id where user has no access returns 403", func(t *testing.T) {
		// Channel BasicUser is not a member of.
		inaccessibleChannel := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, otherTeam.Id)
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			ChannelID:   inaccessibleChannel.Id,
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("target_id alone without scope returns 400", func(t *testing.T) {
		// target_id with no target_type, channel_id or team_id is malformed.
		th.LoginBasic(t)
		_, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost},
			TargetID:    model.NewId(),
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("duplicate object_type values are idempotent", func(t *testing.T) {
		// Sending the same object_type twice should not double-count rows.
		th.LoginBasic(t)
		fields, resp, err := th.Client.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost, model.PropertyFieldObjectTypePost},
			ChannelID:   th.BasicChannel.Id,
			TeamID:      th.BasicTeam.Id,
			PerPage:     200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Same set as a single-OT "post" query in hierarchical scope:
		// postSysField + postTeamAField + postChanXField (no team-B / channel-Y).
		require.ElementsMatch(t, []string{postSysField.ID, postTeamAField.ID, postChanXField.ID}, fieldIDs(fields))
	})

	t.Run("since returns soft-deleted rows across multiple object types", func(t *testing.T) {
		// Create dedicated post and channel fields scoped to BasicTeam, delete
		// them, then verify both tombstones surface in a search delta query.
		postToDelete := mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelTeam, th.BasicTeam.Id)
		chanToDelete := mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelTeam, th.BasicTeam.Id)

		cutoff := min(postToDelete.UpdateAt, chanToDelete.UpdateAt) - 1

		require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, postToDelete.ID, false, ""))
		require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, chanToDelete.ID, false, ""))

		// Sysadmin sidesteps any read-permission masking on tombstones.
		fields, resp, err := th.SystemAdminClient.SearchPropertyFields(context.Background(), group.Name, model.PropertyFieldSearch{
			ObjectTypes:   []string{model.PropertyFieldObjectTypePost, model.PropertyFieldObjectTypeChannel},
			TeamID:        th.BasicTeam.Id,
			SinceUpdateAt: cutoff,
			PerPage:       200,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Build a quick lookup keyed on ID.
		byID := make(map[string]*model.PropertyField, len(fields))
		for _, f := range fields {
			byID[f.ID] = f
		}
		require.Contains(t, byID, postToDelete.ID, "post tombstone should be returned")
		require.Contains(t, byID, chanToDelete.ID, "channel tombstone should be returned")
		require.Greater(t, byID[postToDelete.ID].DeleteAt, int64(0))
		require.Greater(t, byID[chanToDelete.ID].DeleteAt, int64(0))
	})

	t.Run("cursor pagination paginates a multi-OT scope without dups or skips", func(t *testing.T) {
		// Create a dedicated team and a small set of rows under it across
		// multiple object types, then page through in directory mode.
		paginationTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		fresh := []*model.PropertyField{
			mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelTeam, paginationTeam.Id),
			mkField(t, model.PropertyFieldObjectTypePost, model.PropertyFieldTargetLevelTeam, paginationTeam.Id),
			mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelTeam, paginationTeam.Id),
			mkField(t, model.PropertyFieldObjectTypeChannel, model.PropertyFieldTargetLevelTeam, paginationTeam.Id),
		}

		searchBody := model.PropertyFieldSearch{
			ObjectTypes: []string{model.PropertyFieldObjectTypePost, model.PropertyFieldObjectTypeChannel},
			TargetType:  string(model.PropertyFieldTargetLevelTeam),
			TargetID:    paginationTeam.Id,
			PerPage:     2,
		}

		page1, resp, err := th.SystemAdminClient.SearchPropertyFields(context.Background(), group.Name, searchBody)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, page1, 2)

		last := page1[len(page1)-1]
		searchBody.CursorID = last.ID
		searchBody.CursorCreateAt = last.CreateAt
		page2, resp, err := th.SystemAdminClient.SearchPropertyFields(context.Background(), group.Name, searchBody)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, page2, 2)

		combined := append(append([]string{}, fieldIDs(page1)...), fieldIDs(page2)...)
		expected := []string{fresh[0].ID, fresh[1].ID, fresh[2].ID, fresh[3].ID}
		require.ElementsMatch(t, expected, combined, "search cursor pagination must cover the full set without dups or skips")
	})
}

func TestPatchPropertyField(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
		cfg.FeatureFlags.PropertyFieldRank = true
	}).InitBasic(t)

	// Register property groups for testing
	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_patch", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	otherGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_patch_other", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, otherGroup)

	noneLevel := model.PermissionLevelNone
	memberLevel := model.PermissionLevelMember
	sysadminLevel := model.PermissionLevelSysadmin

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		_, resp, err := client.PatchPropertyField(context.Background(), group.Name, "post", model.NewId(), patch)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	// Regression: a field with object_type post + target_type system and
	// omitted permission levels used to default all three slots to member, and
	// member on a system target resolves to "any authenticated user". That let
	// anyone rename, retype or re-option a globally scoped field, and a type
	// change cascade-deletes every value through TypeChangeValueCleanupHook.
	t.Run("plain member cannot edit a system-scoped field created with default levels", func(t *testing.T) {
		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)

		newName := model.NewId()
		_, resp, err = th.Client.PatchPropertyField(context.Background(), group.Name, "post", created.ID, &model.PropertyFieldPatch{Name: &newName})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// The sysadmin who administers the scope still can, so the denial above
		// is the permission level and not a broken patch route.
		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", created.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, patched.Name)
	})

	t.Run("plain member cannot manage options on a system-scoped field created with default levels", func(t *testing.T) {
		// The options-only branch dispatches to
		// SessionHasPermissionToManagePropertyFieldOptions rather than the
		// field-edit check, so it needs covering separately.
		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{{"id": model.NewId(), "name": "first", "color": "#111111"}},
			},
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.PermissionOptions)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionOptions)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{
			"options": []map[string]any{
				{"id": model.NewId(), "name": "first", "color": "#111111"},
				{"id": model.NewId(), "name": "second", "color": "#222222"},
			},
		}}
		_, resp, err = th.Client.PatchPropertyField(context.Background(), group.Name, "post", created.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("DM participant can manage options on a field pinned to admin", func(t *testing.T) {
		// A DM has no channel-admin tier, so admin resolves to participation.
		// This covers the options-only branch specifically: it dispatches to
		// SessionHasPermissionToManagePropertyFieldOptions rather than the
		// field-edit check (see isOptionsOnlyPatch in properties.go).
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)

		adminLevel := model.PermissionLevelAdmin
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "channel",
			TargetID:   dm.Id,
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "first"}},
			},
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		}, false, "")
		require.Nil(t, appErr)

		optionsPatch := func() *model.PropertyFieldPatch {
			return &model.PropertyFieldPatch{Attrs: &model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": model.NewId()}},
			}}
		}

		th.LoginBasic(t)
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, optionsPatch())
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		nonParticipant := th.CreateUser(t)
		th.LinkUserToTeam(t, nonParticipant, th.BasicTeam)
		nonParticipantClient := th.CreateClient()
		_, _, err = nonParticipantClient.Login(context.Background(), nonParticipant.Email, nonParticipant.Password)
		require.NoError(t, err)

		_, resp, err = nonParticipantClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, optionsPatch())
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("patch cannot move a field to another scope", func(t *testing.T) {
		// A field's scope decides who may administer it, so if patch could
		// reparent a field, a channel admin could pin levels on a channel they
		// administer and then move the result somewhere they do not. The
		// handler defends this by discarding patch.TargetID/TargetType
		// outright; nothing else enforces it, so assert it directly.
		_, appErr := th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id,
			model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		require.Nil(t, appErr)
		t.Cleanup(func() {
			_, _ = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id, model.ChannelUserRoleId)
		})
		th.LoginBasic(t)

		adminLevel := model.PermissionLevelAdmin
		created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, model.PermissionLevelAdmin, *created.PermissionField)

		// th.BasicChannel2 is a channel th.BasicUser belongs to but does not
		// administer, which is the reparenting that would actually gain them
		// something.
		for name, patch := range map[string]*model.PropertyFieldPatch{
			"to a channel they do not administer": {TargetID: model.NewPointer(th.BasicChannel2.Id)},
			"to team scope":                       {TargetType: model.NewPointer("team"), TargetID: model.NewPointer(th.BasicTeam.Id)},
			"to system scope":                     {TargetType: model.NewPointer("system"), TargetID: model.NewPointer("")},
		} {
			t.Run(name, func(t *testing.T) {
				patched, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", created.ID, patch)
				require.NoError(t, err)
				CheckOKStatus(t, resp)

				// The patch succeeds but the scope fields are ignored.
				require.Equal(t, "channel", patched.TargetType)
				require.Equal(t, th.BasicChannel.Id, patched.TargetID)
			})
		}

		// Confirm the scope is unchanged in storage, not merely in the
		// responses above.
		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, created.ID)
		require.Nil(t, appErr)
		require.Equal(t, "channel", stored.TargetType)
		require.Equal(t, th.BasicChannel.Id, stored.TargetID)
	})

	t.Run("protected field update should fail", func(t *testing.T) {
		protectedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			Protected:         true,
			PermissionField:   &noneLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdProtectedField, appErr := th.App.CreatePropertyField(th.Context, protectedField, true, "")
		require.Nil(t, appErr)
		require.NotNil(t, createdProtectedField)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdProtectedField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("object type mismatch should fail", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		// Try to update with wrong object_type in URL. Expected 404 to match
		// the shape of a non-existent field.
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "channel", createdField.ID, patch)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("patch with wrong group name should fail 404", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		// Try to patch using the other group's name — field belongs to `group`, not `otherGroup`.
		// A field not found because of a wrong group must surface as 404, not a generic 500.
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), otherGroup.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
		require.Equal(t, "app.property.not_found.app_error", err.(*model.AppError).Id)
	})

	t.Run("options-only update should check options permission", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &sysadminLevel, // Only admin can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)
		require.NotNil(t, createdField)

		// Try to update options as a non-admin
		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "New Option"},
				},
			},
		}

		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should be able to update options
		_, resp, err = th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("options-only update on non-select field should check field permission", func(t *testing.T) {
		// A text field with options in attrs should NOT use the options permission path
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)
		require.NotNil(t, createdField)

		// Non-admin patches only options on a text field — should require field permission, not options
		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
		}

		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should succeed (has field permission)
		_, resp, err = th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("options-only update on select field with member options permission should succeed", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		newOptionID := model.NewId()
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": newOptionID, "name": "New Option"},
				},
			},
		}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updatedField)

		// Verify the patched option was actually stored, not just that the request succeeded.
		opts := updatedField.Attrs[model.PropertyFieldAttributeOptions].([]any)
		require.Len(t, opts, 1)
		option := opts[0].(map[string]any)
		require.Equal(t, newOptionID, option["id"])
		require.Equal(t, "New Option", option["name"])
	})

	t.Run("options-only update on multiselect field with member options permission should succeed", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeMultiselect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		newOptionID := model.NewId()
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": newOptionID, "name": "New Option"},
				},
			},
		}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updatedField)

		// Verify the patched option was actually stored, not just that the request succeeded.
		opts := updatedField.Attrs[model.PropertyFieldAttributeOptions].([]any)
		require.Len(t, opts, 1)
		option := opts[0].(map[string]any)
		require.Equal(t, newOptionID, option["id"])
		require.Equal(t, "New Option", option["name"])
	})

	t.Run("options-only update on rank field with member options permission should succeed", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeRank,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1", "rank": 1},
				},
			},
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		// A member may re-rank options: options-only patches on rank fields
		// use the narrower manage-options permission, same as select/multiselect.
		th.LoginBasic(t)
		newOptionID := model.NewId()
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": newOptionID, "name": "New Option", "rank": 2},
				},
			},
		}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updatedField)

		// Verify the patched option (including its rank) was actually stored.
		opts := updatedField.Attrs[model.PropertyFieldAttributeOptions].([]any)
		require.Len(t, opts, 1)
		option := opts[0].(map[string]any)
		require.Equal(t, newOptionID, option["id"])
		require.Equal(t, "New Option", option["name"])
		require.EqualValues(t, 2, option["rank"])
	})

	t.Run("name and options update on rank field should check field permission not options", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeRank,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1", "rank": 1},
				},
			},
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{
			Name: &newName,
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "New Option", "rank": 2},
				},
			},
		}

		// Member fails — a structural change (name) on a rank field requires the
		// full edit-field permission, not the narrower options permission.
		th.LoginBasic(t)
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin (has field permission) succeeds.
		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, updatedField.Name)
	})

	t.Run("options-only update on select field with none options permission should fail for all", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &noneLevel, // Nobody can manage options via permission
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "New Option"},
				},
			},
		}

		// Non-admin should fail
		th.LoginBasic(t)
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should also fail
		_, resp, err = th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("name and options update on select field should check field permission not options", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel, // Member can manage options
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{
			Name: &newName,
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "New Option"},
				},
			},
		}

		// Non-admin should fail — name change requires field permission (sysadmin)
		th.LoginBasic(t)
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should succeed
		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, updatedField.Name)
	})

	t.Run("field update should check field permission", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &sysadminLevel, // Only admin can edit field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)
		require.NotNil(t, createdField)

		// Try to update name as a non-admin
		th.LoginBasic(t)
		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Admin should be able to update name
		_, resp, err = th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("successful update should return updated field", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Equal(t, newName, updatedField.Name)
		require.Equal(t, th.BasicUser.Id, updatedField.UpdatedBy)
	})

	t.Run("websocket event should be fired on field update", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		var receivedField model.PropertyField
		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyFieldUpdated {
					fieldData, ok := event.GetData()["property_field"].(string)
					require.True(t, ok)
					require.NoError(t, json.Unmarshal([]byte(fieldData), &receivedField))
					require.Equal(t, "post", event.GetData()["object_type"])
					// system-scoped field: no team or channel in broadcast
					require.Empty(t, event.GetBroadcast().TeamId)
					require.Empty(t, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, updatedField.ID, receivedField.ID)
		require.Equal(t, newName, receivedField.Name)
	})

	t.Run("target_id in patch should be silently ignored", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginSystemAdmin(t)
		newName := model.NewId()
		newTargetID := model.NewId()
		patch := &model.PropertyFieldPatch{
			Name:     &newName,
			TargetID: &newTargetID,
		}

		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Equal(t, newName, updatedField.Name)
		require.Equal(t, createdField.TargetID, updatedField.TargetID)
	})

	t.Run("target_type in patch should be silently ignored", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginSystemAdmin(t)
		newName := model.NewId()
		newTargetType := "channel"
		patch := &model.PropertyFieldPatch{
			Name:       &newName,
			TargetType: &newTargetType,
		}

		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Equal(t, newName, updatedField.Name)
		require.Equal(t, "system", updatedField.TargetType)
	})

	t.Run("options-only patch should preserve other attrs keys via merge semantics", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "system",
			Attrs: model.StringInterface{
				"subtype": "color",
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option 1"},
				},
			},
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		newOptionID := model.NewId()
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": newOptionID, "name": "New Option"},
				},
			},
		}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// The "subtype" key should be preserved even though only "options" was patched
		require.Equal(t, "color", updatedField.Attrs["subtype"])
		// The "options" key should be updated
		require.NotNil(t, updatedField.Attrs["options"])
	})

	t.Run("submitted permissions is gated on field.write", func(t *testing.T) {
		v3PermissionsGroup, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_patch_field_permissions", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3PermissionsGroup)

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.PropertyFieldPermissionsV3 = false
		})

		newFieldWithPermissions := func(t *testing.T) *model.PropertyField {
			t.Helper()
			field := &model.PropertyField{
				Name:       model.NewId(),
				Type:       model.PropertyFieldTypeText,
				GroupID:    v3PermissionsGroup.ID,
				ObjectType: model.PropertyFieldObjectTypeUser,
				TargetType: "system",
				Permissions: &model.Permissions{
					Restrictions: &model.Restrictions{
						Field:  model.WriteOnly{Write: model.PermissionLevelSysadmin},
						Value:  model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelSysadmin},
						Option: model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelSysadmin},
					},
					Grants: []model.Grant{
						{Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}, Allow: []string{model.PropertyActionValueWrite}},
					},
					Masking: &model.Masking{
						Except: []model.Identity{{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}},
					},
				},
			}
			created, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
			require.Nil(t, appErr)
			require.NotNil(t, created.Permissions)
			return created
		}

		newGrantsPatch := func(t *testing.T) *model.PermissionsPatch {
			t.Helper()
			grants, err := json.Marshal([]model.Grant{
				{Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}, Allow: []string{model.PropertyActionValueRead}},
			})
			require.NoError(t, err)
			return &model.PermissionsPatch{Grants: grants}
		}

		t.Run("member without field.write is refused and the field is unchanged", func(t *testing.T) {
			createdField := newFieldWithPermissions(t)

			th.LoginBasic(t)
			patch := &model.PropertyFieldPatch{Permissions: newGrantsPatch(t)}
			_, resp, err := th.Client.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			stored, appErr := th.App.GetPropertyField(th.Context, v3PermissionsGroup.ID, createdField.ID)
			require.Nil(t, appErr)
			require.Equal(t, createdField.Permissions, stored.Permissions)
		})

		t.Run("admin's submission is stored", func(t *testing.T) {
			createdField := newFieldWithPermissions(t)

			patch := &model.PropertyFieldPatch{Permissions: newGrantsPatch(t)}
			updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.NotNil(t, updated.Permissions)
			require.Len(t, updated.Permissions.Grants, 1)
			require.Equal(t, []string{model.PropertyActionValueRead}, updated.Permissions.Grants[0].Allow)
		})

		t.Run("permissions absent leaves the stored object untouched", func(t *testing.T) {
			createdField := newFieldWithPermissions(t)

			newName := model.NewId()
			patch := &model.PropertyFieldPatch{Name: &newName}
			updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.Equal(t, newName, updated.Name)
			require.Equal(t, createdField.Permissions, updated.Permissions)
		})

		t.Run("masking null clears masking and leaves restrictions and grants untouched", func(t *testing.T) {
			createdField := newFieldWithPermissions(t)

			patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{Masking: json.RawMessage("null")}}
			updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.Nil(t, updated.Permissions.Masking)
			require.Equal(t, createdField.Permissions.Restrictions, updated.Permissions.Restrictions)
			require.Equal(t, createdField.Permissions.Grants, updated.Permissions.Grants)
		})

		t.Run("a submitted filtered true is refused", func(t *testing.T) {
			createdField := newFieldWithPermissions(t)

			patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{Filtered: true}}
			_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		})

		t.Run("on a field linked to a masked template", func(t *testing.T) {
			restrictions := model.Restrictions{
				Field:  model.WriteOnly{Write: model.PermissionLevelSysadmin},
				Value:  model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelSysadmin},
				Option: model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelSysadmin},
			}

			newLinkedField := func(t *testing.T, grants []model.Grant) *model.PropertyField {
				t.Helper()
				templateRestrictions := restrictions
				template, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
					Name:       model.NewId(),
					Type:       model.PropertyFieldTypeText,
					GroupID:    v3PermissionsGroup.ID,
					ObjectType: model.PropertyFieldObjectTypeTemplate,
					TargetType: "system",
					Permissions: &model.Permissions{
						Restrictions: &templateRestrictions,
						Masking: &model.Masking{
							Except: []model.Identity{{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}},
						},
					},
				}, false, "")
				require.Nil(t, appErr)

				linkedRestrictions := restrictions
				linked, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
					Name:          model.NewId(),
					Type:          model.PropertyFieldTypeText,
					GroupID:       v3PermissionsGroup.ID,
					ObjectType:    model.PropertyFieldObjectTypeUser,
					TargetType:    "system",
					LinkedFieldID: &template.ID,
					Permissions: &model.Permissions{
						Restrictions: &linkedRestrictions,
						Grants:       grants,
					},
				}, false, "")
				require.Nil(t, appErr)
				require.NotNil(t, linked.Permissions)
				require.Nil(t, linked.Permissions.Masking)
				return linked
			}

			someGrants := []model.Grant{
				{Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}, Allow: []string{model.PropertyActionValueWrite}},
			}

			served := func(t *testing.T, client *model.Client4, fieldID string) *model.Permissions {
				t.Helper()
				fields, resp, err := client.GetPropertyFields(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, model.PropertyFieldSearch{
					TargetType: "system",
					PerPage:    200,
				})
				require.NoError(t, err)
				CheckOKStatus(t, resp)
				found := findPropertyFieldByID(fields, fieldID)
				require.NotNil(t, found)
				require.NotNil(t, found.Permissions)
				return found.Permissions
			}

			// echoOf is the permissions object a client sends back after
			// reading it, key for key.
			echoOf := func(t *testing.T, permissions *model.Permissions) *model.PermissionsPatch {
				t.Helper()
				raw, err := json.Marshal(permissions)
				require.NoError(t, err)
				var echo model.PermissionsPatch
				require.NoError(t, json.Unmarshal(raw, &echo))
				return &echo
			}

			requireUnchanged := func(t *testing.T, linked *model.PropertyField) {
				t.Helper()
				stored, appErr := th.App.GetPropertyField(th.Context, v3PermissionsGroup.ID, linked.ID)
				require.Nil(t, appErr)
				require.Equal(t, linked.Permissions, stored.Permissions)
			}

			t.Run("an editor can send back the permissions they were served", func(t *testing.T) {
				linked := newLinkedField(t, someGrants)

				adminView := served(t, th.SystemAdminClient, linked.ID)
				require.NotNil(t, adminView.Masking, "the template's masking is reported as present")
				require.False(t, adminView.Filtered)
				echo := echoOf(t, adminView)
				require.JSONEq(t, "{}", string(echo.Masking))

				patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, linked.ID, &model.PropertyFieldPatch{Permissions: echo})
				require.NoError(t, err)
				CheckOKStatus(t, resp)
				require.Equal(t, adminView, patched.Permissions)
				requireUnchanged(t, linked)
			})

			t.Run("a caller who cannot edit it is still told the object was filtered", func(t *testing.T) {
				// No grants, so the withheld masking details are the only
				// thing that can set filtered here.
				linked := newLinkedField(t, nil)

				th.LoginBasic(t)
				memberView := served(t, th.Client, linked.ID)
				require.NotNil(t, memberView.Masking)
				require.Empty(t, memberView.Grants)
				require.True(t, memberView.Filtered)
			})

			t.Run("a submitted filtered true is still refused", func(t *testing.T) {
				linked := newLinkedField(t, someGrants)

				echo := echoOf(t, served(t, th.SystemAdminClient, linked.ID))
				echo.Filtered = true
				_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, linked.ID, &model.PropertyFieldPatch{Permissions: echo})
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
				requireUnchanged(t, linked)
			})

			t.Run("masking with contents is still refused", func(t *testing.T) {
				linked := newLinkedField(t, someGrants)

				echo := echoOf(t, served(t, th.SystemAdminClient, linked.ID))
				masking, err := json.Marshal(model.Masking{Except: []model.Identity{{Type: model.PropertyOwnerTypeUser, ID: th.SystemAdminUser.Id}}})
				require.NoError(t, err)
				echo.Masking = masking
				_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, linked.ID, &model.PropertyFieldPatch{Permissions: echo})
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
				requireUnchanged(t, linked)
			})
		})

		t.Run("submission against a v2 group is rejected", func(t *testing.T) {
			v2Field := &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				GroupID:           group.ID,
				ObjectType:        "post",
				TargetType:        "system",
				PermissionField:   &memberLevel,
				PermissionValues:  &memberLevel,
				PermissionOptions: &memberLevel,
			}
			createdField, appErr := th.App.CreatePropertyField(th.Context, v2Field, false, "")
			require.Nil(t, appErr)

			patch := &model.PropertyFieldPatch{Permissions: newGrantsPatch(t)}
			_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdField.ID, patch)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		})

		t.Run("a patch carrying both options and permissions is routed to field.write, not option.write", func(t *testing.T) {
			newOptionsField := func(t *testing.T) *model.PropertyField {
				t.Helper()
				field := &model.PropertyField{
					Name:       model.NewId(),
					Type:       model.PropertyFieldTypeSelect,
					GroupID:    v3PermissionsGroup.ID,
					ObjectType: model.PropertyFieldObjectTypeUser,
					TargetType: "system",
					Attrs: model.StringInterface{
						"options": []map[string]any{
							{"id": model.NewId(), "name": "Option 1"},
						},
					},
					Permissions: &model.Permissions{
						Restrictions: &model.Restrictions{
							Field:  model.WriteOnly{Write: model.PermissionLevelSysadmin},
							Option: model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
							Value:  model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
						},
					},
				}
				created, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
				require.Nil(t, appErr)
				require.NotNil(t, created.Permissions)
				return created
			}

			t.Run("options plus permissions is refused and the field is unchanged", func(t *testing.T) {
				createdField := newOptionsField(t)

				th.LoginBasic(t)
				newOptionID := model.NewId()
				patch := &model.PropertyFieldPatch{
					Attrs: &model.StringInterface{
						"options": []map[string]any{
							{"id": newOptionID, "name": "New Option"},
						},
					},
					Permissions: newGrantsPatch(t),
				}
				_, resp, err := th.Client.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)

				stored, appErr := th.App.GetPropertyField(th.Context, v3PermissionsGroup.ID, createdField.ID)
				require.Nil(t, appErr)
				require.Equal(t, createdField.Attrs, stored.Attrs)
				require.Equal(t, createdField.Permissions, stored.Permissions)
			})

			t.Run("options alone is allowed and the new option list is stored", func(t *testing.T) {
				createdField := newOptionsField(t)

				th.LoginBasic(t)
				newOptionID := model.NewId()
				patch := &model.PropertyFieldPatch{
					Attrs: &model.StringInterface{
						"options": []map[string]any{
							{"id": newOptionID, "name": "New Option"},
						},
					},
				}
				updated, resp, err := th.Client.PatchPropertyField(context.Background(), v3PermissionsGroup.Name, model.PropertyFieldObjectTypeUser, createdField.ID, patch)
				require.NoError(t, err)
				CheckOKStatus(t, resp)
				require.NotNil(t, updated.Attrs["options"])
			})
		})
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_patch_field", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           v3Group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		updatedField, resp, err := th.Client.PatchPropertyField(context.Background(), v3Group.Name, "post", createdField.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, updatedField.Name)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_patch_field", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		_, resp, err := th.Client.PatchPropertyField(context.Background(), v1Group.Name, "post", model.NewId(), patch)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("field creator can patch their own field", func(t *testing.T) {
		creatorLevel := model.PermissionLevelCreator
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			CreatedBy:         th.BasicUser.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &creatorLevel,
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)

		newName := model.NewId()
		patched, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, patched.Name)
	})

	t.Run("non-creator member cannot patch a creator-gated field", func(t *testing.T) {
		creatorLevel := model.PermissionLevelCreator
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			CreatedBy:         th.BasicUser.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &creatorLevel,
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic2(t)

		newName := model.NewId()
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{Name: &newName})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestPropertyFieldPermissionsLockout(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
		cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_permissions_lockout", Version: model.PropertyGroupVersionV3})
	require.Nil(t, appErr)

	restrictions := func(fieldWrite model.PermissionLevel) *model.Restrictions {
		return &model.Restrictions{
			Value:  model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
			Option: model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
			Field:  model.WriteOnly{Write: fieldWrite},
		}
	}

	selfGrant := func(userID string) []model.Grant {
		return []model.Grant{{
			Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: userID},
			Allow:    []string{model.PropertyActionFieldWrite},
		}}
	}

	raw := func(t *testing.T, v any) json.RawMessage {
		t.Helper()
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return b
	}

	// newField stores a channel-scoped field directly, so each subtest
	// starts from a known permissions object that th.BasicUser, a plain
	// member of th.BasicChannel, may or may not edit depending on fieldWrite.
	newField := func(t *testing.T, fieldWrite model.PermissionLevel) *model.PropertyField {
		t.Helper()
		created, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:        model.NewId(),
			Type:        model.PropertyFieldTypeText,
			GroupID:     group.ID,
			ObjectType:  "post",
			TargetType:  "channel",
			TargetID:    th.BasicChannel.Id,
			Permissions: &model.Permissions{Restrictions: restrictions(fieldWrite)},
		}, false, "")
		require.Nil(t, appErr)
		require.NotNil(t, created.Permissions)
		return created
	}

	requireUnchanged := func(t *testing.T, field *model.PropertyField) {
		t.Helper()
		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, field.ID)
		require.Nil(t, appErr)
		require.Equal(t, field.Permissions, stored.Permissions)
	}

	requireSysadminCanRename := func(t *testing.T, field *model.PropertyField) {
		t.Helper()
		newName := model.NewId()
		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, patched.Name)
	}

	t.Run("a member cannot take field.write to a grant naming only themselves", func(t *testing.T) {
		field := newField(t, model.PermissionLevelMember)

		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{
			Restrictions: raw(t, restrictions(model.PermissionLevelNone)),
			Grants:       raw(t, selfGrant(th.BasicUser.Id)),
		}}
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.permissions.field_write_none.app_error")

		requireUnchanged(t, field)
		requireSysadminCanRename(t, field)
	})

	t.Run("a member cannot patch themselves off field.write", func(t *testing.T) {
		field := newField(t, model.PermissionLevelMember)

		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{
			Restrictions: raw(t, restrictions(model.PermissionLevelSysadmin)),
		}}
		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.patch.caller_cannot_edit.app_error")

		requireUnchanged(t, field)
	})

	t.Run("a member holding field.write may still change permissions", func(t *testing.T) {
		field := newField(t, model.PermissionLevelMember)

		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{
			Restrictions: raw(t, restrictions(model.PermissionLevelSysadmin)),
			Grants:       raw(t, selfGrant(th.BasicUser.Id)),
		}}
		patched, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, model.PermissionLevelSysadmin, patched.Permissions.Restrictions.Field.Write)

		requireSysadminCanRename(t, field)
	})

	t.Run("a restrictions object that leaves out leaves is refused", func(t *testing.T) {
		field := newField(t, model.PermissionLevelSysadmin)

		for name, body := range map[string]string{
			"only the value leaves":  `{"value":{"read":"member","write":"member"}}`,
			"field.write left out":   `{"value":{"read":"member","write":"member"},"option":{"read":"member","write":"member"},"field":{}}`,
			"one leaf sent as empty": `{"value":{"read":"member","write":""},"option":{"read":"member","write":"member"},"field":{"write":"sysadmin"}}`,
			"an empty object":        `{}`,
		} {
			t.Run(name, func(t *testing.T) {
				patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{Restrictions: json.RawMessage(body)}}
				_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
				CheckErrorID(t, err, "api.property_field.patch.incomplete_restrictions.app_error")

				requireUnchanged(t, field)
			})
		}

		requireSysadminCanRename(t, field)
	})

	t.Run("sending back the served restrictions with one leaf changed is accepted", func(t *testing.T) {
		field := newField(t, model.PermissionLevelSysadmin)

		newName := model.NewId()
		served, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, served.Permissions)
		require.NotNil(t, served.Permissions.Restrictions)

		edited := *served.Permissions.Restrictions
		edited.Value.Read = model.PermissionLevelSysadmin
		patch := &model.PropertyFieldPatch{Permissions: &model.PermissionsPatch{Restrictions: raw(t, edited)}}
		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, edited, *patched.Permissions.Restrictions)
	})

	t.Run("a system admin cannot set field.write to none", func(t *testing.T) {
		field := newField(t, model.PermissionLevelSysadmin)

		for name, permissions := range map[string]*model.PermissionsPatch{
			"none with a grant naming themselves": {
				Restrictions: raw(t, restrictions(model.PermissionLevelNone)),
				Grants:       raw(t, selfGrant(th.SystemAdminUser.Id)),
			},
			"restrictions cleared": {
				Restrictions: json.RawMessage("null"),
				Grants:       raw(t, selfGrant(th.SystemAdminUser.Id)),
			},
		} {
			t.Run(name, func(t *testing.T) {
				patch := &model.PropertyFieldPatch{Permissions: permissions}
				_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, patch)
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
				CheckErrorID(t, err, "api.property_field.permissions.field_write_none.app_error")

				requireUnchanged(t, field)
			})
		}
	})

	t.Run("a system admin cannot create a field with field.write none", func(t *testing.T) {
		for name, permissions := range map[string]*model.Permissions{
			"none with a grant naming themselves": {
				Restrictions: restrictions(model.PermissionLevelNone),
				Grants:       selfGrant(th.SystemAdminUser.Id),
			},
			"no restrictions at all": {
				Grants: selfGrant(th.SystemAdminUser.Id),
			},
		} {
			t.Run(name, func(t *testing.T) {
				field := &model.PropertyField{
					Name:        model.NewId(),
					Type:        model.PropertyFieldTypeText,
					TargetType:  "channel",
					TargetID:    th.BasicChannel.Id,
					Permissions: permissions,
				}
				_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
				require.Error(t, err)
				CheckBadRequestStatus(t, resp)
				CheckErrorID(t, err, "api.property_field.permissions.field_write_none.app_error")
			})
		}
	})

	t.Run("an unchanged permissions echo on an options patch is judged as options only", func(t *testing.T) {
		// The member holds option.write but not field.write, so a
		// presence-keyed anti-lockout check would refuse them.
		created, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Air"}},
			},
			Permissions: &model.Permissions{Restrictions: restrictions(model.PermissionLevelSysadmin)},
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Sea"}},
			},
			Permissions: &model.PermissionsPatch{Restrictions: raw(t, created.Permissions.Restrictions)},
		}
		patched, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", created.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, created.Permissions.Restrictions, patched.Permissions.Restrictions)
	})

	t.Run("callers outside the HTTP API may still leave field.write at none", func(t *testing.T) {
		pluginID := "com.example.lockout"
		pluginAPI := app.NewPluginAPI(th.App, th.Context, &model.Manifest{Id: pluginID})
		newPluginField := func(t *testing.T, fieldWrite model.PermissionLevel) *model.PropertyField {
			t.Helper()
			created, err := pluginAPI.CreatePropertyField(&model.PropertyField{
				Name:       model.NewId(),
				Type:       model.PropertyFieldTypeText,
				GroupID:    group.ID,
				ObjectType: "post",
				TargetType: "channel",
				TargetID:   th.BasicChannel.Id,
				Permissions: &model.Permissions{
					Restrictions: restrictions(fieldWrite),
					Grants: []model.Grant{{
						Identity: model.Identity{Type: model.PropertyOwnerTypePlugin, ID: pluginID},
						Allow:    []string{model.PropertyActionFieldWrite},
					}},
				},
			})
			require.NoError(t, err)
			return created
		}

		t.Run("on create", func(t *testing.T) {
			created := newPluginField(t, model.PermissionLevelNone)
			require.Equal(t, model.PermissionLevelNone, created.Permissions.Restrictions.Field.Write)
		})

		t.Run("on update, where a partial restrictions object keeps its meaning", func(t *testing.T) {
			// The enforcement hook refuses a plugin that is not installed,
			// and none is here, so the LDAP sync stands in: like a plugin,
			// it reaches the app layer without coming through the handlers.
			created, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
				Name:       model.NewId(),
				Type:       model.PropertyFieldTypeText,
				GroupID:    group.ID,
				ObjectType: "post",
				TargetType: "channel",
				TargetID:   th.BasicChannel.Id,
				Permissions: &model.Permissions{
					Restrictions: restrictions(model.PermissionLevelSysadmin),
					Grants: []model.Grant{{
						Identity: model.Identity{Type: model.PropertyOwnerTypeService, ID: model.PropertyFieldAttrLDAP},
						Allow:    []string{model.PropertyActionFieldWrite},
					}},
				},
			}, false, "")
			require.Nil(t, appErr)

			// The leaves it leaves out, field.write among them, are stored
			// as none.
			created.Permissions.Restrictions = &model.Restrictions{Value: model.ReadWrite{Read: model.PermissionLevelMember}}
			syncCtx := app.RequestContextWithCallerID(th.Context, model.CallerIDLDAPSync)
			updated, _, appErr := th.App.UpdatePropertyField(syncCtx, group.ID, created, false, "")
			require.Nil(t, appErr)
			require.Equal(t, model.PermissionLevelMember, updated.Permissions.Restrictions.Value.Read)
			require.Equal(t, model.PermissionLevelNone, updated.Permissions.Restrictions.Field.Write)
		})
	})
}

func TestPermissionsLockOutSystemAdmins(t *testing.T) {
	withFieldWrite := func(level model.PermissionLevel) *model.Permissions {
		return &model.Permissions{Restrictions: &model.Restrictions{Field: model.WriteOnly{Write: level}}}
	}
	session := model.Session{UserId: model.NewId()}

	t.Run("none is below the floor", func(t *testing.T) {
		require.True(t, permissionsLockOutSystemAdmins(session, withFieldWrite(model.PermissionLevelNone)))
		require.True(t, permissionsLockOutSystemAdmins(session, withFieldWrite("")))
		require.True(t, permissionsLockOutSystemAdmins(session, &model.Permissions{}))
	})

	t.Run("every other tier admits a system admin", func(t *testing.T) {
		for _, level := range []model.PermissionLevel{
			model.PermissionLevelSysadmin,
			model.PermissionLevelAdmin,
			model.PermissionLevelCreator,
			model.PermissionLevelMember,
		} {
			require.False(t, permissionsLockOutSystemAdmins(session, withFieldWrite(level)), level)
		}
	})

	t.Run("local mode is exempt", func(t *testing.T) {
		require.False(t, permissionsLockOutSystemAdmins(model.Session{Local: true}, withFieldWrite(model.PermissionLevelNone)))
		require.False(t, permissionsLockOutSystemAdmins(model.Session{Local: true}, &model.Permissions{}))
	})
}

func TestDeletePropertyField(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	// Register property groups for testing
	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_delete", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, group)

	otherGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_delete_other", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)
	require.NotNil(t, otherGroup)

	noneLevel := model.PermissionLevelNone
	memberLevel := model.PermissionLevelMember
	sysadminLevel := model.PermissionLevelSysadmin

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		resp, err := client.DeletePropertyField(context.Background(), group.Name, "post", model.NewId())
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	// Regression: the member default on a system target let any authenticated
	// user delete a globally scoped field. See the matching cases in
	// TestPatchPropertyField and TestCreatePropertyField.
	t.Run("plain member cannot delete a system-scoped field created with default levels", func(t *testing.T) {
		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)

		resp, err = th.Client.DeletePropertyField(context.Background(), group.Name, "post", created.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// The sysadmin who administers the scope still can.
		resp, err = th.SystemAdminClient.DeletePropertyField(context.Background(), group.Name, "post", created.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("protected field delete should fail", func(t *testing.T) {
		protectedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			Protected:         true,
			PermissionField:   &noneLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdProtectedField, appErr := th.App.CreatePropertyField(th.Context, protectedField, true, "")
		require.Nil(t, appErr)

		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), group.Name, "post", createdProtectedField.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("object type mismatch should fail", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		// Try to delete with wrong object_type in URL. Expected 404 to match
		// the shape of a non-existent field.
		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), group.Name, "channel", createdField.ID)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("delete with wrong group name should fail 404", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		// Try to delete using the other group's name — field belongs to `group`, not `otherGroup`.
		// A field not found because of a wrong group must surface as 404, not a generic 500.
		th.LoginSystemAdmin(t)
		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), otherGroup.Name, "post", createdField.ID)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
		require.Equal(t, "app.property.not_found.app_error", err.(*model.AppError).Id)
	})

	t.Run("DM participant can delete a field pinned to admin", func(t *testing.T) {
		// Delete is gated on PermissionField, so a DM participant who pins
		// admin must still be able to remove their own field — this is the
		// delete-and-recreate recovery path for a mis-provisioned field.
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)

		adminLevel := model.PermissionLevelAdmin
		newDMField := func(t *testing.T) *model.PropertyField {
			t.Helper()
			field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				GroupID:           group.ID,
				ObjectType:        "post",
				TargetType:        "channel",
				TargetID:          dm.Id,
				PermissionField:   &adminLevel,
				PermissionValues:  &adminLevel,
				PermissionOptions: &adminLevel,
			}, false, "")
			require.Nil(t, appErr)
			return field
		}

		nonParticipant := th.CreateUser(t)
		th.LinkUserToTeam(t, nonParticipant, th.BasicTeam)
		nonParticipantClient := th.CreateClient()
		_, _, err := nonParticipantClient.Login(context.Background(), nonParticipant.Email, nonParticipant.Password)
		require.NoError(t, err)

		resp, err := nonParticipantClient.DeletePropertyField(context.Background(), group.Name, "post", newDMField(t).ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		th.LoginBasic(t)
		resp, err = th.Client.DeletePropertyField(context.Background(), group.Name, "post", newDMField(t).ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("user without permission should not be able to delete", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &sysadminLevel, // Only admin can edit/delete field
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		resp, err := th.Client.DeletePropertyField(context.Background(), group.Name, "post", createdField.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("successful delete should return 200", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		resp, err := th.Client.DeletePropertyField(context.Background(), group.Name, "post", createdField.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("websocket event should be fired on field deletion", func(t *testing.T) {
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		resp, err := th.Client.DeletePropertyField(context.Background(), group.Name, "post", createdField.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyFieldDeleted {
					require.Equal(t, createdField.ID, event.GetData()["field_id"])
					require.Equal(t, "post", event.GetData()["object_type"])
					// system-scoped field: no team or channel in broadcast
					require.Empty(t, event.GetBroadcast().TeamId)
					require.Empty(t, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_delete_field", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           v3Group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		resp, err := th.Client.DeletePropertyField(context.Background(), v3Group.Name, "post", createdField.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_delete_field", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		resp, err := th.Client.DeletePropertyField(context.Background(), v1Group.Name, "post", model.NewId())
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("field creator can delete their own field", func(t *testing.T) {
		// permission_field backs this handler as well as the patch handler, so
		// the creator level covers deletion too. Intended, and pinned here so
		// it is not mistaken for a bug and "fixed" later.
		creatorLevel := model.PermissionLevelCreator
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			CreatedBy:         th.BasicUser.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &creatorLevel,
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)

		resp, err := th.Client.DeletePropertyField(context.Background(), group.Name, "post", field.ID)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("non-creator member cannot delete a creator-gated field", func(t *testing.T) {
		creatorLevel := model.PermissionLevelCreator
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			CreatedBy:         th.BasicUser.Id,
			PermissionField:   &creatorLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &creatorLevel,
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic2(t)

		resp, err := th.Client.DeletePropertyField(context.Background(), group.Name, "post", field.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestIsOptionsOnlyPatch(t *testing.T) {
	t.Run("nil attrs is not options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Name: new("new name"),
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("empty attrs is not options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{},
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("attrs with only options is options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []any{},
			},
		}
		require.True(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("attrs with only options but nothing actually changed is not options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []any{},
			},
		}
		require.False(t, isOptionsOnlyPatch(patch, false, false))
	})

	t.Run("attrs with options and other keys is not options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []any{},
				"other":   "value",
			},
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("name change with options is not options-only", func(t *testing.T) {
		patch := &model.PropertyFieldPatch{
			Name: new("new name"),
			Attrs: &model.StringInterface{
				"options": []any{},
			},
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("type change is not options-only", func(t *testing.T) {
		newType := model.PropertyFieldTypeSelect
		patch := &model.PropertyFieldPatch{
			Type: &newType,
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true))
	})

	t.Run("PermissionValues alongside an options-only attrs patch is not options-only", func(t *testing.T) {
		memberLevel := model.PermissionLevelMember
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []any{},
			},
			PermissionValues: &memberLevel,
		}
		require.False(t, isOptionsOnlyPatch(patch, false, true), "PermissionValues must always require the full-edit permission tier, never the weaker options-only one")
	})
}

func TestGetPropertyValues(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_values_get", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	// Create a field
	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)

	// Use a real post as the target so target access checks pass
	targetID := th.BasicPost.Id

	// Create a value via upsert
	value := &model.PropertyValue{
		TargetID:   targetID,
		TargetType: "post",
		GroupID:    group.ID,
		FieldID:    createdField.ID,
		Value:      json.RawMessage(`"hello"`),
		CreatedBy:  th.BasicUser.Id,
		UpdatedBy:  th.BasicUser.Id,
	}
	_, appErr2 := th.App.UpsertPropertyValues(th.Context, []*model.PropertyValue{value}, "", "", "")
	require.Nil(t, appErr2)

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		_, resp, err := client.GetPropertyValues(context.Background(), group.Name, "post", targetID, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("successful get should return values", func(t *testing.T) {
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", targetID, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)

		found := false
		for _, v := range values {
			if v.FieldID == createdField.ID {
				found = true
				require.Equal(t, json.RawMessage(`"hello"`), v.Value)
				break
			}
		}
		require.True(t, found, "Created value should be in the response")
	})

	t.Run("nonexistent group should fail", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyValues(context.Background(), "nonexistent_group", "post", targetID, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("invalid target_id should not match route", func(t *testing.T) {
		th.LoginBasic(t)

		// The route regex [A-Za-z0-9]+ rejects IDs with invalid characters
		_, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", "bad-id", model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("get with no values returns empty array", func(t *testing.T) {
		th.LoginBasic(t)

		emptyPost := th.CreatePost(t)
		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", emptyPost.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Empty(t, values)
	})

	t.Run("cursor pagination should return subsequent pages", func(t *testing.T) {
		th.LoginBasic(t)

		// Create a real post and additional fields/values for pagination
		paginationPost := th.CreatePost(t)
		paginationTarget := paginationPost.Id
		for range 4 {
			f := &model.PropertyField{
				Name:              model.NewId(),
				Type:              model.PropertyFieldTypeText,
				GroupID:           group.ID,
				ObjectType:        "post",
				TargetType:        "system",
				PermissionField:   &memberLevel,
				PermissionValues:  &memberLevel,
				PermissionOptions: &memberLevel,
			}
			cf, appErr := th.App.CreatePropertyField(th.Context, f, false, "")
			require.Nil(t, appErr)

			_, appErr2 := th.App.UpsertPropertyValues(th.Context, []*model.PropertyValue{{
				TargetID:   paginationTarget,
				TargetType: "post",
				GroupID:    group.ID,
				FieldID:    cf.ID,
				Value:      json.RawMessage(`"val"`),
				CreatedBy:  th.BasicUser.Id,
				UpdatedBy:  th.BasicUser.Id,
			}}, "", "", "")
			require.Nil(t, appErr2)
		}

		// First page
		page0, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", paginationTarget, model.PropertyValueSearch{PerPage: 2})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, page0, 2)

		// Second page using cursor from last item
		last := page0[len(page0)-1]
		page1, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", paginationTarget, model.PropertyValueSearch{
			PerPage:        2,
			CursorID:       last.ID,
			CursorCreateAt: last.CreateAt,
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, page1)

		// Pages should not overlap
		page0IDs := map[string]bool{}
		for _, v := range page0 {
			page0IDs[v.ID] = true
		}
		for _, v := range page1 {
			require.False(t, page0IDs[v.ID], "Second page should not contain values from first page")
		}
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		th.LoginBasic(t)

		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_get_values", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		_, resp, err := th.Client.GetPropertyValues(context.Background(), v3Group.Name, "post", th.BasicPost.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_get_values", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		_, resp, err := th.Client.GetPropertyValues(context.Background(), v1Group.Name, "post", th.BasicPost.Id, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("since", func(t *testing.T) {
		// Dedicated post + field/value so we can move UpdateAt deterministically.
		sincePost := th.CreatePost(t)
		sinceTarget := sincePost.Id

		sinceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdSinceField, appErr := th.App.CreatePropertyField(th.Context, sinceField, false, "")
		require.Nil(t, appErr)

		upserted, appErr := th.App.UpsertPropertyValues(th.Context, []*model.PropertyValue{{
			TargetID:   sinceTarget,
			TargetType: "post",
			GroupID:    group.ID,
			FieldID:    createdSinceField.ID,
			Value:      json.RawMessage(`"initial"`),
			CreatedBy:  th.BasicUser.Id,
			UpdatedBy:  th.BasicUser.Id,
		}}, "", "", "")
		require.Nil(t, appErr)
		require.Len(t, upserted, 1)
		sinceValue := upserted[0]

		t.Run("since=-1 is treated as no filter", func(t *testing.T) {
			th.LoginBasic(t)

			values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", sinceTarget, model.PropertyValueSearch{
				SinceUpdateAt: -1,
				PerPage:       60,
			})
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.NotEmpty(t, values, "negative since should behave like no filter")
		})

		t.Run("since>0 returns only rows with UpdateAt > since", func(t *testing.T) {
			th.LoginBasic(t)

			cutoff := model.GetMillis() + 10_000 // far in the future
			values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", sinceTarget, model.PropertyValueSearch{
				SinceUpdateAt: cutoff,
				PerPage:       60,
			})
			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.Empty(t, values, "since cutoff in the future should exclude all rows")
		})

		t.Run("since returns soft-deleted rows", func(t *testing.T) {
			th.LoginBasic(t)

			// Snapshot a since cutoff strictly less than the upcoming tombstone UpdateAt.
			cutoff := sinceValue.UpdateAt - 1

			// Soft-delete the field through the app layer; this cascades to its values.
			require.Nil(t, th.App.DeletePropertyField(th.Context, group.ID, createdSinceField.ID, false, ""))

			values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", sinceTarget, model.PropertyValueSearch{
				SinceUpdateAt: cutoff,
				PerPage:       60,
			})
			require.NoError(t, err)
			CheckOKStatus(t, resp)

			var found *model.PropertyValue
			for _, v := range values {
				if v.ID == sinceValue.ID {
					found = v
					break
				}
			}
			require.NotNil(t, found, "soft-deleted value should be returned in since-delta")
			require.Greater(t, found.DeleteAt, int64(0), "returned value should be tombstoned")
		})

		t.Run("cursor_create_at while since>0 returns 400", func(t *testing.T) {
			th.LoginBasic(t)

			_, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", sinceTarget, model.PropertyValueSearch{
				SinceUpdateAt:  1,
				CursorID:       model.NewId(),
				CursorCreateAt: 12345,
				PerPage:        60,
			})
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		})

		t.Run("cursor_update_at while since absent returns 400", func(t *testing.T) {
			th.LoginBasic(t)

			_, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", sinceTarget, model.PropertyValueSearch{
				CursorID:       model.NewId(),
				CursorUpdateAt: 12345,
				PerPage:        60,
			})
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		})
	})
}

func TestPropertyFieldCreatorRemovedFromScope(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_field_creator_scope", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	creatorLevel := model.PermissionLevelCreator
	sysadminLevel := model.PermissionLevelSysadmin

	// Creating a field earns no standing to keep editing it after losing access
	// to the channel it is scoped to. Field and option *reads* are scope-gated
	// in resolveScopeAndCheckPermissions, so without the matching check on the
	// write path a removed creator got 403 on GET and 200 on PATCH and DELETE
	// of the same field.
	removedCreator := th.CreateUser(t)
	th.LinkUserToTeam(t, removedCreator, th.BasicTeam)
	_, appErr := th.App.AddUserToChannel(th.Context, removedCreator, th.BasicChannel, false)
	require.Nil(t, appErr)

	// A channel admin who keeps their membership, to prove the fix denies the
	// removed creator specifically rather than making the field unmanageable.
	channelAdmin := th.CreateUser(t)
	th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
	_, appErr = th.App.AddUserToChannel(th.Context, channelAdmin, th.BasicChannel, false)
	require.Nil(t, appErr)
	_, appErr = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, channelAdmin.Id,
		model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
	require.Nil(t, appErr)

	createField := func(t *testing.T, fieldType model.PropertyFieldType, attrs model.StringInterface) *model.PropertyField {
		t.Helper()
		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              fieldType,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			CreatedBy:         removedCreator.Id,
			Attrs:             attrs,
			PermissionField:   &creatorLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &creatorLevel,
		}, false, "")
		require.Nil(t, appErr)
		return field
	}

	login := func(u *model.User) *model.Client4 {
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), u.Email, u.Password)
		require.NoError(t, err)
		return client
	}

	creatorClient := login(removedCreator)
	channelAdminClient := login(channelAdmin)

	// Every field is built while the creator is still in the channel, so the
	// control below establishes that the setup grants access and the removal is
	// what takes it away.
	inScopeField := createField(t, model.PropertyFieldTypeText, nil)
	nameField := createField(t, model.PropertyFieldTypeText, nil)
	typeField := createField(t, model.PropertyFieldTypeText, nil)
	deleteField := createField(t, model.PropertyFieldTypeText, nil)
	adminField := createField(t, model.PropertyFieldTypeText, nil)
	optionsField := createField(t, model.PropertyFieldTypeSelect, nil)

	t.Run("field creator still in the channel can patch their own field", func(t *testing.T) {
		newName := model.NewId()
		patched, resp, err := creatorClient.PatchPropertyField(context.Background(), group.Name, "post", inScopeField.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, patched.Name)
	})

	require.Nil(t, th.App.RemoveUserFromChannel(th.Context, removedCreator.Id, "", th.BasicChannel))

	t.Run("field creator removed from the channel cannot patch their own field", func(t *testing.T) {
		newName := model.NewId()
		_, resp, err := creatorClient.PatchPropertyField(context.Background(), group.Name, "post", nameField.ID, &model.PropertyFieldPatch{Name: &newName})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.update.no_field_permission.app_error")
	})

	t.Run("field creator removed from the channel cannot retype their own field", func(t *testing.T) {
		// The sharpest of the four: a type change cascades through
		// TypeChangeValueCleanupHook and clears every dependent property
		// value, so this was a way to wipe values in a channel the caller can
		// no longer read.
		newType := model.PropertyFieldTypeSelect
		_, resp, err := creatorClient.PatchPropertyField(context.Background(), group.Name, "post", typeField.ID, &model.PropertyFieldPatch{Type: &newType})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.update.no_field_permission.app_error")
	})

	t.Run("field creator removed from the channel cannot delete their own field", func(t *testing.T) {
		resp, err := creatorClient.DeletePropertyField(context.Background(), group.Name, "post", deleteField.ID)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.delete.no_permission.app_error")
	})

	t.Run("field creator removed from the channel cannot add options to their own field", func(t *testing.T) {
		// permission_options runs through the same dispatcher, so the option
		// handlers inherited the same hole.
		_, resp, err := creatorClient.CreatePropertyFieldOptions(context.Background(), group.Name, "post", optionsField.ID, []*model.PropertyFieldOption{
			{Name: "added after removal"},
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.options.no_permission.app_error")
	})

	t.Run("channel admin can still patch a field whose creator was removed", func(t *testing.T) {
		newName := model.NewId()
		patched, resp, err := channelAdminClient.PatchPropertyField(context.Background(), group.Name, "post", adminField.ID, &model.PropertyFieldPatch{Name: &newName})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, patched.Name)
	})
}

func TestPatchPropertyValuesPostCreator(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.PostAttributes = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_values_creator", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	creatorLevel := model.PermissionLevelCreator
	memberLevel := model.PermissionLevelMember
	sysadminLevel := model.PermissionLevelSysadmin

	creatorField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &sysadminLevel,
		PermissionValues:  &creatorLevel,
		PermissionOptions: &sysadminLevel,
	}, false, "")
	require.Nil(t, appErr)

	memberField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &sysadminLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &sysadminLevel,
	}, false, "")
	require.Nil(t, appErr)

	// th.BasicPost is authored by the team admin (InitBasic creates it before
	// LoginBasic), who would pass through the admin arm and so prove nothing
	// about the creator arm. Author an explicit post as BasicUser, who is a
	// plain channel_user here. Both BasicUser and BasicUser2 are members of
	// BasicChannel, so denials land on the creator check rather than on
	// hasTargetAccess.
	ownPost, _, appErr := th.App.CreatePost(th.Context, &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "authored by BasicUser",
	}, th.BasicChannel, model.CreatePostFlags{})
	require.Nil(t, appErr)

	// A post BasicUser authored in a channel they have since left. They remain
	// its creator forever, but lose create_post on the channel — so the outer
	// hasTargetAccess gate rejects before the creator check is ever consulted.
	leftChannel := th.CreatePublicChannel(t)
	_, appErr = th.App.AddUserToChannel(th.Context, th.BasicUser, leftChannel, false)
	require.Nil(t, appErr)
	orphanPost, _, appErr := th.App.CreatePost(th.Context, &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: leftChannel.Id,
		Message:   "authored then left",
	}, leftChannel, model.CreatePostFlags{})
	require.Nil(t, appErr)
	appErr = th.App.RemoveUserFromChannel(th.Context, th.BasicUser.Id, "", leftChannel)
	require.Nil(t, appErr)

	channelAdmin := th.CreateUser(t)
	th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
	_, appErr = th.App.AddUserToChannel(th.Context, channelAdmin, th.BasicChannel, false)
	require.Nil(t, appErr)
	_, appErr = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, channelAdmin.Id,
		model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
	require.Nil(t, appErr)

	teamAdminInChannel := th.CreateUser(t)
	th.LinkUserToTeam(t, teamAdminInChannel, th.BasicTeam)
	_, appErr = th.App.AddUserToChannel(th.Context, teamAdminInChannel, th.BasicChannel, false)
	require.Nil(t, appErr)
	_, appErr = th.App.UpdateTeamMemberRoles(th.Context, th.BasicTeam.Id, teamAdminInChannel.Id,
		model.TeamUserRoleId+" "+model.TeamAdminRoleId)
	require.Nil(t, appErr)

	teamAdminOutsideChannel := th.CreateUser(t)
	th.LinkUserToTeam(t, teamAdminOutsideChannel, th.BasicTeam)
	_, appErr = th.App.UpdateTeamMemberRoles(th.Context, th.BasicTeam.Id, teamAdminOutsideChannel.Id,
		model.TeamUserRoleId+" "+model.TeamAdminRoleId)
	require.Nil(t, appErr)

	// One client per caller so the table can name the caller directly instead
	// of switching a shared client's session between cases.
	login := func(u *model.User) *model.Client4 {
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), u.Email, u.Password)
		require.NoError(t, err)
		return client
	}

	authorClient := login(th.BasicUser)
	nonAuthorClient := login(th.BasicUser2)
	channelAdminClient := login(channelAdmin)
	teamAdminInChannelClient := login(teamAdminInChannel)
	teamAdminOutsideChannelClient := login(teamAdminOutsideChannel)

	creatorItem := []model.PropertyValuePatchItem{
		{FieldID: creatorField.ID, Value: json.RawMessage(`"tagged"`)},
	}

	testCases := []struct {
		name     string
		client   *model.Client4
		targetID string
		items    []model.PropertyValuePatchItem
		allowed  bool
		errorID  string
	}{
		{
			name:     "author can set values on their own post",
			client:   authorClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  true,
		},
		{
			// Specifically the inner permission gate, not hasTargetAccess:
			// BasicUser2 is a channel member and may post there.
			name:     "channel member who did not author the post is rejected by the values check",
			client:   nonAuthorClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  false,
			errorID:  "api.property_value.patch.no_values_permission.app_error",
		},
		{
			name:     "channel admin can set values on someone else's post",
			client:   channelAdminClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  true,
		},
		{
			name:     "team admin who is a channel member can set values on someone else's post",
			client:   teamAdminInChannelClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  true,
		},
		{
			// Rejected by hasTargetAccess, not by the creator check: without
			// channel membership there is no create_post on the channel. The
			// error id is what distinguishes the two gates.
			name:     "team admin who is not a channel member is rejected by the outer target gate",
			client:   teamAdminOutsideChannelClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  false,
			errorID:  "api.context.permissions.app_error",
		},
		{
			name:     "system admin can set values on someone else's post",
			client:   th.SystemAdminClient,
			targetID: ownPost.Id,
			items:    creatorItem,
			allowed:  true,
		},
		{
			// creator narrows within hasTargetAccess, it does not replace it.
			// No errorID: this is the outer gate's permission error, whose id
			// depends on the channel type's write permission.
			name:     "author who left the channel is rejected by the outer target gate",
			client:   authorClient,
			targetID: orphanPost.Id,
			items:    creatorItem,
			allowed:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values, resp, err := tc.client.PatchPropertyValues(context.Background(), group.Name, "post", tc.targetID, tc.items)

			if !tc.allowed {
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
				if tc.errorID != "" {
					CheckErrorID(t, err, tc.errorID)
				}
				return
			}

			require.NoError(t, err)
			CheckOKStatus(t, resp)
			require.Len(t, values, len(tc.items))
		})
	}

	// Kept out of the table above: this asserts the absence of a side effect
	// rather than a permission outcome, and it issues its own patch so it still
	// holds when run in isolation.
	t.Run("a denied item rejects the whole batch and writes nothing", func(t *testing.T) {
		// The handler checks every item before any upsert, so a batch mixing an
		// allowed member field with a denied creator field must not persist the
		// allowed half.
		items := []model.PropertyValuePatchItem{
			{FieldID: memberField.ID, Value: json.RawMessage(`"allowed"`)},
			{FieldID: creatorField.ID, Value: json.RawMessage(`"denied"`)},
		}
		_, resp, err := nonAuthorClient.PatchPropertyValues(context.Background(), group.Name, "post", ownPost.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_value.patch.no_values_permission.app_error")

		values, resp, err := authorClient.GetPropertyValues(context.Background(), group.Name, "post", ownPost.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		for _, v := range values {
			require.NotEqual(t, memberField.ID, v.FieldID, "the allowed item of the rejected batch must not have been written")
		}
	})

	t.Run("guest who authored the post can set values on it", func(t *testing.T) {
		prevLicense := th.App.Srv().License()
		prevGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
		t.Cleanup(func() {
			th.App.Srv().SetLicense(prevLicense)
			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = prevGuestAccounts })
		})
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
		th.App.Srv().SetLicense(model.NewTestLicense())

		id := model.NewId()
		guestPassword := model.NewTestPassword()
		guest, appErr := th.App.CreateGuest(th.Context, &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      guestPassword,
			EmailVerified: true,
		})
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, guest, th.BasicTeam)
		th.AddUserToChannel(t, guest, th.BasicChannel)
		require.True(t, guest.IsGuest(), "the fixture must be a guest, or this case proves nothing")

		guestPost, _, appErr := th.App.CreatePost(th.Context, &model.Post{
			UserId:    guest.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "authored by the guest",
		}, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)

		guestClient := th.CreateClient()
		_, _, loginErr := guestClient.Login(context.Background(), guest.Email, guestPassword)
		require.NoError(t, loginErr)

		values, resp, err := guestClient.PatchPropertyValues(context.Background(), group.Name, "post", guestPost.Id, creatorItem)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, len(creatorItem))

		// The grant is authorship, not guest-ness. ownPost is authored by
		// BasicUser in the same channel, so this denial lands on the inner
		// values check rather than on channel access — the error id is what
		// separates the two gates.
		_, resp, err = guestClient.PatchPropertyValues(context.Background(), group.Name, "post", ownPost.Id, creatorItem)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.property_value.patch.no_values_permission.app_error")
	})
}

func TestPatchPropertyValues(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_values_patch", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember
	sysadminLevel := model.PermissionLevelSysadmin
	noneLevel := model.PermissionLevelNone

	// Create fields with different permission levels
	memberField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdMemberField, appErr := th.App.CreatePropertyField(th.Context, memberField, false, "")
	require.Nil(t, appErr)

	adminField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &sysadminLevel,
		PermissionValues:  &sysadminLevel,
		PermissionOptions: &sysadminLevel,
	}
	createdAdminField, appErr := th.App.CreatePropertyField(th.Context, adminField, false, "")
	require.Nil(t, appErr)

	noneField := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "post",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &noneLevel,
		PermissionOptions: &memberLevel,
	}
	createdNoneField, appErr := th.App.CreatePropertyField(th.Context, noneField, false, "")
	require.Nil(t, appErr)

	// Use a real post as the target so target access checks pass.
	targetID := th.BasicPost.Id

	t.Run("unauthenticated request should fail", func(t *testing.T) {
		client := model.NewAPIv4Client(th.Client.URL)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	// A system target defaults every slot to sysadmin, values included, so a
	// field created with omitted levels is sysadmin-only end to end. Callers
	// that want members to write values must pin permission_values themselves.
	t.Run("member can set values on a system-scoped field created with default levels", func(t *testing.T) {
		th.LoginBasic(t)

		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "system",
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, created.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)
		require.NotNil(t, created.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *created.PermissionValues)

		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, []model.PropertyValuePatchItem{
			{FieldID: created.ID, Value: json.RawMessage(`"annotated"`)},
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)

		// A caller that does want the values locked down pins it explicitly,
		// and that pin is honoured over the default.
		sysadminValues, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", &model.PropertyField{
			Name:             model.NewId(),
			Type:             model.PropertyFieldTypeText,
			TargetType:       "system",
			PermissionValues: &sysadminLevel,
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, model.PermissionLevelSysadmin, *sysadminValues.PermissionValues)

		_, resp, err = th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, []model.PropertyValuePatchItem{
			{FieldID: sysadminValues.ID, Value: json.RawMessage(`"annotated"`)},
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("member can set values on field with values permission member", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"hello"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, createdMemberField.ID, values[0].FieldID)
		require.Equal(t, json.RawMessage(`"hello"`), values[0].Value)
	})

	t.Run("websocket event should be fired on values update", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"ws-test"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyValuesUpdated {
					require.Equal(t, "post", event.GetData()["object_type"])
					require.Equal(t, targetID, event.GetData()["target_id"])
					// Post target: broadcast should be to the post's channel
					require.Equal(t, th.BasicPost.ChannelId, event.GetBroadcast().ChannelId)
					// values should be a JSON string
					valuesStr, ok := event.GetData()["values"].(string)
					require.True(t, ok)
					var values []*model.PropertyValue
					require.NoError(t, json.Unmarshal([]byte(valuesStr), &values))
					require.Len(t, values, 1)
					require.Equal(t, createdMemberField.ID, values[0].FieldID)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("non-admin cannot set values on field with values permission sysadmin", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdAdminField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("admin can set values on field with values permission sysadmin", func(t *testing.T) {
		th.LoginSystemAdmin(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdAdminField.ID, Value: json.RawMessage(`"admin-value"`)},
		}
		values, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"admin-value"`), values[0].Value)
	})

	t.Run("values permission none blocks everyone", func(t *testing.T) {
		th.LoginSystemAdmin(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdNoneField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("batch update with mixed permissions fails all-or-nothing", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"allowed"`)},
			{FieldID: createdAdminField.ID, Value: json.RawMessage(`"denied"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("empty body should fail", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("invalid field_id should fail", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: "not-valid", Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("field from different group should fail", func(t *testing.T) {
		th.LoginBasic(t)

		otherGroup, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_values_patch_other", Version: model.PropertyGroupVersionV2})
		require.Nil(t, err)

		otherField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           otherGroup.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdOtherField, appErr := th.App.CreatePropertyField(th.Context, otherField, false, "")
		require.Nil(t, appErr)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdOtherField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, patchErr)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("nonexistent group should fail", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), "nonexistent_group", "post", targetID, items)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("field with mismatched object type should fail 404", func(t *testing.T) {
		// A field in the same group but scoped to a different ObjectType must not
		// be patchable through the URL of a peer ObjectType; the mismatch collapses
		// to 404 so callers cannot distinguish "no such field" from "field exists
		// but in a different object-type bucket".
		userField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdUserField, appErr := th.App.CreatePropertyField(th.Context, userField, false, "")
		require.Nil(t, appErr)

		th.LoginSystemAdmin(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdUserField.ID, Value: json.RawMessage(`"test"`)},
		}
		_, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
		require.Equal(t, "api.property_field.object_type_mismatch.app_error", err.(*model.AppError).Id)
	})

	t.Run("channel member can set values on channel-scoped field with values permission member", func(t *testing.T) {
		th.LoginBasic(t)

		// Create a channel-scoped field with member values permission
		channelField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdChannelField, appErr := th.App.CreatePropertyField(th.Context, channelField, false, "")
		require.Nil(t, appErr)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdChannelField.ID, Value: json.RawMessage(`"channel-value"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"channel-value"`), values[0].Value)
	})

	t.Run("non-member cannot set values on post in a channel they don't belong to", func(t *testing.T) {
		privateChannel, chanErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        model.ChannelTypePrivate,
			Name:        model.NewId(),
			DisplayName: "Private Channel",
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, chanErr)

		// Create a post in the private channel as SystemAdmin (BasicUser is not a member).
		privatePost := th.CreatePostWithClient(t, th.SystemAdminClient, privateChannel)

		th.LoginBasic(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", privatePost.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("upsert updates existing value", func(t *testing.T) {
		th.LoginBasic(t)

		// Set initial value
		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"initial"`)},
		}
		_, _, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)

		// Update to new value
		items[0].Value = json.RawMessage(`"updated"`)
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"updated"`), values[0].Value)

		// Verify via GET
		got, _, err := th.Client.GetPropertyValues(context.Background(), group.Name, "post", targetID, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		found := false
		for _, v := range got {
			if v.FieldID == createdMemberField.ID {
				found = true
				require.Equal(t, json.RawMessage(`"updated"`), v.Value)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("returned value should have all fields correctly set", func(t *testing.T) {
		th.LoginBasic(t)

		valueTargetPost := th.CreatePost(t)
		valueTargetID := valueTargetPost.Id
		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"test-fields"`)},
		}

		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", valueTargetID, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)

		v := values[0]
		require.NotEmpty(t, v.ID, "ID should be set")
		require.True(t, model.IsValidId(v.ID), "ID should be a valid ID")
		require.Equal(t, valueTargetID, v.TargetID)
		require.Equal(t, "post", v.TargetType)
		require.Equal(t, group.ID, v.GroupID)
		require.Equal(t, createdMemberField.ID, v.FieldID)
		require.Equal(t, json.RawMessage(`"test-fields"`), v.Value)
		require.NotZero(t, v.CreateAt, "CreateAt should be set")
		require.NotZero(t, v.UpdateAt, "UpdateAt should be set")
		require.Equal(t, v.CreateAt, v.UpdateAt, "CreateAt and UpdateAt should be equal on first insert")
		require.Equal(t, int64(0), v.DeleteAt, "DeleteAt should be zero")
		require.Equal(t, th.BasicUser.Id, v.CreatedBy)
		require.Equal(t, th.BasicUser.Id, v.UpdatedBy)
	})

	t.Run("upsert should update timestamps and updatedBy correctly", func(t *testing.T) {
		// Create initial value as BasicUser
		th.LoginBasic(t)

		upsertTargetPost := th.CreatePost(t)
		upsertTargetID := upsertTargetPost.Id
		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"first"`)},
		}
		created, _, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", upsertTargetID, items)
		require.NoError(t, err)
		require.Len(t, created, 1)

		originalID := created[0].ID
		originalCreateAt := created[0].CreateAt
		originalUpdatedBy := created[0].UpdatedBy

		require.Equal(t, th.BasicUser.Id, originalUpdatedBy)

		// Update the same value as SystemAdmin
		th.LoginSystemAdmin(t)
		items[0].Value = json.RawMessage(`"second"`)
		updated, _, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "post", upsertTargetID, items)
		require.NoError(t, err)
		require.Len(t, updated, 1)

		u := updated[0]
		// ID should be the same (upsert, not new insert)
		require.Equal(t, originalID, u.ID, "ID should not change on upsert")
		// CreateAt should not change
		require.Equal(t, originalCreateAt, u.CreateAt, "CreateAt should not change on upsert")
		// UpdateAt should be >= CreateAt
		require.GreaterOrEqual(t, u.UpdateAt, u.CreateAt, "UpdateAt should be >= CreateAt after update")
		// UpdatedBy should reflect the new user
		require.Equal(t, th.SystemAdminUser.Id, u.UpdatedBy, "UpdatedBy should be the user who performed the update")
		// Value should be updated
		require.Equal(t, json.RawMessage(`"second"`), u.Value)
	})

	t.Run("duplicate field IDs should fail", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"first"`)},
			{FieldID: createdMemberField.ID, Value: json.RawMessage(`"second"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("v3 group with flag off falls back to v2 and returns 200", func(t *testing.T) {
		v3Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_patch_values", Version: model.PropertyGroupVersionV3})
		require.Nil(t, appErr)
		require.NotNil(t, v3Group)

		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           v3Group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"test"`)},
		}

		values, resp, err := th.Client.PatchPropertyValues(context.Background(), v3Group.Name, "post", th.BasicPost.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("v1 group should return 404", func(t *testing.T) {
		v1Group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v1_patch_values", Version: model.PropertyGroupVersionV1})
		require.Nil(t, appErr)
		require.NotNil(t, v1Group)

		items := []model.PropertyValuePatchItem{
			{FieldID: model.NewId(), Value: json.RawMessage(`"test"`)},
		}

		_, resp, err := th.Client.PatchPropertyValues(context.Background(), v1Group.Name, "post", th.BasicPost.Id, items)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})
}

func TestGetPropertyValuesUserTargetAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_user_get_access", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	// Create a user-scoped field
	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "user",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)

	// Create a value for BasicUser
	_, appErr = th.App.UpsertPropertyValues(th.Context, []*model.PropertyValue{{
		TargetID:   th.BasicUser.Id,
		TargetType: "user",
		GroupID:    group.ID,
		FieldID:    createdField.ID,
		Value:      json.RawMessage(`"my-value"`),
		CreatedBy:  th.BasicUser.Id,
		UpdatedBy:  th.BasicUser.Id,
	}}, "", "", "")
	require.Nil(t, appErr)

	t.Run("user can get their own property values", func(t *testing.T) {
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("non-admin can get another user's property values", func(t *testing.T) {
		th.LoginBasic2(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("admin can get another user's property values", func(t *testing.T) {
		th.LoginSystemAdmin(t)

		values, resp, err := th.SystemAdminClient.GetPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("non-admin cannot get values of a user they cannot see", func(t *testing.T) {
		// Strip system-wide view_members so UserCanSeeOtherUser falls back to team/channel membership.
		th.RemovePermissionFromRole(t, model.PermissionViewMembers.Id, model.SystemUserRoleId)
		defer th.AddPermissionToRole(t, model.PermissionViewMembers.Id, model.SystemUserRoleId)

		// Drop BasicUser2 from BasicTeam so they no longer share a team with BasicUser.
		resp, err := th.SystemAdminClient.RemoveTeamMember(context.Background(), th.BasicTeam.Id, th.BasicUser2.Id)
		CheckOKStatus(t, resp)
		require.NoError(t, err)

		th.LoginBasic2(t)

		_, resp, err = th.Client.GetPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, model.PropertyValueSearch{PerPage: 60})
		CheckForbiddenStatus(t, resp)
		require.Error(t, err)
	})
}

func TestPatchPropertyValuesUserTargetAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_user_patch_access", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	// Create a user-scoped field
	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "user",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)

	t.Run("user can set their own property values", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"self-value"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"self-value"`), values[0].Value)
	})

	t.Run("non-admin cannot set another user's property values", func(t *testing.T) {
		th.LoginBasic2(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("admin can set another user's property values", func(t *testing.T) {
		th.LoginSystemAdmin(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"admin-set"`)},
		}
		values, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"admin-set"`), values[0].Value)
	})
}

func TestPatchPropertyValuesBotTargetAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

	defaultPerms := th.SaveDefaultRolePermissions(t)
	defer th.RestoreDefaultRolePermissions(t, defaultPerms)
	withPerms := func(t *testing.T, perms ...*model.Permission) {
		t.Helper()
		th.RestoreDefaultRolePermissions(t, defaultPerms)
		for _, p := range perms {
			th.AddPermissionToRole(t, p.Id, model.SystemUserRoleId)
		}
	}

	bot, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: GenerateTestUsername(), OwnerId: th.BasicUser.Id})
	require.Nil(t, appErr)

	cpaField, resp, err := th.SystemAdminClient.CreateCPAField(context.Background(), &model.PropertyField{
		Name: celSafeName(),
		Type: model.PropertyFieldTypeText,
	})
	CheckCreatedStatus(t, resp)
	require.NoError(t, err)

	otherGroup, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_bot_other_group", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)
	memberLevel := model.PermissionLevelMember
	otherField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           otherGroup.ID,
		ObjectType:        model.PropertyFieldObjectTypeUser,
		TargetType:        string(model.PropertyFieldTargetLevelSystem),
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}, false, "")
	require.Nil(t, appErr)

	cpaItems := []model.PropertyValuePatchItem{{FieldID: cpaField.ID, Value: json.RawMessage(`"bot-value"`)}}
	otherItems := []model.PropertyValuePatchItem{{FieldID: otherField.ID, Value: json.RawMessage(`"other-value"`)}}

	th.LoginBasic(t)

	t.Run("bot manager writes own bot in access_control", func(t *testing.T) {
		withPerms(t, model.PermissionManageBots)
		upserted, resp, err := th.Client.PatchPropertyValues(context.Background(), model.AccessControlPropertyGroupName, model.PropertyFieldObjectTypeUser, bot.UserId, cpaItems)
		CheckOKStatus(t, resp)
		require.NoError(t, err)
		require.Len(t, upserted, 1)
		require.Equal(t, bot.UserId, upserted[0].TargetID)
		require.Equal(t, json.RawMessage(`"bot-value"`), upserted[0].Value)

		stored, resp, err := th.SystemAdminClient.ListCPAValues(context.Background(), bot.UserId)
		CheckOKStatus(t, resp)
		require.NoError(t, err)
		require.Equal(t, json.RawMessage(`"bot-value"`), stored[cpaField.ID])
	})

	t.Run("bot manager cannot write own bot in another group", func(t *testing.T) {
		withPerms(t, model.PermissionManageBots)
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), otherGroup.Name, model.PropertyFieldObjectTypeUser, bot.UserId, otherItems)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("other group still open to admins", func(t *testing.T) {
		withPerms(t)
		upserted, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), otherGroup.Name, model.PropertyFieldObjectTypeUser, bot.UserId, otherItems)
		CheckOKStatus(t, resp)
		require.NoError(t, err)
		require.Len(t, upserted, 1)
		require.Equal(t, json.RawMessage(`"other-value"`), upserted[0].Value)
	})

	t.Run("bot permissions don't open non-bot targets", func(t *testing.T) {
		withPerms(t, model.PermissionManageBots, model.PermissionManageOthersBots)
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), model.AccessControlPropertyGroupName, model.PropertyFieldObjectTypeUser, th.BasicUser2.Id, cpaItems)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("nonexistent target", func(t *testing.T) {
		withPerms(t, model.PermissionManageBots, model.PermissionManageOthersBots)
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), model.AccessControlPropertyGroupName, model.PropertyFieldObjectTypeUser, model.NewId(), cpaItems)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})

	t.Run("edit_other_users cannot write a system admin in another group", func(t *testing.T) {
		withPerms(t, model.PermissionEditOtherUsers)
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), otherGroup.Name, model.PropertyFieldObjectTypeUser, th.SystemAdminUser.Id, otherItems)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.context.permissions.app_error")
	})
}

func TestGetPropertyValuesChannelTargetAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_chan_get_access", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)

	memberLevel := model.PermissionLevelMember

	createFieldAndValue := func(t *testing.T, channelID string) {
		t.Helper()
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)

		_, appErr = th.App.UpsertPropertyValues(th.Context, []*model.PropertyValue{{
			TargetID:   channelID,
			TargetType: "channel",
			GroupID:    group.ID,
			FieldID:    createdField.ID,
			Value:      json.RawMessage(`"val"`),
			CreatedBy:  th.BasicUser.Id,
			UpdatedBy:  th.BasicUser.Id,
		}}, "", "", "")
		require.Nil(t, appErr)
	}

	// Create a non-member user
	nonMember := th.CreateUser(t)
	nonMemberClient := th.CreateClient()
	_, _, err := nonMemberClient.Login(context.Background(), nonMember.Email, nonMember.Password)
	require.NoError(t, err)

	t.Run("public channel - member can read", func(t *testing.T) {
		createFieldAndValue(t, th.BasicChannel.Id)
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("public channel - non-member cannot read", func(t *testing.T) {
		_, resp, err := nonMemberClient.GetPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("private channel - member can read", func(t *testing.T) {
		createFieldAndValue(t, th.BasicPrivateChannel.Id)
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "channel", th.BasicPrivateChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("private channel - non-member cannot read", func(t *testing.T) {
		_, resp, err := nonMemberClient.GetPropertyValues(context.Background(), group.Name, "channel", th.BasicPrivateChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("DM channel - participant can read", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		createFieldAndValue(t, dmChannel.Id)
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("DM channel - non-participant cannot read", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		createFieldAndValue(t, dmChannel.Id)

		_, resp, err := nonMemberClient.GetPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("GM channel - participant can read", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		createFieldAndValue(t, gmChannel.Id)
		th.LoginBasic(t)

		values, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "channel", gmChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, values)
	})

	t.Run("GM channel - non-participant cannot read", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		createFieldAndValue(t, gmChannel.Id)

		_, resp, err := nonMemberClient.GetPropertyValues(context.Background(), group.Name, "channel", gmChannel.Id, model.PropertyValueSearch{PerPage: 60})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestPatchPropertyValuesChannelTargetAccess(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_chan_patch_access", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)

	memberLevel := model.PermissionLevelMember

	createField := func(t *testing.T) *model.PropertyField {
		t.Helper()
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)
		return createdField
	}

	createAdminField := func(t *testing.T) *model.PropertyField {
		t.Helper()
		adminLevel := model.PermissionLevelAdmin
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		}
		createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
		require.Nil(t, appErr)
		return createdField
	}

	// Create a non-member user
	nonMember := th.CreateUser(t)
	nonMemberClient := th.CreateClient()
	_, _, err := nonMemberClient.Login(context.Background(), nonMember.Email, nonMember.Password)
	require.NoError(t, err)

	t.Run("public channel - member with manage permission can write", func(t *testing.T) {
		f := createField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"pub-val"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("public channel - non-member cannot write", func(t *testing.T) {
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := nonMemberClient.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("private channel - member with manage permission can write", func(t *testing.T) {
		f := createField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"priv-val"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicPrivateChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("private channel - non-member cannot write", func(t *testing.T) {
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := nonMemberClient.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicPrivateChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("DM channel - participant can write member-tier value", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"dm-val"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("DM channel - participant can write admin-tier value", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createAdminField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"dm-admin-val"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("DM channel - system admin can write member-tier value", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"dm-sysadmin-val"`)},
		}
		values, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("DM channel - participant can still read values", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"dm-readable"`)},
		}
		_, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		th.LoginBasic(t)
		read, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, model.PropertyValueSearch{})
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// The DM is shared across the subtests here, so assert on this field only.
		var found *model.PropertyValue
		for _, v := range read {
			if v.FieldID == f.ID {
				found = v
			}
		}
		require.NotNil(t, found)
		require.Equal(t, json.RawMessage(`"dm-readable"`), found.Value)
	})

	t.Run("DM channel - non-participant cannot write", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := nonMemberClient.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("DM channel - system admin non-participant can write admin-tier value", func(t *testing.T) {
		dmChannel := th.CreateDmChannel(t, th.BasicUser2)
		f := createAdminField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"dm-admin-val"`)},
		}
		values, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "channel", dmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("GM channel - participant can write member-tier value", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		f := createField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"gm-val"`)},
		}
		values, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", gmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})

	t.Run("GM channel - non-participant cannot write", func(t *testing.T) {
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		f := createField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"should-fail"`)},
		}
		_, resp, err := nonMemberClient.PatchPropertyValues(context.Background(), group.Name, "channel", gmChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("GM channel - system admin non-participant can write admin-tier value", func(t *testing.T) {
		thirdUser := th.CreateUser(t)
		gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, thirdUser.Id}, th.BasicUser.Id)
		require.Nil(t, appErr)
		f := createAdminField(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"gm-admin-val"`)},
		}
		values, resp, err := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, "channel", gmChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})
}

// TestPatchPropertyValuesChannelAdminTier covers the two independent gates an
// admin-tier channel value write has to clear: the outer per-channel
// "may write properties here" check (manage_*_channel_properties, a channel_user
// grant) and the inner per-field tier (manage_channel_roles, a channel_admin
// grant that team_admin also carries). Clearing one never implies the other.
func TestPatchPropertyValuesChannelAdminTier(t *testing.T) {
	mainHelper.Parallel(t)
	// ChannelAttributes is the only enabled gate flag, so this also asserts it
	// opens the channel *value* routes and not just the field routes.
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = false
		cfg.FeatureFlags.ManagedChannelCategories = false
		cfg.FeatureFlags.ClassificationMarkings = false
		cfg.FeatureFlags.SessionAttributes = false
		cfg.FeatureFlags.PostAttributes = false
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_chan_admin_tier", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)

	adminLevel := model.PermissionLevelAdmin

	createAdminField := func(t *testing.T) *model.PropertyField {
		t.Helper()
		createdField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &adminLevel,
			PermissionValues:  &adminLevel,
			PermissionOptions: &adminLevel,
		}, false, "")
		require.Nil(t, appErr)
		return createdField
	}

	newChannel := func(t *testing.T, channelType model.ChannelType) *model.Channel {
		t.Helper()
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			TeamId:      th.BasicTeam.Id,
			Type:        channelType,
			Name:        model.NewId(),
			DisplayName: "Channel Attributes Test",
			CreatorId:   th.SystemAdminUser.Id,
		}, false)
		require.Nil(t, appErr)
		return channel
	}

	loginNewClient := func(t *testing.T, user *model.User) *model.Client4 {
		t.Helper()
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)
		return client
	}

	t.Run("plain channel member cannot write an admin-tier value", func(t *testing.T) {
		f := createAdminField(t)
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"member-attempt"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel admin can write an admin-tier value", func(t *testing.T) {
		channel := newChannel(t, model.ChannelTypeOpen)
		channelAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
		th.AddUserToChannel(t, channelAdmin, channel)
		th.MakeUserChannelAdmin(t, channelAdmin, channel)
		client := loginNewClient(t, channelAdmin)

		f := createAdminField(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"channel-admin-val"`)},
		}
		values, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "channel", channel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"channel-admin-val"`), values[0].Value)
	})

	t.Run("team admin inherits the admin tier inside a channel they belong to", func(t *testing.T) {
		channel := newChannel(t, model.ChannelTypeOpen)
		teamAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, teamAdmin, th.BasicTeam)
		th.UpdateUserToTeamAdmin(t, teamAdmin, th.BasicTeam)
		member := th.AddUserToChannel(t, teamAdmin, channel)
		// The admin tier has to come from the team role: this membership is a
		// plain channel_user, so channel_admin cannot be the reason it passes.
		require.False(t, member.SchemeAdmin)
		client := loginNewClient(t, teamAdmin)

		f := createAdminField(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"team-admin-val"`)},
		}
		values, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "channel", channel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
		require.Equal(t, json.RawMessage(`"team-admin-val"`), values[0].Value)
	})

	t.Run("team admin outside the channel is stopped by the outer property-write gate", func(t *testing.T) {
		channel := newChannel(t, model.ChannelTypePrivate)
		teamAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, teamAdmin, th.BasicTeam)
		th.UpdateUserToTeamAdmin(t, teamAdmin, th.BasicTeam)
		client := loginNewClient(t, teamAdmin)

		f := createAdminField(t)

		// Inner tier is satisfied without membership: manage_channel_roles
		// cascades from the team role.
		require.True(t, th.App.HasPermissionToSetPropertyFieldValues(th.Context, teamAdmin.Id, f, channel.Id))

		// The outer gate is independent and still rejects the write:
		// manage_private_channel_properties is a channel_user grant that
		// team_admin does not carry.
		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"outside-attempt"`)},
		}
		_, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "channel", channel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("channel admin of one channel cannot write on another", func(t *testing.T) {
		ownChannel := newChannel(t, model.ChannelTypeOpen)
		otherChannel := newChannel(t, model.ChannelTypeOpen)

		channelAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
		th.AddUserToChannel(t, channelAdmin, ownChannel)
		th.MakeUserChannelAdmin(t, channelAdmin, ownChannel)
		member := th.AddUserToChannel(t, channelAdmin, otherChannel)
		require.False(t, member.SchemeAdmin)
		client := loginNewClient(t, channelAdmin)

		f := createAdminField(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: f.ID, Value: json.RawMessage(`"cross-channel-attempt"`)},
		}

		// The tier is evaluated against the target channel, so channel_admin
		// elsewhere grants nothing here.
		_, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "channel", otherChannel.Id, items)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		values, resp, err := client.PatchPropertyValues(context.Background(), group.Name, "channel", ownChannel.Id, items)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Len(t, values, 1)
	})
}

func TestCreatePropertyFieldTeamScopedBroadcast(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_team_broadcast", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	t.Run("team-scoped field broadcast has TeamId set and ChannelId empty", func(t *testing.T) {
		// Connect websocket as BasicUser (who is a member of BasicTeam)
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		// Create the field as SystemAdmin (has ManageTeam permission)
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: "team",
			TargetID:   th.BasicTeam.Id,
		}

		createdField, resp, createErr := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "post", field)
		require.NoError(t, createErr)
		CheckCreatedStatus(t, resp)

		var receivedField model.PropertyField
		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyFieldCreated {
					fieldData, ok := event.GetData()["property_field"].(string)
					require.True(t, ok)
					require.NoError(t, json.Unmarshal([]byte(fieldData), &receivedField))
					require.Equal(t, "post", event.GetData()["object_type"])
					// Team-scoped: TeamId should be set, ChannelId should be empty
					require.Equal(t, th.BasicTeam.Id, event.GetBroadcast().TeamId)
					require.Empty(t, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, createdField.ID, receivedField.ID)
		require.Equal(t, createdField.Name, receivedField.Name)
	})
}

// nextPropertyValuesEvent drains the websocket stream until the next
// property_values_updated event arrives, ignoring unrelated traffic.
func nextPropertyValuesEvent(t *testing.T, wsClient *model.WebSocketClient) *model.WebSocketEvent {
	t.Helper()

	deadline := time.After(10 * time.Second)
	for {
		select {
		case event, ok := <-wsClient.EventChannel:
			require.True(t, ok, "websocket event channel closed while waiting for property_values_updated")
			if event.EventType() == model.WebsocketEventPropertyValuesUpdated {
				return event
			}
		case <-deadline:
			require.FailNow(t, "timed out waiting for property_values_updated")
		}
	}
}

func decodeBroadcastValues(t *testing.T, event *model.WebSocketEvent) []*model.PropertyValue {
	t.Helper()

	raw, ok := event.GetData()["values"].(string)
	require.True(t, ok, "event carries no values payload")

	var values []*model.PropertyValue
	require.NoError(t, json.Unmarshal([]byte(raw), &values))
	return values
}

// Locks the four payloads the server emits under the single
// property_values_updated event. The webapp tells an upsert from each deletion
// path purely by which keys are present, so a silent change to any shape breaks
// client cache invalidation without failing any other test:
//
//  1. upsert (incl. PATCH value:null)  object_type + target_id + full values array
//  2. single delete                    object_type + target_id + one tombstone, ID empty
//  3. delete for target                object_type + target_id + values "[]"
//  4. delete for field                 field_id only, no object_type/target_id, values "[]"
func TestPropertyValuesUpdatedPayloadShapes(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_value_payload_shapes", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)

	memberLevel := model.PermissionLevelMember
	newChannelField := func(t *testing.T) *model.PropertyField {
		t.Helper()
		field, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}, false, "")
		require.Nil(t, fieldErr)
		return field
	}

	th.LoginBasic(t)
	webSocketClient := th.CreateConnectedWebSocketClient(t)

	field := newChannelField(t)
	var valueID string

	t.Run("shape 1: upsert carries object_type, target_id, and the full values array", func(t *testing.T) {
		items := []model.PropertyValuePatchItem{
			{FieldID: field.ID, Value: json.RawMessage(`"aurora"`)},
		}
		upserted, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)
		require.Len(t, upserted, 1)
		valueID = upserted[0].ID

		event := nextPropertyValuesEvent(t, webSocketClient)
		require.Equal(t, "channel", event.GetData()["object_type"])
		require.Equal(t, th.BasicChannel.Id, event.GetData()["target_id"])
		require.Equal(t, th.BasicChannel.Id, event.GetBroadcast().ChannelId)

		values := decodeBroadcastValues(t, event)
		require.Len(t, values, 1)
		require.Equal(t, field.ID, values[0].FieldID)
		require.Equal(t, th.BasicChannel.Id, values[0].TargetID)
		require.Equal(t, json.RawMessage(`"aurora"`), values[0].Value)
	})

	t.Run("shape 1: PATCH value:null follows the upsert path and keeps a null-valued row", func(t *testing.T) {
		// The user-initiated clear is not a deletion: the row survives holding
		// null, so client selectors must treat null as unset rather than wait
		// for a delete broadcast that never arrives.
		items := []model.PropertyValuePatchItem{
			{FieldID: field.ID, Value: json.RawMessage(`null`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		event := nextPropertyValuesEvent(t, webSocketClient)
		require.Equal(t, "channel", event.GetData()["object_type"])
		require.Equal(t, th.BasicChannel.Id, event.GetData()["target_id"])

		values := decodeBroadcastValues(t, event)
		require.Len(t, values, 1)
		require.Equal(t, field.ID, values[0].FieldID)
		require.Equal(t, json.RawMessage(`null`), values[0].Value)
		// Populated ID: this is a real row, which is what separates it from the
		// delete tombstone below.
		require.NotEmpty(t, values[0].ID)
	})

	t.Run("shape 2: single delete carries a one-element zero-valued tombstone", func(t *testing.T) {
		require.NotEmpty(t, valueID)
		require.Nil(t, th.App.DeletePropertyValue(th.Context, group.ID, valueID))

		event := nextPropertyValuesEvent(t, webSocketClient)
		require.Equal(t, "channel", event.GetData()["object_type"])
		require.Equal(t, th.BasicChannel.Id, event.GetData()["target_id"])

		values := decodeBroadcastValues(t, event)
		require.Len(t, values, 1)
		require.Equal(t, field.ID, values[0].FieldID)
		require.Equal(t, th.BasicChannel.Id, values[0].TargetID)
		require.Equal(t, "channel", values[0].TargetType)
		require.Equal(t, group.ID, values[0].GroupID)

		// The tombstone is synthesized without a Value, and a nil RawMessage
		// marshals to null — so on the wire a delete is indistinguishable from
		// the value:null clear above except for the empty ID. A client keying off
		// the value alone leaves a stale row behind instead of removing it.
		require.Equal(t, json.RawMessage(`null`), values[0].Value)
		require.Empty(t, values[0].ID)
	})

	t.Run("shape 3: delete for target carries an empty values array", func(t *testing.T) {
		targetField := newChannelField(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: targetField.ID, Value: json.RawMessage(`"noforn"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)
		nextPropertyValuesEvent(t, webSocketClient) // the upsert from the line above

		require.Nil(t, th.App.DeletePropertyValuesForTarget(th.Context, group.ID, "channel", th.BasicChannel.Id))

		event := nextPropertyValuesEvent(t, webSocketClient)
		require.Equal(t, "channel", event.GetData()["object_type"])
		require.Equal(t, th.BasicChannel.Id, event.GetData()["target_id"])
		// Empty array, not tombstones: the event's target ID is the only thing
		// identifying what to clear.
		require.Equal(t, "[]", event.GetData()["values"])
		require.Empty(t, decodeBroadcastValues(t, event))
	})

	t.Run("shape 4: delete for field carries field_id and no target", func(t *testing.T) {
		fieldScoped := newChannelField(t)
		items := []model.PropertyValuePatchItem{
			{FieldID: fieldScoped.ID, Value: json.RawMessage(`"elevated"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)
		nextPropertyValuesEvent(t, webSocketClient) // the upsert from the line above

		require.Nil(t, th.App.DeletePropertyValuesForField(th.Context, group.ID, fieldScoped.ID))

		event := nextPropertyValuesEvent(t, webSocketClient)
		require.Equal(t, fieldScoped.ID, event.GetData()["field_id"])
		require.Equal(t, "[]", event.GetData()["values"])

		// No target scoping: this shape clears one field across every target, so
		// it broadcasts system-wide rather than to a channel.
		_, hasObjectType := event.GetData()["object_type"]
		require.False(t, hasObjectType, "delete-for-field must not carry object_type")
		_, hasTargetID := event.GetData()["target_id"]
		require.False(t, hasTargetID, "delete-for-field must not carry target_id")
		require.Empty(t, event.GetBroadcast().ChannelId)
		require.Empty(t, event.GetBroadcast().TeamId)
	})

	t.Run("shape 5: a non-public field's value is withheld", func(t *testing.T) {
		withheldField := newChannelField(t)

		// Denying value.read through the store, not the API, keeps value.write
		// open to a session caller so the test can provoke a broadcast, which
		// no plugin-authored source_only field would allow.
		withheldField.Permissions.Restrictions.Value.Read = model.PermissionLevelNone
		_, err := th.App.Srv().Store().PropertyField().Update(group.ID, []*model.PropertyField{withheldField}, nil)
		require.NoError(t, err)

		items := []model.PropertyValuePatchItem{
			{FieldID: field.ID, Value: json.RawMessage(`"visible"`)},
			{FieldID: withheldField.ID, Value: json.RawMessage(`"hidden"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		event := nextPropertyValuesEvent(t, webSocketClient)
		values := decodeBroadcastValues(t, event)
		require.Len(t, values, 2)

		byFieldID := make(map[string]*model.PropertyValue, len(values))
		for _, v := range values {
			byFieldID[v.FieldID] = v
		}

		withheld := byFieldID[withheldField.ID]
		require.Equal(t, json.RawMessage(model.PropertyValueWithheldJSON), withheld.Value)
		require.NotEmpty(t, withheld.ID)
		require.Equal(t, withheldField.ID, withheld.FieldID)
		require.Equal(t, th.BasicChannel.Id, withheld.TargetID)

		// The public field in the same batch keeps its real value: masking is
		// decided per row, not per broadcast.
		public := byFieldID[field.ID]
		require.Equal(t, json.RawMessage(`"visible"`), public.Value)
	})
}

func TestPatchPropertyValuesChannelObjectTypeBroadcast(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_channel_val_broadcast", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "channel",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)

	t.Run("channel objectType broadcasts to channel members", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"chan-val"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyValuesUpdated {
					require.Equal(t, "channel", event.GetData()["object_type"])
					require.Equal(t, th.BasicChannel.Id, event.GetData()["target_id"])
					// Channel target: broadcast should be to that channel
					require.Equal(t, th.BasicChannel.Id, event.GetBroadcast().ChannelId)
					require.Empty(t, event.GetBroadcast().TeamId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestPatchPropertyValuesUserObjectTypeBroadcast(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_user_val_broadcast", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	field := &model.PropertyField{
		Name:              model.NewId(),
		Type:              model.PropertyFieldTypeText,
		GroupID:           group.ID,
		ObjectType:        "user",
		TargetType:        "system",
		PermissionField:   &memberLevel,
		PermissionValues:  &memberLevel,
		PermissionOptions: &memberLevel,
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, field, false, "")
	require.Nil(t, appErr)

	t.Run("user objectType broadcasts system-wide", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdField.ID, Value: json.RawMessage(`"user-val"`)},
		}
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "user", th.BasicUser.Id, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyValuesUpdated {
					require.Equal(t, "user", event.GetData()["object_type"])
					require.Equal(t, th.BasicUser.Id, event.GetData()["target_id"])
					// User target: broadcast should be system-wide (empty team/channel)
					require.Empty(t, event.GetBroadcast().TeamId)
					require.Empty(t, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestUpsertPropertyValuesPSAv1OptOut(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	// PSAv1 fields must live in a v1 group; the marker v2 field uses a separate v2 group.
	psav1Group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_psav1_optout_v1", Version: model.PropertyGroupVersionV1})
	require.Nil(t, err)
	v2Group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_psav1_optout_v2", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	// Create a PSAv1-style field: no ObjectType, meaning it predates
	// the websocket broadcast machinery added in PSAv2.
	psav1Field := &model.PropertyField{
		Name:       model.NewId(),
		Type:       model.PropertyFieldTypeText,
		GroupID:    psav1Group.ID,
		TargetType: "system",
	}
	createdField, appErr := th.App.CreatePropertyField(th.Context, psav1Field, false, "")
	require.Nil(t, appErr)
	require.Empty(t, createdField.ObjectType, "PSAv1 field should have no ObjectType")

	t.Run("upserting values for a PSAv1 field should not publish websocket event", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		// We call the app layer directly because the PSAv2 API endpoints
		// would reject a request without objectType. This mimics what a
		// PSAv1 custom endpoint would do: call UpsertPropertyValues with
		// empty objectType/targetID, skipping websocket broadcasting.
		values := []*model.PropertyValue{
			{
				TargetID:   th.BasicPost.Id,
				TargetType: "post",
				GroupID:    psav1Group.ID,
				FieldID:    createdField.ID,
				Value:      json.RawMessage(`"psav1-value"`),
				CreatedBy:  th.BasicUser.Id,
				UpdatedBy:  th.BasicUser.Id,
			},
		}
		result, upsertErr := th.App.UpsertPropertyValues(th.Context, values, "", "", "")
		require.Nil(t, upsertErr)
		require.Len(t, result, 1)

		// Trigger a known marker event so we can detect when we've
		// drained the websocket channel. If a property_values_updated
		// event arrives before the marker, the test fails.
		memberLevel := model.PermissionLevelMember
		markerField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           v2Group.ID,
			ObjectType:        "post",
			TargetType:        "channel",
			TargetID:          th.BasicChannel.Id,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		_, markerErr := th.App.CreatePropertyField(th.Context, markerField, false, "")
		require.Nil(t, markerErr)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				require.NotEqual(t, model.WebsocketEventPropertyValuesUpdated, event.EventType(),
					"PSAv1 opt-out should not produce a property_values_updated event")
				if event.EventType() == model.WebsocketEventPropertyFieldCreated {
					// Marker arrived, no values event was seen
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestPatchPropertyValuesMultiValuePayload(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_multi_val_payload", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	memberLevel := model.PermissionLevelMember

	// Create three fields
	var createdFields []*model.PropertyField
	for i := range 3 {
		f := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		cf, fieldErr := th.App.CreatePropertyField(th.Context, f, false, "")
		require.Nil(t, fieldErr, "failed to create field %d", i)
		createdFields = append(createdFields, cf)
	}

	t.Run("multi-value patch includes all values in websocket event", func(t *testing.T) {
		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: createdFields[0].ID, Value: json.RawMessage(`"val-0"`)},
			{FieldID: createdFields[1].ID, Value: json.RawMessage(`"val-1"`)},
			{FieldID: createdFields[2].ID, Value: json.RawMessage(`"val-2"`)},
		}

		targetID := th.BasicPost.Id
		_, resp, patchErr := th.Client.PatchPropertyValues(context.Background(), group.Name, "post", targetID, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyValuesUpdated {
					require.Equal(t, "post", event.GetData()["object_type"])
					require.Equal(t, targetID, event.GetData()["target_id"])

					valuesStr, ok := event.GetData()["values"].(string)
					require.True(t, ok)
					var values []*model.PropertyValue
					require.NoError(t, json.Unmarshal([]byte(valuesStr), &values))
					require.Len(t, values, 3, "websocket event should contain all 3 values")

					// Verify all field IDs are present
					fieldIDs := map[string]bool{}
					for _, v := range values {
						fieldIDs[v.FieldID] = true
					}
					for _, f := range createdFields {
						require.True(t, fieldIDs[f.ID], "field %s should be in the websocket event values", f.ID)
					}
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}
func TestLinkedProperties(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_linked_properties", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	sysadminLevel := model.PermissionLevelSysadmin
	memberLevel := model.PermissionLevelMember

	t.Run("create template field requires sysadmin", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			TargetType: "system",
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "template", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdField.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionField)
		require.NotNil(t, createdField.PermissionValues)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionValues)
		require.NotNil(t, createdField.PermissionOptions)
		require.Equal(t, model.PermissionLevelSysadmin, *createdField.PermissionOptions)
	})

	t.Run("create template field as non-admin fails", func(t *testing.T) {
		th.LoginBasic(t)

		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			TargetType: "system",
		}

		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, "template", field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("create linked field with valid source", func(t *testing.T) {
		// Create a template field as the source
		sourceField := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: model.PropertyFieldObjectTypeTemplate,
			TargetType: "system",
			Attrs: model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "Option A"},
					{"id": model.NewId(), "name": "Option B"},
				},
			},
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		field := &model.PropertyField{
			Name:          model.NewId(),
			Type:          model.PropertyFieldTypeSelect,
			TargetType:    "system",
			LinkedFieldID: &sourceID,
		}

		createdField, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "user", field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.Equal(t, createdSource.Type, createdField.Type)
		require.NotNil(t, createdField.Attrs)
		require.NotNil(t, createdField.Attrs["options"])
	})

	t.Run("patch linked field rejects type change", func(t *testing.T) {
		// Create source + linked field via App
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		newType := model.PropertyFieldTypeText
		patch := &model.PropertyFieldPatch{Type: &newType}

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch linked field rejects options change", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				"options": []map[string]any{
					{"id": model.NewId(), "name": "New Option"},
				},
			},
		}

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch linked field allows name change", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}

		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, newName, updatedField.Name)
	})

	t.Run("patch unlink clears LinkedFieldID", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		emptyID := ""
		patch := &model.PropertyFieldPatch{LinkedFieldID: &emptyID}

		updatedField, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Nil(t, updatedField.LinkedFieldID)
	})

	t.Run("patch existing field rejects setting LinkedFieldID", func(t *testing.T) {
		// Create a regular (non-linked) field
		regularField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		createdRegular, appErr := th.App.CreatePropertyField(th.Context, regularField, false, "")
		require.Nil(t, appErr)

		someID := model.NewId()
		patch := &model.PropertyFieldPatch{LinkedFieldID: &someID}

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdRegular.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("delete source with dependents returns 409", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		_, appErr = th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), group.Name, "template", createdSource.ID)
		require.Error(t, err)
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("get values for template returns 400", func(t *testing.T) {
		th.LoginBasic(t)

		_, resp, err := th.Client.GetPropertyValues(context.Background(), group.Name, "template", model.NewId(), model.PropertyValueSearch{PerPage: 10})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch values for template returns 400", func(t *testing.T) {
		th.LoginBasic(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: model.NewId(), Value: json.RawMessage(`"val"`)},
		}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "template", model.NewId(), items)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch source field options propagates to linked fields", func(t *testing.T) {
		// Create a template (source) with one option
		optA := map[string]any{"id": model.NewId(), "name": "PropTestA"}
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeSelect,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []any{optA},
			},
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		// Create two linked fields and keep their IDs
		sourceID := createdSource.ID
		var linkedIDs []string
		for range 2 {
			linked := &model.PropertyField{
				Name:              "PropLinked-" + model.NewId(),
				Type:              model.PropertyFieldTypeSelect,
				GroupID:           group.ID,
				ObjectType:        "user",
				TargetType:        "system",
				LinkedFieldID:     &sourceID,
				PermissionField:   &memberLevel,
				PermissionValues:  &memberLevel,
				PermissionOptions: &memberLevel,
			}
			createdLinked, lErr := th.App.CreatePropertyField(th.Context, linked, false, "")
			require.Nil(t, lErr)
			linkedIDs = append(linkedIDs, createdLinked.ID)
		}

		// Patch the source field's options via the API — add a second option
		optB := map[string]any{"id": model.NewId(), "name": "PropTestB"}
		patch := &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				model.PropertyFieldAttributeOptions: []any{optA, optB},
			},
		}
		updatedSource, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "template", createdSource.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Source should have 2 options now
		sourceOpts := updatedSource.Attrs[model.PropertyFieldAttributeOptions].([]any)
		require.Len(t, sourceOpts, 2)

		// Fetch each linked field and verify propagation
		for _, linkedID := range linkedIDs {
			lf, lfErr := th.App.GetPropertyField(th.Context, group.ID, linkedID)
			require.Nil(t, lfErr)
			opts := lf.Attrs[model.PropertyFieldAttributeOptions].([]any)
			require.Len(t, opts, 2, "linked field %s should have 2 options after propagation", linkedID)
		}
	})

	// A linked field (any object type) must NOT inherit PermissionValues from
	// its (always-sysadmin) template -- it needs to be independently settable
	// per resource. Standalone (non-linked) fields are untouched since the API
	// only accepts permission_values when LinkedFieldID is set (see
	// patchPropertyField).
	t.Run("create linked user field via API defaults PermissionValues to member, not the sysadmin template", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:          model.NewId(),
			Type:          model.PropertyFieldTypeText,
			TargetType:    "system",
			LinkedFieldID: &sourceID,
		}

		createdLinked, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "user", linkedField)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdLinked.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdLinked.PermissionValues)
		// PermissionField/PermissionOptions still inherit from the template unchanged.
		require.NotNil(t, createdLinked.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *createdLinked.PermissionField)
	})

	t.Run("create linked user field via API honors an explicit PermissionValues", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:             model.NewId(),
			Type:             model.PropertyFieldTypeText,
			TargetType:       "system",
			LinkedFieldID:    &sourceID,
			PermissionValues: &sysadminLevel,
		}

		createdLinked, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "user", linkedField)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdLinked.PermissionValues)
		require.Equal(t, model.PermissionLevelSysadmin, *createdLinked.PermissionValues)
	})

	t.Run("create linked channel field via API also defaults PermissionValues to member, not the sysadmin template", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:          model.NewId(),
			Type:          model.PropertyFieldTypeText,
			TargetType:    "system",
			LinkedFieldID: &sourceID,
		}

		createdLinked, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, "channel", linkedField)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, createdLinked.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *createdLinked.PermissionValues)
	})

	t.Run("patch linked user field PermissionValues both directions (lock and unlock)", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:             model.NewId(),
			Type:             model.PropertyFieldTypeText,
			GroupID:          group.ID,
			ObjectType:       "user",
			TargetType:       "system",
			LinkedFieldID:    &sourceID,
			PermissionField:  &sysadminLevel,
			PermissionValues: &sysadminLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		// Lock -> unlock: previously a one-way ratchet (see enforceGroupPermissions),
		// this must now actually take effect in both directions.
		unlockPatch := &model.PropertyFieldPatch{PermissionValues: &memberLevel}
		updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, unlockPatch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updated.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *updated.PermissionValues)

		lockPatch := &model.PropertyFieldPatch{PermissionValues: &sysadminLevel}
		updated, resp, err = th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, lockPatch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updated.PermissionValues)
		require.Equal(t, model.PermissionLevelSysadmin, *updated.PermissionValues)
	})

	t.Run("patch PermissionValues on a standalone (non-linked) user field is rejected", func(t *testing.T) {
		standaloneField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		created, appErr := th.App.CreatePropertyField(th.Context, standaloneField, false, "")
		require.Nil(t, appErr)

		patch := &model.PropertyFieldPatch{PermissionValues: &sysadminLevel}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", created.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch PermissionValues on a field with an empty (non-nil) LinkedFieldID is rejected", func(t *testing.T) {
		// LinkedFieldID = &"" is a degenerate, non-linked state: IsValid only
		// validates the ID format when non-empty, so this passes validation and
		// can be persisted (e.g. a caller explicitly sending linked_field_id:
		// "" on create). The guard must treat this the same as nil -- not doing
		// so would let a caller bypass the "must be linked" restriction on any
		// standalone field that happens to carry an empty LinkedFieldID.
		standaloneField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "user",
			TargetType:        "system",
			LinkedFieldID:     model.NewPointer(""),
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
		}
		created, appErr := th.App.CreatePropertyField(th.Context, standaloneField, false, "")
		require.Nil(t, appErr)

		patch := &model.PropertyFieldPatch{PermissionValues: &sysadminLevel}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", created.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("patch PermissionValues on a linked post field also takes effect", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        "post",
			TargetType:        "system",
			LinkedFieldID:     &sourceID,
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		patch := &model.PropertyFieldPatch{PermissionValues: &memberLevel}
		updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", createdLinked.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotNil(t, updated.PermissionValues)
		require.Equal(t, model.PermissionLevelMember, *updated.PermissionValues)
	})

	t.Run("patch PermissionValues with a value other than member/sysadmin is rejected", func(t *testing.T) {
		sourceField := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        "system",
			PermissionField:   &sysadminLevel,
			PermissionValues:  &sysadminLevel,
			PermissionOptions: &sysadminLevel,
		}
		createdSource, appErr := th.App.CreatePropertyField(th.Context, sourceField, false, "")
		require.Nil(t, appErr)

		sourceID := createdSource.ID
		linkedField := &model.PropertyField{
			Name:             model.NewId(),
			Type:             model.PropertyFieldTypeText,
			GroupID:          group.ID,
			ObjectType:       "user",
			TargetType:       "system",
			LinkedFieldID:    &sourceID,
			PermissionField:  &sysadminLevel,
			PermissionValues: &sysadminLevel,
		}
		createdLinked, appErr := th.App.CreatePropertyField(th.Context, linkedField, false, "")
		require.Nil(t, appErr)

		invalidLevel := model.PermissionLevelAdmin
		patch := &model.PropertyFieldPatch{PermissionValues: &invalidLevel}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "user", createdLinked.ID, patch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestSystemObjectType(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, err := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_system_object_type", Version: model.PropertyGroupVersionV2})
	require.Nil(t, err)

	t.Run("non-admin cannot create a system field", func(t *testing.T) {
		field := &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeText,
			TargetType: string(model.PropertyFieldTargetLevelSystem),
		}
		_, resp, createErr := th.Client.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, field)
		require.Error(t, createErr)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("sysadmin creates a system field with canonical target and sysadmin-default permissions", func(t *testing.T) {
		field := &model.PropertyField{
			Name: model.NewId(),
			Type: model.PropertyFieldTypeText,
			// Intentionally submit mismatched target fields to confirm the server canonicalizes them.
			TargetType: string(model.PropertyFieldTargetLevelTeam),
			TargetID:   model.NewId(),
		}
		created, resp, createErr := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, field)
		require.NoError(t, createErr)
		CheckCreatedStatus(t, resp)

		require.Equal(t, model.PropertyFieldObjectTypeSystem, created.ObjectType)
		require.Equal(t, string(model.PropertyFieldTargetLevelSystem), created.TargetType)
		require.Empty(t, created.TargetID)
		require.NotNil(t, created.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)
		require.NotNil(t, created.PermissionValues)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionValues)
	})

	t.Run("any authenticated user can list system fields", func(t *testing.T) {
		fields, resp, getErr := th.Client.GetPropertyFields(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelSystem),
		})
		require.NoError(t, getErr)
		CheckOKStatus(t, resp)
		require.NotEmpty(t, fields)
	})

	t.Run("legacy values GET route rejects system object type", func(t *testing.T) {
		// Force a URL that matches {object_type}/values/{target_id}. "system" is a valid
		// character set for {target_id}, so the route is reachable even though the handler
		// must reject it.
		_, resp, getErr := th.Client.GetPropertyValues(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, model.PropertyValueSystemTargetID, model.PropertyValueSearch{})
		require.Error(t, getErr)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("legacy values PATCH route rejects system object type", func(t *testing.T) {
		// Mirrors the GET rejection: the legacy URL pattern matches system/system, so the
		// route is reachable and the handler must reject it pointing callers to the
		// dedicated system route.
		items := []model.PropertyValuePatchItem{
			{FieldID: model.NewId(), Value: json.RawMessage(`"ignored"`)},
		}
		_, resp, patchErr := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, model.PropertyValueSystemTargetID, items)
		require.Error(t, patchErr)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("system PATCH rejects template field IDs", func(t *testing.T) {
		// Create the template field directly via the app layer so the test stays
		// focused on the value-patch handler. Template fields are definition-only
		// and must never accept values, regardless of entry point.
		templateField := &model.PropertyField{
			GroupID:           group.ID,
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			ObjectType:        model.PropertyFieldObjectTypeTemplate,
			TargetType:        string(model.PropertyFieldTargetLevelSystem),
			PermissionField:   model.NewPointer(model.PermissionLevelSysadmin),
			PermissionValues:  model.NewPointer(model.PermissionLevelSysadmin),
			PermissionOptions: model.NewPointer(model.PermissionLevelSysadmin),
		}
		created, appErr := th.App.CreatePropertyField(th.Context, templateField, false, "")
		require.Nil(t, appErr)

		items := []model.PropertyValuePatchItem{
			{FieldID: created.ID, Value: json.RawMessage(`"ignored"`)},
		}
		_, resp, patchErr := th.SystemAdminClient.PatchSystemPropertyValues(context.Background(), group.Name, items)
		require.Error(t, patchErr)
		// Mismatch (template field ObjectType != system route's objectType)
		// collapses to 404 to match the executePatchPropertyField shape.
		CheckNotFoundStatus(t, resp)
	})

	t.Run("system field round-trips a value via the dedicated route", func(t *testing.T) {
		field := &model.PropertyField{
			Name: model.NewId(),
			Type: model.PropertyFieldTypeText,
		}
		created, resp, createErr := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, field)
		require.NoError(t, createErr)
		CheckCreatedStatus(t, resp)

		items := []model.PropertyValuePatchItem{
			{FieldID: created.ID, Value: json.RawMessage(`"hello-system"`)},
		}

		// Non-admin cannot write a system value.
		_, resp, patchErr := th.Client.PatchSystemPropertyValues(context.Background(), group.Name, items)
		require.Error(t, patchErr)
		CheckForbiddenStatus(t, resp)

		// Sysadmin writes succeed and store the sentinel TargetID.
		upserted, resp, patchErr := th.SystemAdminClient.PatchSystemPropertyValues(context.Background(), group.Name, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)
		require.Len(t, upserted, 1)
		require.Equal(t, model.PropertyValueSystemTargetID, upserted[0].TargetID)
		require.Equal(t, model.PropertyValueTargetTypeSystem, upserted[0].TargetType)

		// Any authed user can read.
		values, resp, getErr := th.Client.GetSystemPropertyValues(context.Background(), group.Name, model.PropertyValueSearch{})
		require.NoError(t, getErr)
		CheckOKStatus(t, resp)
		found := false
		for _, v := range values {
			if v.FieldID == created.ID {
				require.Equal(t, model.PropertyValueSystemTargetID, v.TargetID)
				require.Equal(t, json.RawMessage(`"hello-system"`), v.Value)
				found = true
			}
		}
		require.True(t, found, "expected to find the system value we just wrote")
	})

	t.Run("system value patch broadcasts system-wide", func(t *testing.T) {
		field := &model.PropertyField{
			Name: model.NewId(),
			Type: model.PropertyFieldTypeText,
		}
		created, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			GroupID:           group.ID,
			Name:              field.Name,
			Type:              field.Type,
			ObjectType:        model.PropertyFieldObjectTypeSystem,
			TargetType:        string(model.PropertyFieldTargetLevelSystem),
			PermissionField:   model.NewPointer(model.PermissionLevelSysadmin),
			PermissionValues:  model.NewPointer(model.PermissionLevelSysadmin),
			PermissionOptions: model.NewPointer(model.PermissionLevelSysadmin),
		}, false, "")
		require.Nil(t, appErr)

		th.LoginBasic(t)
		webSocketClient := th.CreateConnectedWebSocketClient(t)

		items := []model.PropertyValuePatchItem{
			{FieldID: created.ID, Value: json.RawMessage(`"broadcast-me"`)},
		}
		_, resp, patchErr := th.SystemAdminClient.PatchSystemPropertyValues(context.Background(), group.Name, items)
		require.NoError(t, patchErr)
		CheckOKStatus(t, resp)

		require.Eventually(t, func() bool {
			select {
			case event := <-webSocketClient.EventChannel:
				if event.EventType() == model.WebsocketEventPropertyValuesUpdated &&
					event.GetData()["object_type"] == model.PropertyFieldObjectTypeSystem {
					require.Equal(t, model.PropertyValueSystemTargetID, event.GetData()["target_id"])
					// System target: broadcast should be system-wide (empty team/channel)
					require.Empty(t, event.GetBroadcast().TeamId)
					require.Empty(t, event.GetBroadcast().ChannelId)
					return true
				}
			default:
				return false
			}
			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("system field permissions are canonicalized to sysadmin even when admin requests member", func(t *testing.T) {
		// Without canonicalization, an admin could POST member-level
		// permissions on a system field; the per-field permission check
		// would then resolve "member" against TargetType=system, which
		// hasPropertyFieldScopeAccess treats as "any authenticated user",
		// effectively making the field publicly mutable.
		field := &model.PropertyField{
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			PermissionField:   model.NewPointer(model.PermissionLevelMember),
			PermissionValues:  model.NewPointer(model.PermissionLevelMember),
			PermissionOptions: model.NewPointer(model.PermissionLevelMember),
		}
		created, resp, createErr := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, field)
		require.NoError(t, createErr)
		CheckCreatedStatus(t, resp)

		require.NotNil(t, created.PermissionField)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionField)
		require.NotNil(t, created.PermissionValues)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionValues)
		require.NotNil(t, created.PermissionOptions)
		require.Equal(t, model.PermissionLevelSysadmin, *created.PermissionOptions)

		// With permissions canonicalized to sysadmin, the existing
		// per-field permission check rejects non-admin patch/delete
		// without any additional handler-level floor.
		newName := model.NewId()
		patch := &model.PropertyFieldPatch{Name: &newName}
		_, resp, patchErr := th.Client.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, created.ID, patch)
		require.Error(t, patchErr)
		CheckForbiddenStatus(t, resp)

		resp, deleteErr := th.Client.DeletePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeSystem, created.ID)
		require.Error(t, deleteErr)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("legacy values PATCH route rejects body referencing a system field ID", func(t *testing.T) {
		// Cross-route reference: a non-system route receives a body whose
		// field IDs belong to a system-typed field. Without the ObjectType
		// match check this would either bypass the system-write sysadmin
		// gate or persist a value whose TargetType disagrees with
		// field.ObjectType.
		systemField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			GroupID:           group.ID,
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			ObjectType:        model.PropertyFieldObjectTypeSystem,
			TargetType:        string(model.PropertyFieldTargetLevelSystem),
			PermissionField:   model.NewPointer(model.PermissionLevelSysadmin),
			PermissionValues:  model.NewPointer(model.PermissionLevelMember),
			PermissionOptions: model.NewPointer(model.PermissionLevelSysadmin),
		}, false, "")
		require.Nil(t, appErr)

		items := []model.PropertyValuePatchItem{
			{FieldID: systemField.ID, Value: json.RawMessage(`"smuggled"`)},
		}
		// Even sysadmin should be rejected — this is a structural check on
		// the route, not a permission check. Mismatch collapses to 404 to
		// match the executePatchPropertyField/executeDeletePropertyField shape.
		_, resp, patchErr := th.SystemAdminClient.PatchPropertyValues(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, th.SystemAdminUser.Id, items)
		require.Error(t, patchErr)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("system values PATCH route rejects body referencing a non-system field ID", func(t *testing.T) {
		// Inverse of the previous case: the dedicated system route is
		// reachable only by sysadmin, but it must still reject body field
		// IDs whose ObjectType isn't system, otherwise rows get persisted
		// with TargetType=system pointing at fields that attach to other
		// entity types.
		postField, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			GroupID:           group.ID,
			Name:              model.NewId(),
			Type:              model.PropertyFieldTypeText,
			ObjectType:        model.PropertyFieldObjectTypePost,
			TargetType:        string(model.PropertyFieldTargetLevelSystem),
			PermissionField:   model.NewPointer(model.PermissionLevelSysadmin),
			PermissionValues:  model.NewPointer(model.PermissionLevelSysadmin),
			PermissionOptions: model.NewPointer(model.PermissionLevelSysadmin),
		}, false, "")
		require.Nil(t, appErr)

		items := []model.PropertyValuePatchItem{
			{FieldID: postField.ID, Value: json.RawMessage(`"misrouted"`)},
		}
		_, resp, patchErr := th.SystemAdminClient.PatchSystemPropertyValues(context.Background(), group.Name, items)
		require.Error(t, patchErr)
		CheckNotFoundStatus(t, resp)
	})
}

func TestPatchPropertyValuesChangePolicy(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)

	// The change policy is enforced by the attribute validation hook, which is
	// registered against the access_control group only, and channel attributes
	// need Enterprise Advanced.
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	memberLevel := model.PermissionLevelMember

	createField := func(t *testing.T, fieldType model.PropertyFieldType, attrs model.StringInterface) *model.PropertyField {
		t.Helper()
		field, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			// Prefixed: a raw NewId may start with a digit, which the CEL
			// identifier rule on channel attribute names rejects.
			Name:              "attr_" + model.NewId(),
			Type:              fieldType,
			GroupID:           group.ID,
			ObjectType:        "channel",
			TargetType:        "system",
			PermissionField:   &memberLevel,
			PermissionValues:  &memberLevel,
			PermissionOptions: &memberLevel,
			Attrs:             attrs,
		}, false, "")
		require.Nil(t, fieldErr)
		return field
	}

	patch := func(t *testing.T, field *model.PropertyField, raw string) (*model.Response, error) {
		t.Helper()
		items := []model.PropertyValuePatchItem{{FieldID: field.ID, Value: json.RawMessage(raw)}}
		_, resp, err := th.Client.PatchPropertyValues(context.Background(), group.Name, "channel", th.BasicChannel.Id, items)
		return resp, err
	}

	th.LoginBasic(t)

	t.Run("never: a second write is forbidden", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeText, model.StringInterface{
			model.PropertyFieldAttrChangePolicy: model.PropertyFieldChangePolicyNever,
		})

		resp, err := patch(t, field, `"SECRET"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		resp, err = patch(t, field, `"OTHER"`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "app.property_value.change_policy.never.app_error")
	})

	t.Run("raise_only: down is forbidden and up is allowed", func(t *testing.T) {
		field := createField(t, model.PropertyFieldTypeRank, model.StringInterface{
			model.PropertyFieldAttributeOptions: []any{
				map[string]any{"name": "LOW", "rank": 1},
				map[string]any{"name": "MID", "rank": 2},
				map[string]any{"name": "HIGH", "rank": 3},
			},
			model.PropertyFieldAttrChangePolicy: model.PropertyFieldChangePolicyRaiseOnly,
		})

		options, ok := field.Attrs[model.PropertyFieldAttributeOptions].([]any)
		require.True(t, ok)
		optionID := func(idx int) string {
			m, isMap := options[idx].(map[string]any)
			require.True(t, isMap)
			id, isString := m["id"].(string)
			require.True(t, isString)
			return id
		}

		resp, err := patch(t, field, fmt.Sprintf("%q", optionID(1)))
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		resp, err = patch(t, field, fmt.Sprintf("%q", optionID(0)))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "app.property_value.change_policy.raise_only.app_error")

		resp, err = patch(t, field, fmt.Sprintf("%q", optionID(2)))
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}

// End-to-end proof of the DM/GM rule against the REAL access_control group,
// which is the only group the attribute validation hook is registered for.
// TestPatchPropertyValuesChannelTargetAccess is the sibling suite on a
// throwaway group; it measures the generic default, not this restriction.
func TestPatchPropertyValuesAccessControlDirectChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
		cfg.FeatureFlags.PostAttributes = true
	}).InitBasic(t)

	// The rule lives in the attribute validation hook, which sits behind the
	// licence check hook; channel attributes need Enterprise Advanced.
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	createField := func(t *testing.T, objectType string, level model.PermissionLevel) *model.PropertyField {
		t.Helper()
		field, fieldErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			// Prefixed: a raw NewId may start with a digit, which the CEL
			// identifier rule on channel attribute names rejects.
			Name:              "attr_" + model.NewId(),
			Type:              model.PropertyFieldTypeText,
			GroupID:           group.ID,
			ObjectType:        objectType,
			TargetType:        "system",
			PermissionField:   &level,
			PermissionValues:  &level,
			PermissionOptions: &level,
		}, false, "")
		require.Nil(t, fieldErr)
		return field
	}

	patchAs := func(t *testing.T, client *model.Client4, field *model.PropertyField, objectType, targetID, raw string) (*model.Response, error) {
		t.Helper()
		items := []model.PropertyValuePatchItem{{FieldID: field.ID, Value: json.RawMessage(raw)}}
		_, resp, err := client.PatchPropertyValues(context.Background(), group.Name, objectType, targetID, items)
		return resp, err
	}

	const directErrorID = "app.property_value.direct_channel.app_error"

	dmChannel := th.CreateDmChannel(t, th.BasicUser2)
	gmChannel, appErr := th.App.CreateGroupChannel(th.Context, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id}, th.BasicUser.Id)
	require.Nil(t, appErr)
	th.LoginBasic(t)

	t.Run("participant cannot write a member-tier value on their own DM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelMember)

		resp, err := patchAs(t, th.Client, field, "channel", dmChannel.Id, `"dm-val"`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, directErrorID)
	})

	t.Run("participant cannot write an admin-tier value on their own DM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelAdmin)

		resp, err := patchAs(t, th.Client, field, "channel", dmChannel.Id, `"dm-val"`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, directErrorID)
	})

	t.Run("participant cannot write a member-tier value on their own GM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelMember)

		resp, err := patchAs(t, th.Client, field, "channel", gmChannel.Id, `"gm-val"`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, directErrorID)
	})

	t.Run("participant cannot write an admin-tier value on their own GM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelAdmin)

		resp, err := patchAs(t, th.Client, field, "channel", gmChannel.Id, `"gm-val"`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, directErrorID)
	})

	t.Run("a system admin can write on the same DM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelMember)

		resp, err := patchAs(t, th.SystemAdminClient, field, "channel", dmChannel.Id, `"admin-val"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	// Regression guard: the rule must not leak to normal channels.
	t.Run("a channel member can still write on a public channel", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelMember)

		resp, err := patchAs(t, th.Client, field, "channel", th.BasicChannel.Id, `"pub-val"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	// Clearing is an upsert of an empty value, not a DELETE -- there is no
	// delete route for property values. A participant must not be able to
	// launder a marking off a DM this way.
	t.Run("participant cannot clear a value on their own DM", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypeChannel, model.PermissionLevelMember)

		resp, err := patchAs(t, th.SystemAdminClient, field, "channel", dmChannel.Id, `"seeded"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		resp, err = patchAs(t, th.Client, field, "channel", dmChannel.Id, `""`)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, directErrorID)
	})

	// The recorded post-object decision, end to end: post attributes are the
	// feature this branch serves, and a marking on a message in a DM stays
	// writable by a participant.
	t.Run("participant can write a post-object value on a DM post", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypePost, model.PermissionLevelMember)
		post := th.CreatePostWithClient(t, th.Client, dmChannel)

		resp, err := patchAs(t, th.Client, field, "post", post.Id, `"flagged"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	// A participant administers posts in their own DM, and the hook does not
	// take that back: its refusal is scoped to channel-object values.
	t.Run("participant can write an admin-tier post-object value on a DM post", func(t *testing.T) {
		field := createField(t, model.PropertyFieldObjectTypePost, model.PermissionLevelAdmin)
		post := th.CreatePostWithClient(t, th.Client, dmChannel)

		resp, err := patchAs(t, th.Client, field, "post", post.Id, `"flagged"`)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}

func TestChannelAttributesRequireEnterpriseAdvanced(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
	}).InitBasic(t)

	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

	groupName := model.AccessControlPropertyGroupName

	t.Run("listing channel fields is refused", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			PerPage:    100,
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("reading channel values is refused", func(t *testing.T) {
		_, resp, err := th.Client.GetPropertyValues(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, th.BasicChannel.Id, model.PropertyValueSearch{PerPage: 100})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("user attributes in the same group are unaffected", func(t *testing.T) {
		_, _, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, model.PropertyFieldObjectTypeUser, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			PerPage:    100,
		})
		require.NoError(t, err)
	})

	t.Run("Enterprise Advanced is admitted", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

		_, _, err := th.SystemAdminClient.GetPropertyFields(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, model.PropertyFieldSearch{
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			PerPage:    100,
		})
		require.NoError(t, err)
	})

	fieldID := model.NewId()

	t.Run("listing channel field options is refused", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, fieldID, 0, "", model.PropertyFieldOptionsMaxPerRequest)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("creating channel field options is refused", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.CreatePropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, fieldID, []*model.PropertyFieldOption{
			{Name: "x"},
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("patching channel field options is refused", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.PatchPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, fieldID, []*model.PropertyFieldOption{
			{Name: "x"},
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("deleting channel field options is refused", func(t *testing.T) {
		resp, err := th.SystemAdminClient.DeletePropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeChannel, fieldID, []string{model.NewId()})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("listing user field options in the same group is not a license refusal", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeUser, fieldID, 0, "", model.PropertyFieldOptionsMaxPerRequest)
		require.Error(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	group, appErr := th.App.GetPropertyGroup(th.Context, groupName)
	require.Nil(t, appErr)

	createLinkedSelect := func(t *testing.T, objectType string) *model.PropertyField {
		t.Helper()
		template, createErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       celSafeName(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: model.PropertyFieldObjectTypeTemplate,
			TargetType: "system",
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []any{
					map[string]any{"name": "alpha"},
				},
			},
		}, false, "")
		require.Nil(t, createErr)

		linked := &model.PropertyField{
			Name:          celSafeName(),
			Type:          model.PropertyFieldTypeSelect,
			GroupID:       group.ID,
			ObjectType:    objectType,
			TargetType:    "system",
			LinkedFieldID: &template.ID,
		}
		_, createErr = th.App.CreatePropertyField(th.Context, linked, false, "")
		require.Nil(t, createErr)
		return template
	}

	t.Run("options on a template serving a channel field are refused", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeChannel)

		_, resp, err := th.SystemAdminClient.GetPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, 0, "", model.PropertyFieldOptionsMaxPerRequest)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)

		_, resp, err = th.SystemAdminClient.CreatePropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, []*model.PropertyFieldOption{
			{Name: "beta"},
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("options on a template serving only a user field are admitted", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeUser)

		listed, _, err := th.SystemAdminClient.GetPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, 0, "", model.PropertyFieldOptionsMaxPerRequest)
		require.NoError(t, err)
		require.Equal(t, []string{"alpha"}, optionNames(listed.Options))
	})

	t.Run("Enterprise Advanced admits options on a template serving a channel field", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeChannel)

		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

		listed, _, err := th.SystemAdminClient.GetPropertyFieldOptions(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, 0, "", model.PropertyFieldOptionsMaxPerRequest)
		require.NoError(t, err)
		require.Equal(t, []string{"alpha"}, optionNames(listed.Options))
	})

	optionsOnlyPatch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{
		model.PropertyFieldAttributeOptions: []any{map[string]any{"name": "beta"}},
	}}

	t.Run("patching a template serving a channel field is refused", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeChannel)

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, optionsOnlyPatch)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("deleting a template serving a channel field is refused", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeChannel)

		resp, err := th.SystemAdminClient.DeletePropertyField(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property.channel_attributes.license.app_error")
		require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	})

	t.Run("patching a template serving only a user field is not a license refusal", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeUser)

		_, _, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, optionsOnlyPatch)
		if err != nil {
			var appErr *model.AppError
			require.ErrorAs(t, err, &appErr)
			require.NotEqual(t, "api.property.channel_attributes.license.app_error", appErr.Id)
		}
	})

	t.Run("Enterprise Advanced admits patching a template serving a channel field", func(t *testing.T) {
		template := createLinkedSelect(t, model.PropertyFieldObjectTypeChannel)

		th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))
		defer th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

		_, _, err := th.SystemAdminClient.PatchPropertyField(context.Background(), groupName, model.PropertyFieldObjectTypeTemplate, template.ID, optionsOnlyPatch)
		require.NoError(t, err)
	})
}

// isOptionsOnlyPatch decides which permission a patch is gated on at this
// layer; model.PropertyFieldChangeIsOptionsOnly decides the same thing inside
// the enforcement hook, which never sees the patch and has to compare the
// merged field against the stored one. If the two ever disagree, one layer
// allows what the other refuses. This asserts they agree on every shape of
// patch that reaches the handler.
func TestOptionsOnlyPatchAgreesWithFieldComparison(t *testing.T) {
	// One fixed option ID: stored() is called twice, so a fresh model.NewId()
	// per call would make the two fields differ before any patch was applied.
	storedOptionID := model.NewId()

	stored := func() *model.PropertyField {
		return &model.PropertyField{
			ID:         "fieldid",
			GroupID:    "groupid",
			Name:       "Programs",
			Type:       model.PropertyFieldTypeSelect,
			ObjectType: "user",
			TargetType: "system",
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []any{
					map[string]any{"id": storedOptionID, "name": "Air"},
				},
				"visibility": "always",
			},
		}
	}

	newOptions := []any{map[string]any{"id": model.NewId(), "name": "Sea"}}
	storedPermissions := &model.Permissions{Restrictions: &model.Restrictions{
		Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
	}}

	// A field linked to a template, stored as one is when created without
	// grants: null grants, no masking of its own. It is served grants: [] and,
	// when its template is masked, masking: {}.
	linkedPermissions := &model.Permissions{Restrictions: &model.Restrictions{
		Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
	}}
	servedLinkedEcho := &model.PermissionsPatch{
		Restrictions: json.RawMessage(`{"field":{"write":"sysadmin"}}`),
		Grants:       json.RawMessage(`[]`),
		Masking:      json.RawMessage(`{}`),
	}

	testCases := []struct {
		name string
		// permissions the field already carries, if any
		permissions *model.Permissions
		// whether the field is linked to a template
		linked bool
		patch  *model.PropertyFieldPatch
		// what both layers are expected to answer
		want bool
	}{
		{"options alone", nil, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
		}, true},
		{"options plus a sibling attr", nil, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions, "visibility": "hidden"},
		}, false},
		{"options plus a name", nil, false, &model.PropertyFieldPatch{
			Name:  model.NewPointer("Renamed"),
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
		}, false},
		{"options plus permissions that change something", storedPermissions, false, &model.PropertyFieldPatch{
			Attrs:       &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: &model.PermissionsPatch{Restrictions: json.RawMessage(`{"field":{"write":"member"}}`)},
		}, false},
		{"options plus a permissions echo", storedPermissions, false, &model.PropertyFieldPatch{
			Attrs:       &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: &model.PermissionsPatch{Restrictions: json.RawMessage(`{"field":{"write":"sysadmin"}}`)},
		}, true},
		{"options plus an echo of null grants as served, []", storedPermissions, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: &model.PermissionsPatch{
				Restrictions: json.RawMessage(`{"field":{"write":"sysadmin"}}`),
				Grants:       json.RawMessage(`[]`),
			},
		}, true},
		{"options plus a grant added to null grants", storedPermissions, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: &model.PermissionsPatch{
				Grants: json.RawMessage(`[{"type":"user","id":"` + model.NewId() + `","allow":["field.write"]}]`),
			},
		}, false},
		{"options plus a linked field's served permissions echo", linkedPermissions, true, &model.PropertyFieldPatch{
			Attrs:       &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: servedLinkedEcho,
		}, true},
		{"options plus masking {} on an unlinked field", storedPermissions, false, &model.PropertyFieldPatch{
			Attrs:       &model.StringInterface{model.PropertyFieldAttributeOptions: newOptions},
			Permissions: &model.PermissionsPatch{Masking: json.RawMessage(`{}`)},
		}, false},
		{"a sibling attr alone", nil, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{"visibility": "hidden"},
		}, false},
		{"a name alone", nil, false, &model.PropertyFieldPatch{
			Name: model.NewPointer("Renamed"),
		}, false},
		{"no attrs at all", nil, false, &model.PropertyFieldPatch{}, false},
		{"an unchanged options echo", nil, false, &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{model.PropertyFieldAttributeOptions: []any{
				map[string]any{"id": storedOptionID, "name": "Air"},
			}},
		}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			existing := stored()
			existing.Permissions = tc.permissions
			patched := stored()
			patched.Permissions = tc.permissions
			if tc.linked {
				existing.LinkedFieldID = model.NewPointer("templateid")
				patched.LinkedFieldID = model.NewPointer("templateid")
			}

			// Mirror patchPropertyField: resolve the permissions patch first,
			// decide whether it changed anything, then apply the rest.
			permissionsChanged := false
			if tc.patch.Permissions != nil {
				applied, err := applyPermissionsPatch(patched, tc.patch)
				require.NoError(t, err)
				permissionsChanged = !model.PermissionsEqual(patched.Permissions, applied)
				patched.Permissions = applied
			}

			optionsChanged := false
			if tc.patch.Attrs != nil {
				if newOptions, ok := (*tc.patch.Attrs)[model.PropertyFieldAttributeOptions]; ok {
					optionsChanged = !reflect.DeepEqual(existing.Attrs[model.PropertyFieldAttributeOptions], newOptions)
				}
			}

			patched.Patch(tc.patch, true)

			handlerSays := isOptionsOnlyPatch(tc.patch, permissionsChanged, optionsChanged)
			require.Equal(t,
				handlerSays,
				model.PropertyFieldChangeIsOptionsOnly(existing, patched),
				"the api4 patch check and the hook's field comparison must agree")
			require.Equal(t, tc.want, handlerSays)
		})
	}
}

func TestPatchPropertyFieldLegacyAttrs(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_properties_legacy_attrs", Version: model.PropertyGroupVersionV2})
	require.Nil(t, appErr)

	restrictions := &model.Restrictions{
		Field:  model.WriteOnly{Write: model.PermissionLevelSysadmin},
		Value:  model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelMember},
		Option: model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelSysadmin},
	}
	createField := func(t *testing.T, grants []model.Grant) *model.PropertyField {
		field, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:     group.ID,
			Name:        model.NewId(),
			Type:        model.PropertyFieldTypeText,
			ObjectType:  model.PropertyFieldObjectTypePost,
			TargetType:  string(model.PropertyFieldTargetLevelSystem),
			Permissions: &model.Permissions{Restrictions: restrictions, Grants: grants},
		})
		require.NoError(t, err)
		return field
	}
	ownersPatch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{
		model.PropertyAttrsOwners: []model.PropertyOwner{{Type: model.PropertyOwnerTypeUser, ID: model.NewId()}},
	}}

	t.Run("an owners change on permissions the legacy shape can express succeeds", func(t *testing.T) {
		// Outside access_control the legacy shape converts to the wildcard
		// plugin grant alone, so that is the grant set it can express.
		field := createField(t, []model.Grant{{
			Identity: model.Identity{Type: model.PropertyOwnerTypePlugin, ID: "*"},
			Allow:    []string{model.PropertyActionFieldWrite, model.PropertyActionOptionRead, model.PropertyActionOptionWrite, model.PropertyActionValueRead, model.PropertyActionValueWrite},
		}})

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, ownersPatch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("an owners change on permissions the legacy shape cannot express is a 400", func(t *testing.T) {
		grant := model.Grant{
			Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: model.NewId()},
			Allow:    []string{model.PropertyActionValueWrite},
		}
		field := createField(t, []model.Grant{grant})

		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, "post", field.ID, ownersPatch)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "app.property_field.update.legacy_attrs_lossy.app_error")

		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, field.ID)
		require.Nil(t, appErr)
		require.Equal(t, []model.Grant{grant}, stored.Permissions.Grants)
	})
}

func TestDeleteOrphanedProtectedField(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	// Written straight to the store with field.write none, the shape a
	// protected field converts to; the Protected column stays false so the
	// store's IsValid accepts PermissionField sysadmin. No test plugin is
	// installed, so any source_plugin_id names an uninstalled plugin.
	createProtected := func(t *testing.T, sourcePluginID string) *model.PropertyField {
		t.Helper()
		sysadmin := model.PermissionLevelSysadmin
		attrs := model.StringInterface{model.PropertyAttrsProtected: true}
		if sourcePluginID != "" {
			attrs[model.PropertyAttrsSourcePluginID] = sourcePluginID
		}
		field, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:         group.ID,
			Name:            celSafeName(),
			Type:            model.PropertyFieldTypeText,
			Attrs:           attrs,
			ObjectType:      model.PropertyFieldObjectTypeUser,
			TargetType:      string(model.PropertyFieldTargetLevelSystem),
			PermissionField: &sysadmin,
			Permissions: &model.Permissions{Restrictions: &model.Restrictions{
				Field:  model.WriteOnly{Write: model.PermissionLevelNone},
				Value:  model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelNone},
				Option: model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelNone},
			}},
		})
		require.NoError(t, err)
		return field
	}

	endpoints := map[string]func(fieldID string) (*model.Response, error){
		"deletePropertyField": func(fieldID string) (*model.Response, error) {
			return th.SystemAdminClient.DeletePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, fieldID)
		},
		"deleteCPAField": func(fieldID string) (*model.Response, error) {
			return th.SystemAdminClient.DeleteCPAField(context.Background(), fieldID)
		},
	}
	for name, deleteField := range endpoints {
		t.Run(name, func(t *testing.T) {
			t.Run("a sysadmin can delete a field whose source plugin is gone", func(t *testing.T) {
				field := createProtected(t, "removed-plugin")

				resp, err := deleteField(field.ID)
				require.NoError(t, err)
				CheckOKStatus(t, resp)
			})

			t.Run("a sysadmin cannot delete a protected field with no source plugin", func(t *testing.T) {
				field := createProtected(t, "")

				resp, err := deleteField(field.ID)
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
			})
		})
	}
}

func TestCreateAccessControlFieldWithPermissions(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)
	require.True(t, group.IsPSAv3())

	// field.write none with a grant naming only the creator: stored, it would
	// leave no administrator able to edit or delete the field.
	lockingField := func(creatorID string) *model.PropertyField {
		return &model.PropertyField{
			Name:       celSafeName(),
			Type:       model.PropertyFieldTypeText,
			TargetType: string(model.PropertyFieldTargetLevelChannel),
			TargetID:   th.BasicChannel.Id,
			Permissions: &model.Permissions{
				Restrictions: &model.Restrictions{
					Field: model.WriteOnly{Write: model.PermissionLevelNone},
				},
				Grants: []model.Grant{{
					Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: creatorID},
					Allow:    []string{model.PropertyActionFieldWrite},
				}},
			},
		}
	}

	requireNotStored := func(t *testing.T, name string) {
		t.Helper()
		stored, appErr := th.App.SearchPropertyFields(th.Context, group.ID, model.PropertyFieldSearchOpts{
			ObjectTypes: []string{model.PropertyFieldObjectTypeUser},
			TargetIDs:   []string{th.BasicChannel.Id},
			PerPage:     100,
		})
		require.Nil(t, appErr)
		for _, f := range stored {
			require.NotEqual(t, name, f.Name)
		}
	}

	t.Run("a plain channel member is refused and nothing is stored", func(t *testing.T) {
		th.LoginBasic(t)

		field := lockingField(th.BasicUser.Id)
		_, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		requireNotStored(t, field.Name)
	})

	t.Run("a channel admin is refused too, since the group's fields are sysadmin-edited", func(t *testing.T) {
		channelAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, channelAdmin, th.BasicTeam)
		_, appErr := th.App.AddUserToChannel(th.Context, channelAdmin, th.BasicChannel, false)
		require.Nil(t, appErr)
		_, appErr = th.App.UpdateChannelMemberRoles(th.Context, th.BasicChannel.Id, channelAdmin.Id,
			model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		require.Nil(t, appErr)

		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), channelAdmin.Email, channelAdmin.Password)
		require.NoError(t, err)

		field := lockingField(channelAdmin.Id)
		_, resp, err := client.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		requireNotStored(t, field.Name)
	})

	t.Run("a member can still create the field without one", func(t *testing.T) {
		th.LoginBasic(t)

		field := lockingField(th.BasicUser.Id)
		field.Permissions = nil
		created, resp, err := th.Client.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		// The default the hook pins: only a system administrator edits it.
		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, created.ID)
		require.Nil(t, appErr)
		require.Equal(t, model.PermissionLevelSysadmin, stored.Permissions.Restrictions.Field.Write)
	})

	t.Run("a system admin may submit one, and it is stored as sent", func(t *testing.T) {
		// Not field.write none, though: no permissions object submitted
		// through the API may leave a field that no system admin can edit.
		field := lockingField(th.SystemAdminUser.Id)
		_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		requireNotStored(t, field.Name)

		field = lockingField(th.SystemAdminUser.Id)
		field.Permissions.Restrictions.Field.Write = model.PermissionLevelSysadmin
		created, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, created.ID)
		require.Nil(t, appErr)
		require.Equal(t, model.PermissionLevelSysadmin, stored.Permissions.Restrictions.Field.Write)
		require.Equal(t, field.Permissions.Grants, stored.Permissions.Grants)
	})
}

func TestPatchAccessControlFieldPermissionsEcho(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterprise))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	// A field kept at field.write none, as one only a plugin may redefine
	// is, but whose options a system admin may still change. Written
	// straight to the store, since no API caller may set field.write none.
	newField := func(t *testing.T, grants []model.Grant) *model.PropertyField {
		t.Helper()
		field, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:    group.ID,
			Name:       celSafeName(),
			Type:       model.PropertyFieldTypeSelect,
			ObjectType: model.PropertyFieldObjectTypeUser,
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Air"}},
			},
			Permissions: &model.Permissions{Restrictions: &model.Restrictions{
				Field:  model.WriteOnly{Write: model.PermissionLevelNone},
				Value:  model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelNone},
				Option: model.ReadWrite{Read: model.PermissionLevelEveryone, Write: model.PermissionLevelSysadmin},
			}, Grants: grants},
		})
		require.NoError(t, err)
		return field
	}

	optionsPatch := func(t *testing.T, restrictions *model.Restrictions) *model.PropertyFieldPatch {
		t.Helper()
		raw, err := json.Marshal(restrictions)
		require.NoError(t, err)
		return &model.PropertyFieldPatch{
			Attrs: &model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Sea"}},
			},
			Permissions: &model.PermissionsPatch{Restrictions: raw},
		}
	}

	t.Run("sending back the stored restrictions with new options is accepted", func(t *testing.T) {
		field := newField(t, nil)

		patched, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field.ID, optionsPatch(t, field.Permissions.Restrictions))
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, model.PermissionLevelNone, patched.Permissions.Restrictions.Field.Write)
	})

	t.Run("changing the restrictions while leaving field.write at none is refused", func(t *testing.T) {
		// A grant lets the caller past field.write, so it is the floor that
		// refuses them.
		field := newField(t, []model.Grant{{
			Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: th.SystemAdminUser.Id},
			Allow:    []string{model.PropertyActionFieldWrite},
		}})

		changed := *field.Permissions.Restrictions
		changed.Value.Read = model.PermissionLevelSysadmin
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field.ID, optionsPatch(t, &changed))
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.property_field.permissions.field_write_none.app_error")
	})
}

func TestPatchPropertyFieldNullGrantsEcho(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.IntegratedBoards = true
		cfg.FeatureFlags.PropertyFieldPermissionsV3 = true
	}).InitBasic(t)

	group, appErr := th.App.RegisterPropertyGroup(th.Context, &model.PropertyGroup{Name: "test_v3_null_grants_echo", Version: model.PropertyGroupVersionV3})
	require.Nil(t, appErr)

	// A field created without grants stores them as null, and is served
	// grants: []. th.BasicUser, a member of th.BasicChannel, may change its
	// options but not the rest of its definition.
	newField := func(t *testing.T) *model.PropertyField {
		t.Helper()
		created, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       model.NewId(),
			Type:       model.PropertyFieldTypeSelect,
			GroupID:    group.ID,
			ObjectType: "post",
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			Attrs: model.StringInterface{
				model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Air"}},
			},
			Permissions: &model.Permissions{Restrictions: &model.Restrictions{
				Value:  model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
				Option: model.ReadWrite{Read: model.PermissionLevelMember, Write: model.PermissionLevelMember},
				Field:  model.WriteOnly{Write: model.PermissionLevelSysadmin},
			}},
		}, false, "")
		require.Nil(t, appErr)

		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, created.ID)
		require.Nil(t, appErr)
		require.Nil(t, stored.Permissions.Grants, "the fixture must store null grants")
		return stored
	}

	// served reads the field through the API as th.Client's user sees it.
	served := func(t *testing.T, fieldID string) *model.PropertyField {
		t.Helper()
		fields, _, err := th.Client.GetPropertyFields(context.Background(), group.Name, "post", model.PropertyFieldSearch{
			TargetType: "channel",
			TargetID:   th.BasicChannel.Id,
			PerPage:    200,
		})
		require.NoError(t, err)
		for _, f := range fields {
			if f.ID == fieldID {
				return f
			}
		}
		require.FailNow(t, "field not served", fieldID)
		return nil
	}

	// asPatch sends permissions back the way a client would: the JSON it was
	// served, decoded into a patch.
	asPatch := func(t *testing.T, permissions *model.Permissions) *model.PermissionsPatch {
		t.Helper()
		b, err := json.Marshal(permissions)
		require.NoError(t, err)
		var patch model.PermissionsPatch
		require.NoError(t, json.Unmarshal(b, &patch))
		return &patch
	}

	newOptions := func() *model.StringInterface {
		return &model.StringInterface{
			model.PropertyFieldAttributeOptions: []map[string]any{{"id": model.NewId(), "name": "Sea"}},
		}
	}

	th.LoginBasic(t)

	t.Run("sending back the served permissions with new options needs only option.write", func(t *testing.T) {
		field := newField(t)
		got := served(t, field.ID)
		require.NotNil(t, got.Permissions)
		require.NotNil(t, got.Permissions.Grants, "null grants are served as []")
		require.Empty(t, got.Permissions.Grants)

		patched, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{
			Attrs:       newOptions(),
			Permissions: asPatch(t, got.Permissions),
		})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, field.Permissions.Restrictions, patched.Permissions.Restrictions)
		require.Empty(t, patched.Permissions.Grants)
	})

	t.Run("a grant added alongside new options still needs field.write", func(t *testing.T) {
		field := newField(t)
		got := served(t, field.ID)
		got.Permissions.Grants = []model.Grant{{
			Identity: model.Identity{Type: model.PropertyOwnerTypeUser, ID: th.BasicUser.Id},
			Allow:    []string{model.PropertyActionFieldWrite},
		}}

		_, resp, err := th.Client.PatchPropertyField(context.Background(), group.Name, "post", field.ID, &model.PropertyFieldPatch{
			Attrs:       newOptions(),
			Permissions: asPatch(t, got.Permissions),
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		stored, appErr := th.App.GetPropertyField(th.Context, group.ID, field.ID)
		require.Nil(t, appErr)
		require.Nil(t, stored.Permissions.Grants)
		require.Equal(t, field.Attrs[model.PropertyFieldAttributeOptions], stored.Attrs[model.PropertyFieldAttributeOptions])
	})
}

// TestPatchPropertyFieldRequiredChannelsMissingValuesGate covers
// checkRequiredAttributeTransition (MM-70717): PATCHing a channel attribute's
// attrs.required from false to true must be rejected with 409 while any
// active channel lacks a value for it.
//
// Only the *blocking* direction is asserted against an exact channel
// population: any Mattermost test server already has at least one active
// channel (InitBasic's defaults, if nothing else), and the field ID used in
// each subtest is freshly generated, so that pre-existing channel is
// guaranteed to have no value for it. The reverse ("succeeds once every
// channel has a value") is not asserted here -- it would require every
// channel already in the shared test database to carry a value for this
// field, which this suite cannot control -- and is instead covered at the
// store layer (storetest.testChannelStoreExistsChannelMissingPropertyValue),
// which starts from a clean, fully-controlled Channels table.
func TestPatchPropertyFieldRequiredChannelsMissingValuesGate(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
		cfg.FeatureFlags.ChannelAttributesRequired = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	newField := func(t *testing.T, objectType string, required bool) *model.PropertyField {
		t.Helper()
		// "a" prefix: field names in this group are validated as CEL
		// identifiers (must start with a letter/underscore), which
		// model.NewId() alone does not always satisfy.
		attrs := model.StringInterface{}
		if required {
			// Create the field through the normal App path first so all defaults
			// are populated, then establish the already-required fixture directly
			// in the store. Creating it required through App is intentionally
			// blocked by the transition gate under test.
			field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
				Name:       "a" + model.NewId(),
				Type:       model.PropertyFieldTypeText,
				GroupID:    group.ID,
				ObjectType: objectType,
				TargetType: "system",
				Attrs:      attrs,
			}, false, "")
			require.Nil(t, appErr)

			field.Attrs[model.PropertyFieldAttrRequired] = true
			updated, err := th.App.Srv().Store().PropertyField().Update(group.ID, []*model.PropertyField{field}, nil)
			require.NoError(t, err)
			return updated[0]
		}

		field, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       "a" + model.NewId(),
			Type:       model.PropertyFieldTypeText,
			GroupID:    group.ID,
			ObjectType: objectType,
			TargetType: "system",
			Attrs:      attrs,
		}, false, "")
		require.Nil(t, appErr)
		return field
	}

	t.Run("blocks turning required on while an active channel lacks a value", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeChannel, false)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"required": true}}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, field.ID, patch)
		require.Error(t, err)
		CheckErrorID(t, err, "api.property_field.required.channels_missing_values.app_error")
		require.Equal(t, http.StatusConflict, resp.StatusCode)

		reloaded, appErr := th.App.GetPropertyField(th.Context, group.ID, field.ID)
		require.Nil(t, appErr)
		require.False(t, model.IsPropertyFieldRequired(reloaded), "the field must be unchanged in the database after a rejected patch")
	})

	t.Run("an already-required field accepts an unrelated attribute change", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeChannel, true)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"display_name": "Renamed"}}
		updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.True(t, model.IsPropertyFieldRequired(updated), "required must be untouched by a patch that never mentions it")
	})

	t.Run("re-sending required=true unchanged succeeds", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeChannel, true)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"required": true}}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("flipping required back to false always succeeds -- the escape hatch", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeChannel, true)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"required": false}}
		updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.False(t, model.IsPropertyFieldRequired(updated))
	})

	t.Run("the same toggle on a user-object-type field is unaffected", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeUser, false)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"required": true}}
		updated, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, field.ID, patch)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.True(t, model.IsPropertyFieldRequired(updated))
	})

	t.Run("non-HTTP callers (plugin/local) are blocked too, not just the api4 handler", func(t *testing.T) {
		field := newField(t, model.PropertyFieldObjectTypeChannel, false)
		field.Attrs["required"] = true

		_, _, appErr := th.App.UpdatePropertyField(th.Context, group.ID, field, false, "")
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusConflict, appErr.StatusCode)
	})

	t.Run("with required enforcement disabled the disabled error wins over the missing-values gate", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ChannelAttributesRequired = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.FeatureFlags.ChannelAttributesRequired = true })

		field := newField(t, model.PropertyFieldObjectTypeChannel, false)

		patch := &model.PropertyFieldPatch{Attrs: &model.StringInterface{"required": true}}
		_, resp, err := th.SystemAdminClient.PatchPropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, field.ID, patch)
		require.Error(t, err)
		CheckErrorID(t, err, "app.property_field.required_disabled.app_error")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestCreatePropertyFieldRequiredChannelsMissingValuesGate verifies callers
// cannot bypass the false-to-true update gate by creating a field already
// required. A new field has no ID and therefore no existing channel can hold
// a value for it, so required creation is blocked whenever active local
// channels exist.
func TestCreatePropertyFieldRequiredChannelsMissingValuesGate(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ChannelAttributes = true
		cfg.FeatureFlags.ChannelAttributesRequired = true
	}).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)

	t.Run("blocks creating a channel-object field already required", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, &model.PropertyField{
			Name:       "a" + model.NewId(),
			Type:       model.PropertyFieldTypeText,
			ObjectType: model.PropertyFieldObjectTypeChannel,
			TargetType: "system",
			Attrs:      model.StringInterface{"required": true},
		})
		require.Error(t, err)
		CheckErrorID(t, err, "api.property_field.required.channels_missing_values.app_error")
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("creating without required succeeds", func(t *testing.T) {
		field, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeChannel, &model.PropertyField{
			Name:       "a" + model.NewId(),
			Type:       model.PropertyFieldTypeText,
			ObjectType: model.PropertyFieldObjectTypeChannel,
			TargetType: "system",
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.False(t, model.IsPropertyFieldRequired(field))
	})

	t.Run("required=true on a user-object-type field is unaffected", func(t *testing.T) {
		field, resp, err := th.SystemAdminClient.CreatePropertyField(context.Background(), group.Name, model.PropertyFieldObjectTypeUser, &model.PropertyField{
			// User-object (CPA) field names are validated as CEL identifiers,
			// which model.NewId() alone does not always satisfy.
			Name:       "a" + model.NewId(),
			Type:       model.PropertyFieldTypeText,
			ObjectType: model.PropertyFieldObjectTypeUser,
			TargetType: "system",
			Attrs:      model.StringInterface{"required": true},
		})
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.True(t, model.IsPropertyFieldRequired(field))
	})

	t.Run("non-HTTP callers (plugin/local) are blocked too, not just the api4 handler", func(t *testing.T) {
		_, appErr := th.App.CreatePropertyField(th.Context, &model.PropertyField{
			Name:       "a" + model.NewId(),
			Type:       model.PropertyFieldTypeText,
			GroupID:    group.ID,
			ObjectType: model.PropertyFieldObjectTypeChannel,
			TargetType: "system",
			Attrs:      model.StringInterface{"required": true},
		}, false, "")
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusConflict, appErr.StatusCode)
	})
}
