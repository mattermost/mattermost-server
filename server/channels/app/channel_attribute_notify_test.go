// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
)

// The notify flow's delivery is asynchronous (fired via Server.Go), so
// assertions on the resulting DMs must poll rather than check immediately.
const (
	waitTimeout      = 5 * time.Second
	shortWaitTimeout = 200 * time.Millisecond
	waitInterval     = 20 * time.Millisecond
)

// dmPostsFromBotToUser returns the posts the system bot has sent to userID in
// their DM channel, or nil if that DM channel does not exist yet.
func dmPostsFromBotToUser(t *testing.T, th *TestHelper, botUserID, userID string) []*model.Post {
	t.Helper()

	channel, err := th.App.Srv().Store().Channel().GetByName("", model.GetDMNameFromIds(botUserID, userID), false)
	if err != nil {
		return nil
	}

	postList, err := th.App.Srv().Store().Post().GetPosts(th.Context, model.GetPostsOptions{ChannelId: channel.Id, Page: 0, PerPage: 50}, false, map[string]bool{})
	require.NoError(t, err)

	posts := make([]*model.Post, 0, len(postList.Order))
	for _, id := range postList.Order {
		posts = append(posts, postList.Posts[id])
	}
	return posts
}

// newChannelNoAdmin creates a channel via th.CreateChannel and immediately
// demotes th.BasicUser (which CreateChannel always makes a scheme admin as
// its creator) back to a regular member, so the channel starts with no
// channel admin at all -- required for these tests, which otherwise treat
// every channel as already having an admin.
func newChannelNoAdmin(t *testing.T, th *TestHelper, team *model.Team) *model.Channel {
	t.Helper()
	c := th.CreateChannel(t, team)
	_, appErr := th.App.UpdateChannelMemberSchemeRoles(th.Context, c.Id, th.BasicUser.Id, false, true, false)
	require.Nil(t, appErr)
	return c
}

func setChannelPropertyValue(t *testing.T, th *TestHelper, groupID, fieldID, channelID, value string) {
	t.Helper()
	_, err := th.App.Srv().Store().PropertyValue().Create(&model.PropertyValue{
		TargetID:   channelID,
		TargetType: model.PropertyValueTargetTypeChannel,
		GroupID:    groupID,
		FieldID:    fieldID,
		Value:      json.RawMessage(value),
	})
	require.NoError(t, err)
}

func TestNotifyChannelAdminsOfMissingAttribute(t *testing.T) {
	mainHelper.Parallel(t)

	makeField := func(displayName string) *model.PropertyField {
		return &model.PropertyField{
			ID:         model.NewId(),
			ObjectType: model.PropertyFieldObjectTypeChannel,
			Attrs:      model.StringInterface{"display_name": displayName},
		}
	}

	t.Run("one DM per unique admin, listing every one of their affected channels", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		bot, appErr := th.App.GetSystemBot(th.Context)
		require.Nil(t, appErr)

		team := th.CreateTeam(t)
		admin := th.CreateUser(t)
		groupID := model.NewId()
		field := makeField("Cost center")

		var channels []*model.Channel
		th.LinkUserToTeam(t, admin, team)
		for range 3 {
			c := newChannelNoAdmin(t, th, team)
			th.AddUserToChannel(t, admin, c)
			_, appErr = th.App.UpdateChannelMemberSchemeRoles(th.Context, c.Id, admin.Id, false, true, true)
			require.Nil(t, appErr)
			channels = append(channels, c)
		}

		result, appErr := th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.Nil(t, appErr)
		// Not asserted as exact equality: the field ID is fresh, so every
		// other active channel already in the test server (e.g. InitBasic's
		// default channels) also "lacks a value" for it and contributes to
		// these aggregates. The per-admin DM assertions below are what this
		// test is actually about.
		assert.GreaterOrEqual(t, result.QueuedAdminCount, int64(1))
		assert.GreaterOrEqual(t, result.QueuedChannelCount, int64(3))
		assert.False(t, result.Truncated)

		require.Eventually(t, func() bool {
			return len(dmPostsFromBotToUser(t, th, bot.UserId, admin.Id)) == 1
		}, waitTimeout, waitInterval, "admin should receive exactly one DM")

		posts := dmPostsFromBotToUser(t, th, bot.UserId, admin.Id)
		require.Len(t, posts, 1)
		assert.Equal(t, bot.UserId, posts[0].UserId)
		assert.Equal(t, model.PostTypeDefault, posts[0].Type)
		assert.Equal(t, field.ID, posts[0].GetProp("property_field_id"))
		for _, c := range channels {
			assert.Contains(t, posts[0].Message, c.DisplayName)
		}
	})

	t.Run("channels with no channel admin are counted but nobody is notified", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		team := th.CreateTeam(t)
		groupID := model.NewId()
		field := makeField("Cost center")

		// A channel with members but no scheme admin.
		member := th.CreateUser(t)
		th.LinkUserToTeam(t, member, team)
		c := newChannelNoAdmin(t, th, team)
		th.AddUserToChannel(t, member, c)

		result, appErr := th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.Nil(t, appErr)
		// >= 1, not ==1: other active channels already in the test server
		// (e.g. InitBasic's defaults) also lack a value for this fresh field
		// and may themselves have no admin.
		assert.GreaterOrEqual(t, result.ChannelsWithoutAdminCount, int64(1))
	})

	t.Run("a channel that already has a value is not counted and its admin is not notified for it", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		bot, appErr := th.App.GetSystemBot(th.Context)
		require.Nil(t, appErr)

		team := th.CreateTeam(t)
		admin := th.CreateUser(t)
		groupID := model.NewId()
		field := makeField("Cost center")

		th.LinkUserToTeam(t, admin, team)
		compliant := newChannelNoAdmin(t, th, team)
		th.AddUserToChannel(t, admin, compliant)
		_, appErr = th.App.UpdateChannelMemberSchemeRoles(th.Context, compliant.Id, admin.Id, false, true, true)
		require.Nil(t, appErr)
		setChannelPropertyValue(t, th, groupID, field.ID, compliant.Id, `"set"`)

		_, appErr = th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.Nil(t, appErr)

		// The load-bearing assertion: this admin's only channel already has a
		// value, so they must never be notified -- regardless of what other
		// channels elsewhere in the test server contribute to the aggregate
		// counts.
		require.Never(t, func() bool {
			return len(dmPostsFromBotToUser(t, th, bot.UserId, admin.Id)) > 0
		}, shortWaitTimeout, waitInterval, "an admin with no non-compliant channel must not be notified")
	})

	t.Run("deactivated and bot channel admins are skipped", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		bot, appErr := th.App.GetSystemBot(th.Context)
		require.Nil(t, appErr)

		team := th.CreateTeam(t)
		groupID := model.NewId()
		field := makeField("Cost center")

		deactivated := th.CreateUser(t)
		th.LinkUserToTeam(t, deactivated, team)
		_, appErr = th.App.UpdateActive(th.Context, deactivated, false)
		require.Nil(t, appErr)

		botUser := th.CreateBot(t)

		c := newChannelNoAdmin(t, th, team)
		th.AddUserToChannel(t, deactivated, c)
		_, appErr = th.App.UpdateChannelMemberSchemeRoles(th.Context, c.Id, deactivated.Id, false, true, true)
		require.Nil(t, appErr)

		_, appErr = th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.Nil(t, appErr)

		// The load-bearing assertion: neither of these two, specifically,
		// must ever be notified -- regardless of what other channels
		// elsewhere in the test server contribute to the aggregate counts.
		require.Never(t, func() bool {
			return len(dmPostsFromBotToUser(t, th, bot.UserId, deactivated.Id)) > 0 || len(dmPostsFromBotToUser(t, th, bot.UserId, botUser.UserId)) > 0
		}, shortWaitTimeout, waitInterval, "deactivated users and bots must never be notified")
	})

	t.Run("a second call inside the cooldown is throttled and sends nothing further", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		bot, appErr := th.App.GetSystemBot(th.Context)
		require.Nil(t, appErr)

		team := th.CreateTeam(t)
		admin := th.CreateUser(t)
		groupID := model.NewId()
		field := makeField("Cost center")

		th.LinkUserToTeam(t, admin, team)
		c := newChannelNoAdmin(t, th, team)
		th.AddUserToChannel(t, admin, c)
		_, appErr = th.App.UpdateChannelMemberSchemeRoles(th.Context, c.Id, admin.Id, false, true, true)
		require.Nil(t, appErr)

		_, appErr = th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.Nil(t, appErr)

		require.Eventually(t, func() bool {
			return len(dmPostsFromBotToUser(t, th, bot.UserId, admin.Id)) == 1
		}, waitTimeout, waitInterval)

		_, appErr = th.App.NotifyChannelAdminsOfMissingAttribute(th.Context, groupID, field)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusTooManyRequests, appErr.StatusCode)

		posts := dmPostsFromBotToUser(t, th, bot.UserId, admin.Id)
		assert.Len(t, posts, 1, "the throttled call must not send a second DM")
	})

	t.Run("a run that delivers to nobody releases the cooldown instead of burning the full hour", func(t *testing.T) {
		th := Setup(t).InitBasic(t)
		fieldID := model.NewId()

		claimed, token, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		require.True(t, claimed, "sanity check: the first claim on a fresh field must succeed")

		stillThrottled, _, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		require.False(t, stillThrottled, "sanity check: a second claim right after the first must be refused")

		// A batch naming a channel ID that cannot resolve to a real direct
		// channel stands in for "every admin's DM failed to send" -- the
		// specific failure mode does not matter, only that delivered ends up
		// at zero despite there being someone to notify.
		batches := map[string]*channelAttributeAdminBatch{
			model.NewId(): {username: "ghost", locale: "en"},
		}
		th.App.sendChannelAttributeNotifyDMs(th.Context, batches, "Cost center", fieldID, token)

		canClaimAgain, _, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		assert.True(t, canClaimAgain, "a fully-failed run must release the cooldown, not leave it claimed for an hour")
	})

	t.Run("releasing a claim that has since been superseded does not delete the newer claim", func(t *testing.T) {
		th := Setup(t).InitBasic(t)
		fieldID := model.NewId()

		claimed, staleToken, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		require.True(t, claimed)

		// Simulate run A's claim aging out (e.g. a stalled goroutine) by
		// directly overwriting the row with an old timestamp, then letting
		// a second, legitimate run B claim fresh.
		require.NoError(t, th.App.Srv().Store().System().Update(&model.System{
			Name:  channelAttributeNotifyThrottleKey(fieldID),
			Value: "0",
		}))
		time.Sleep(2 * time.Millisecond)
		claimedB, tokenB, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		require.True(t, claimedB)
		require.NotEqual(t, staleToken, tokenB)

		// Run A finally gets around to releasing its own (now-stale) claim.
		th.App.releaseChannelAttributeNotifyThrottle(th.Context, fieldID, staleToken)

		// Run B's claim must still stand -- a run C must be throttled.
		claimedC, _, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
		require.Nil(t, appErr)
		assert.False(t, claimedC, "releasing run A's stale token must not delete run B's still-current claim")
	})

	t.Run("concurrent claims on the same field never both succeed", func(t *testing.T) {
		th := Setup(t).InitBasic(t)
		fieldID := model.NewId()

		const attempts = 20
		var wg sync.WaitGroup
		var successes int32
		for range attempts {
			wg.Go(func() {
				claimed, _, appErr := th.App.claimChannelAttributeNotifyThrottle(fieldID)
				require.Nil(t, appErr)
				if claimed {
					atomic.AddInt32(&successes, 1)
				}
			})
		}
		wg.Wait()

		assert.EqualValues(t, 1, successes, "exactly one of many concurrent claims on the same field must win -- a read-then-write check would let more than one through")
	})
}

func TestChannelAttributeDisplayName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuEnterpriseAdvanced))

	group, appErr := th.App.GetPropertyGroup(th.Context, model.AccessControlPropertyGroupName)
	require.Nil(t, appErr)
	groupID := group.ID

	// Read as the system admin the compliance and notify endpoints require.
	rctx := RequestContextWithCallerID(th.Context, th.SystemAdminUser.Id)

	template, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
		GroupID:    groupID,
		Name:       "data_owner",
		Type:       model.PropertyFieldTypeText,
		ObjectType: model.PropertyFieldObjectTypeTemplate,
		TargetType: string(model.PropertyFieldTargetLevelSystem),
		Attrs:      model.StringInterface{model.PropertyFieldAttrDisplayName: "Data Owner"},
		Permissions: &model.Permissions{Restrictions: &model.Restrictions{
			Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
		}},
	})
	require.NoError(t, err)

	linked := func(name string, linkedTo string, attrs model.StringInterface) *model.PropertyField {
		return &model.PropertyField{
			ID:            model.NewId(),
			Name:          name,
			ObjectType:    model.PropertyFieldObjectTypeChannel,
			LinkedFieldID: model.NewPointer(linkedTo),
			Attrs:         attrs,
		}
	}

	t.Run("the field's own display name wins", func(t *testing.T) {
		field := linked("data_owner", template.ID, model.StringInterface{model.PropertyFieldAttrDisplayName: "Owner"})
		assert.Equal(t, "Owner", th.App.channelAttributeDisplayName(rctx, groupID, field))
	})

	t.Run("a linked field without one uses its template's", func(t *testing.T) {
		field := linked("data_owner", template.ID, nil)
		assert.Equal(t, "Data Owner", th.App.channelAttributeDisplayName(rctx, groupID, field))
	})

	t.Run("an unreadable template falls back to the field's name", func(t *testing.T) {
		field := linked("data_owner", model.NewId(), nil)
		assert.Equal(t, "data_owner", th.App.channelAttributeDisplayName(rctx, groupID, field))
	})

	t.Run("classification, whose template has no display name, is labelled Classification", func(t *testing.T) {
		classificationTemplate, err := th.App.Srv().Store().PropertyField().Create(&model.PropertyField{
			GroupID:    groupID,
			Name:       "classification",
			Type:       model.PropertyFieldTypeSelect,
			ObjectType: model.PropertyFieldObjectTypeTemplate,
			TargetType: string(model.PropertyFieldTargetLevelSystem),
			Permissions: &model.Permissions{Restrictions: &model.Restrictions{
				Field: model.WriteOnly{Write: model.PermissionLevelSysadmin},
			}},
		})
		require.NoError(t, err)

		field := linked("classification", classificationTemplate.ID, nil)
		assert.Equal(t, "Classification", th.App.channelAttributeDisplayName(rctx, groupID, field))
	})

	t.Run("the preview names the attribute the same way", func(t *testing.T) {
		field := linked("data_owner", template.ID, nil)
		preview := channelAttributeNotifyMessagePreview(th.App.channelAttributeDisplayName(rctx, groupID, field), "en")
		assert.Contains(t, preview, "**Data Owner**")
	})
}

func TestChannelAttributeNotifyChannelList(t *testing.T) {
	mainHelper.Parallel(t)

	T := i18n.GetUserTranslations("en")

	t.Run("renders each channel as a markdown link bullet", func(t *testing.T) {
		list := channelAttributeNotifyChannelList([]channelAttributeNotifyChannelRef{
			{teamName: "team-a", channelName: "town-square", displayName: "Town Square"},
			{teamName: "team-b", channelName: "off-topic", displayName: "Off-Topic"},
		}, T)

		assert.Equal(t, "\n- [Town Square](/team-a/channels/town-square)\n- [Off-Topic](/team-b/channels/off-topic)", list)
	})

	t.Run("escapes backslashes and brackets in the display name so it cannot redirect the link", func(t *testing.T) {
		list := channelAttributeNotifyChannelList([]channelAttributeNotifyChannelRef{
			{teamName: "team-a", channelName: "sneaky", displayName: "Safe](https://evil.example)[x"},
			{teamName: "team-a", channelName: "slashes", displayName: `trailing\`},
		}, T)

		assert.Equal(t,
			"\n- [Safe\\](https://evil.example)\\[x](/team-a/channels/sneaky)"+
				"\n- [trailing\\\\](/team-a/channels/slashes)",
			list,
		)
	})
}
