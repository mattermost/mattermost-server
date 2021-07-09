// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestImportValidateSchemeImportData(t *testing.T) {
	// Test with minimum required valid properties and team scope.
	data := SchemeImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Scope:       ptrStr("team"),
		DefaultTeamAdminRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
		DefaultTeamUserRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
		DefaultTeamGuestRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
		DefaultChannelAdminRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
		DefaultChannelUserRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
		DefaultChannelGuestRole: &RoleImportData{
			Name:        ptrStr("name"),
			DisplayName: ptrStr("display name"),
			Permissions: &[]string{"invite_user"},
		},
	}

	err := validateSchemeImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with various invalid names.
	data.Name = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	// Test with empty string
	data.Name = ptrStr("")
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	// Test with numbers
	data.Name = ptrStr(strings.Repeat("1234567890", 100))
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	data.Name = ptrStr("name")

	// Test with invalid display name.
	data.DisplayName = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	// Test with display name.
	data.DisplayName = ptrStr("")
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	// Test display name with numbers
	data.DisplayName = ptrStr(strings.Repeat("1234567890", 100))
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	data.DisplayName = ptrStr("display name")

	// Test with various missing roles.
	data.DefaultTeamAdminRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing role.")

	data.DefaultTeamAdminRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}

	data.DefaultTeamUserRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing role.")

	data.DefaultTeamUserRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}
	data.DefaultChannelAdminRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing role.")

	data.DefaultChannelAdminRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}
	data.DefaultChannelUserRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing role.")

	data.DefaultChannelUserRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}

	// Test with various invalid roles.
	data.DefaultTeamAdminRole.Name = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid role.")

	data.DefaultTeamAdminRole.Name = ptrStr("name")
	data.DefaultTeamUserRole.Name = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid role.")

	data.DefaultTeamUserRole.Name = ptrStr("name")
	data.DefaultChannelAdminRole.Name = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid role.")

	data.DefaultChannelAdminRole.Name = ptrStr("name")
	data.DefaultChannelUserRole.Name = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid role.")

	data.DefaultChannelUserRole.Name = ptrStr("name")

	// Change to a Channel scope role, and check with missing or extra roles again.
	data.Scope = ptrStr("channel")
	data.DefaultTeamAdminRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to spurious role.")

	data.DefaultTeamAdminRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}
	data.DefaultTeamUserRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to spurious role.")

	data.DefaultTeamUserRole = &RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
		Permissions: &[]string{"invite_user"},
	}
	data.DefaultTeamGuestRole = nil
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to spurious role.")

	data.DefaultTeamGuestRole = nil
	data.DefaultTeamUserRole = nil
	data.DefaultTeamAdminRole = nil
	err = validateSchemeImportData(&data)
	require.Nil(t, err, "Should have succeeded.")

	// Test with all combinations of optional parameters.
	data.Description = ptrStr(strings.Repeat("1234567890", 1024))
	err = validateSchemeImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid description.")

	data.Description = ptrStr("description")
	err = validateSchemeImportData(&data)
	require.Nil(t, err, "Should have succeeded.")
}

func TestImportValidateRoleImportData(t *testing.T) {
	// Test with minimum required valid properties.
	data := RoleImportData{
		Name:        ptrStr("name"),
		DisplayName: ptrStr("display name"),
	}
	err := validateRoleImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with various invalid names.
	data.Name = nil
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	data.Name = ptrStr("")
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	data.Name = ptrStr(strings.Repeat("1234567890", 100))
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	data.Name = ptrStr("name")

	// Test with invalid display name.
	data.DisplayName = nil
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	data.DisplayName = ptrStr("")
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	data.DisplayName = ptrStr(strings.Repeat("1234567890", 100))
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid display name.")

	data.DisplayName = ptrStr("display name")

	// Test with various valid/invalid permissions.
	data.Permissions = &[]string{}
	err = validateRoleImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	data.Permissions = &[]string{"invite_user", "add_user_to_team"}
	err = validateRoleImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	data.Permissions = &[]string{"invite_user", "add_user_to_team", "derp"}
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid permission.")

	data.Permissions = &[]string{"invite_user", "add_user_to_team"}

	// Test with various valid/invalid descriptions.
	data.Description = ptrStr(strings.Repeat("1234567890", 1024))
	err = validateRoleImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid description.")

	data.Description = ptrStr("description")
	err = validateRoleImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")
}

func TestImportValidateTeamImportData(t *testing.T) {

	// Test with minimum required valid properties.
	data := TeamImportData{
		Name:        ptrStr("teamname"),
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
	}
	err := validateTeamImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with various invalid names.
	data = TeamImportData{
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
	}
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing name.")

	data.Name = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long name.")

	data.Name = ptrStr("login")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to reserved word in name.")

	data.Name = ptrStr("Test::''ASD")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to non alphanum characters in name.")

	data.Name = ptrStr("A")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to short name.")

	// Test team various invalid display names.
	data = TeamImportData{
		Name: ptrStr("teamname"),
		Type: ptrStr("O"),
	}
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing display_name.")

	data.DisplayName = ptrStr("")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to empty display_name.")

	data.DisplayName = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long display_name.")

	// Test with various valid and invalid types.
	data = TeamImportData{
		Name:        ptrStr("teamname"),
		DisplayName: ptrStr("Display Name"),
	}
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing type.")

	data.Type = ptrStr("A")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid type.")

	data.Type = ptrStr("I")
	err = validateTeamImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid type.")

	// Test with all the combinations of optional parameters.
	data = TeamImportData{
		Name:            ptrStr("teamname"),
		DisplayName:     ptrStr("Display Name"),
		Type:            ptrStr("O"),
		Description:     ptrStr("The team description."),
		AllowOpenInvite: ptrBool(true),
	}
	err = validateTeamImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid optional properties.")

	data.AllowOpenInvite = ptrBool(false)
	err = validateTeamImportData(&data)
	require.Nil(t, err, "Should have succeeded with allow open invites false.")

	data.Description = ptrStr(strings.Repeat("abcdefghij ", 26))
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long description.")

	// Test with an empty scheme name.
	data.Description = ptrStr("abcdefg")
	data.Scheme = ptrStr("")
	err = validateTeamImportData(&data)
	require.NotNil(t, err, "Should have failed due to empty scheme name.")

	// Test with a valid scheme name.
	data.Scheme = ptrStr("abcdefg")
	err = validateTeamImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid scheme name.")
}

func TestImportValidateChannelImportData(t *testing.T) {

	// Test with minimum required valid properties.
	data := ChannelImportData{
		Team:        ptrStr("teamname"),
		Name:        ptrStr("channelname"),
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
	}
	err := validateChannelImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with missing team.
	data = ChannelImportData{
		Name:        ptrStr("channelname"),
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
	}
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing team.")

	// Test with various invalid names.
	data = ChannelImportData{
		Team:        ptrStr("teamname"),
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
	}
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing name.")

	data.Name = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long name.")

	data.Name = ptrStr("Test::''ASD")
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to non alphanum characters in name.")

	data.Name = ptrStr("A")
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to short name.")

	// Test team various invalid display names.
	data = ChannelImportData{
		Team: ptrStr("teamname"),
		Name: ptrStr("channelname"),
		Type: ptrStr("O"),
	}
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing display_name.")

	data.DisplayName = ptrStr("")
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to empty display_name.")

	data.DisplayName = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long display_name.")

	// Test with various valid and invalid types.
	data = ChannelImportData{
		Team:        ptrStr("teamname"),
		Name:        ptrStr("channelname"),
		DisplayName: ptrStr("Display Name"),
	}
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to missing type.")

	data.Type = ptrStr("A")
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid type.")

	data.Type = ptrStr("P")
	err = validateChannelImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid type.")

	// Test with all the combinations of optional parameters.
	data = ChannelImportData{
		Team:        ptrStr("teamname"),
		Name:        ptrStr("channelname"),
		DisplayName: ptrStr("Display Name"),
		Type:        ptrStr("O"),
		Header:      ptrStr("Channel Header Here"),
		Purpose:     ptrStr("Channel Purpose Here"),
	}
	err = validateChannelImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid optional properties.")

	data.Header = ptrStr(strings.Repeat("abcdefghij ", 103))
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long header.")

	data.Header = ptrStr("Channel Header Here")
	data.Purpose = ptrStr(strings.Repeat("abcdefghij ", 26))
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long purpose.")

	// Test with an empty scheme name.
	data.Purpose = ptrStr("abcdefg")
	data.Scheme = ptrStr("")
	err = validateChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to empty scheme name.")

	// Test with a valid scheme name.
	data.Scheme = ptrStr("abcdefg")
	err = validateChannelImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid scheme name.")
}

func TestImportValidateUserImportData(t *testing.T) {

	// Test with minimum required valid properties.
	data := UserImportData{
		Username: ptrStr("bob"),
		Email:    ptrStr("bob@example.com"),
	}
	err := validateUserImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Invalid Usernames.
	data.Username = nil
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to nil Username.")

	data.Username = ptrStr("")
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to 0 length Username.")

	data.Username = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long Username.")

	data.Username = ptrStr("i am a username with spaces and !!!")
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid characters in Username.")

	data.Username = ptrStr("bob")

	// Unexisting Picture Image
	data.ProfileImage = ptrStr("not-existing-file")
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to not existing profile image file.")

	data.ProfileImage = nil

	// Invalid Emails
	data.Email = nil
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to nil Email.")

	data.Email = ptrStr("")
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to 0 length Email.")

	data.Email = ptrStr(strings.Repeat("abcdefghij", 13))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long Email.")

	data.Email = ptrStr("bob@example.com")

	// Empty AuthService indicates user/password auth.
	data.AuthService = ptrStr("")
	checkNoError(t, validateUserImportData(&data))

	data.AuthService = ptrStr("saml")
	data.AuthData = ptrStr(strings.Repeat("abcdefghij", 15))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long auth data.")

	data.AuthData = ptrStr("bobbytables")
	err = validateUserImportData(&data)
	require.Nil(t, err, "Validation should have succeeded with valid auth service and auth data.")

	// Test a valid User with all fields populated.
	testsDir, _ := fileutils.FindDir("tests")
	data = UserImportData{
		ProfileImage: ptrStr(filepath.Join(testsDir, "test.png")),
		Username:     ptrStr("bob"),
		Email:        ptrStr("bob@example.com"),
		AuthService:  ptrStr("ldap"),
		AuthData:     ptrStr("bob"),
		Nickname:     ptrStr("BobNick"),
		FirstName:    ptrStr("Bob"),
		LastName:     ptrStr("Blob"),
		Position:     ptrStr("The Boss"),
		Roles:        ptrStr("system_user"),
		Locale:       ptrStr("en"),
	}
	err = validateUserImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test various invalid optional field values.
	data.Nickname = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long Nickname.")

	data.Nickname = ptrStr("BobNick")

	data.FirstName = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long First Name.")

	data.FirstName = ptrStr("Bob")

	data.LastName = ptrStr(strings.Repeat("abcdefghij", 7))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long Last name.")

	data.LastName = ptrStr("Blob")

	data.Position = ptrStr(strings.Repeat("abcdefghij", 13))
	err = validateUserImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to too long Position.")

	data.Position = ptrStr("The Boss")

	data.Roles = nil
	err = validateUserImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	data.Roles = ptrStr("")
	err = validateUserImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	data.Roles = ptrStr("system_user")

	// Try various valid/invalid notify props.
	data.NotifyProps = &UserNotifyPropsImportData{}

	data.NotifyProps.Desktop = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.Desktop = ptrStr(model.UserNotifyAll)
	data.NotifyProps.DesktopSound = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.DesktopSound = ptrStr("true")
	data.NotifyProps.Email = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.Email = ptrStr("true")
	data.NotifyProps.Mobile = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.Mobile = ptrStr(model.UserNotifyAll)
	data.NotifyProps.MobilePushStatus = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.MobilePushStatus = ptrStr(model.StatusOnline)
	data.NotifyProps.ChannelTrigger = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.ChannelTrigger = ptrStr("true")
	data.NotifyProps.CommentsTrigger = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.NotifyProps.CommentsTrigger = ptrStr(model.CommentsNotifyRoot)
	data.NotifyProps.MentionKeys = ptrStr("valid")
	checkNoError(t, validateUserImportData(&data))

	//Test the emai batching interval validators
	//Happy paths
	data.EmailInterval = ptrStr("immediately")
	checkNoError(t, validateUserImportData(&data))

	data.EmailInterval = ptrStr("fifteen")
	checkNoError(t, validateUserImportData(&data))

	data.EmailInterval = ptrStr("hour")
	checkNoError(t, validateUserImportData(&data))

	//Invalid values
	data.EmailInterval = ptrStr("invalid")
	checkError(t, validateUserImportData(&data))

	data.EmailInterval = ptrStr("")
	checkError(t, validateUserImportData(&data))
}

func TestImportValidateUserAuth(t *testing.T) {
	tests := []struct {
		authService *string
		authData    *string
		isValid     bool
	}{
		{nil, nil, true},
		{ptrStr(""), ptrStr(""), true},
		{ptrStr("foo"), ptrStr("foo"), true},
		{nil, ptrStr(""), true},
		{ptrStr(""), nil, true},

		{ptrStr("foo"), nil, false},
		{ptrStr("foo"), ptrStr(""), false},
		{nil, ptrStr("foo"), false},
		{ptrStr(""), ptrStr("foo"), false},
	}

	for _, test := range tests {
		data := UserImportData{
			Username:    ptrStr("bob"),
			Email:       ptrStr("bob@example.com"),
			AuthService: test.authService,
			AuthData:    test.authData,
		}
		err := validateUserImportData(&data)

		if test.isValid {
			require.Nil(t, err, fmt.Sprintf("authService: %v, authData: %v", test.authService, test.authData))
		} else {
			require.NotNil(t, err, fmt.Sprintf("authService: %v, authData: %v", test.authService, test.authData))
			require.Equal(t, "app.import.validate_user_import_data.auth_data_and_service_dependency.error", err.ID)
		}
	}

}

func TestImportValidateUserTeamsImportData(t *testing.T) {

	// Invalid Name.
	data := []UserTeamImportData{
		{
			Roles: ptrStr("team_admin team_user"),
		},
	}
	err := validateUserTeamsImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")

	data[0].Name = ptrStr("teamname")

	// Valid (nil roles)
	data[0].Roles = nil
	err = validateUserTeamsImportData(&data)
	require.Nil(t, err, "Should have succeeded with empty roles.")

	// Valid (empty roles)
	data[0].Roles = ptrStr("")
	err = validateUserTeamsImportData(&data)
	require.Nil(t, err, "Should have succeeded with empty roles.")

	// Valid (with roles)
	data[0].Roles = ptrStr("team_admin team_user")
	err = validateUserTeamsImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid roles.")

	// Valid (with JSON string of theme)
	data[0].Theme = ptrStr(`{"awayIndicator":"#DBBD4E","buttonBg":"#23A1FF","buttonColor":"#FFFFFF","centerChannelBg":"#ffffff","centerChannelColor":"#333333","codeTheme":"github","image":"/static/files/a4a388b38b32678e83823ef1b3e17766.png","linkColor":"#2389d7","mentionBg":"#2389d7","mentionColor":"#ffffff","mentionHighlightBg":"#fff2bb","mentionHighlightLink":"#2f81b7","newMessageSeparator":"#FF8800","onlineIndicator":"#7DBE00","sidebarBg":"#fafafa","sidebarHeaderBg":"#3481B9","sidebarHeaderTextColor":"#ffffff","sidebarText":"#333333","sidebarTextActiveBorder":"#378FD2","sidebarTextActiveColor":"#111111","sidebarTextHoverBg":"#e6f2fa","sidebarUnreadText":"#333333","type":"Mattermost"}`)
	err = validateUserTeamsImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid theme.")

	// Invalid (invalid JSON string of theme)
	data[0].Theme = ptrStr(`This is the invalid string which cannot be marshalled to JSON object :) + {"#DBBD4E","buttonBg", "#23A1FF", buttonColor`)
	err = validateUserTeamsImportData(&data)
	require.NotNil(t, err, "Should have fail with invalid JSON string of theme.")

	// Invalid (valid JSON but invalid theme description)
	data[0].Theme = ptrStr(`{"somekey": 25, "json_obj1": {"color": "#DBBD4E","buttonBg": "#23A1FF"}}`)
	err = validateUserTeamsImportData(&data)
	require.NotNil(t, err, "Should have fail with valid JSON which contains invalid string of theme description.")

	data[0].Theme = nil
}

func TestImportValidateUserChannelsImportData(t *testing.T) {

	// Invalid Name.
	data := []UserChannelImportData{
		{
			Roles: ptrStr("channel_admin channel_user"),
		},
	}
	err := validateUserChannelsImportData(&data)
	require.NotNil(t, err, "Should have failed due to invalid name.")
	data[0].Name = ptrStr("channelname")

	// Valid (nil roles)
	data[0].Roles = nil
	err = validateUserChannelsImportData(&data)
	require.Nil(t, err, "Should have succeeded with empty roles.")

	// Valid (empty roles)
	data[0].Roles = ptrStr("")
	err = validateUserChannelsImportData(&data)
	require.Nil(t, err, "Should have succeeded with empty roles.")

	// Valid (with roles)
	data[0].Roles = ptrStr("channel_admin channel_user")
	err = validateUserChannelsImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid roles.")

	// Empty notify props.
	data[0].NotifyProps = &UserChannelNotifyPropsImportData{}
	err = validateUserChannelsImportData(&data)
	require.Nil(t, err, "Should have succeeded with empty notify props.")

	// Invalid desktop notify props.
	data[0].NotifyProps.Desktop = ptrStr("invalid")
	err = validateUserChannelsImportData(&data)
	require.NotNil(t, err, "Should have failed with invalid desktop notify props.")

	// Invalid mobile notify props.
	data[0].NotifyProps.Desktop = ptrStr("mention")
	data[0].NotifyProps.Mobile = ptrStr("invalid")
	err = validateUserChannelsImportData(&data)
	require.NotNil(t, err, "Should have failed with invalid mobile notify props.")

	// Invalid mark_unread notify props.
	data[0].NotifyProps.Mobile = ptrStr("mention")
	data[0].NotifyProps.MarkUnread = ptrStr("invalid")
	err = validateUserChannelsImportData(&data)
	require.NotNil(t, err, "Should have failed with invalid mark_unread notify props.")

	// Valid notify props.
	data[0].NotifyProps.MarkUnread = ptrStr("mention")
	err = validateUserChannelsImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid notify props.")
}

func TestImportValidateReactionImportData(t *testing.T) {
	// Test with minimum required valid properties.
	parentCreateAt := model.GetMillis() - 100
	data := ReactionImportData{
		User:      ptrStr("username"),
		EmojiName: ptrStr("emoji"),
		CreateAt:  ptrInt64(model.GetMillis()),
	}
	err := validateReactionImportData(&data, parentCreateAt)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with missing required properties.
	data = ReactionImportData{
		EmojiName: ptrStr("emoji"),
		CreateAt:  ptrInt64(model.GetMillis()),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = ReactionImportData{
		User:     ptrStr("username"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = ReactionImportData{
		User:      ptrStr("username"),
		EmojiName: ptrStr("emoji"),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	// Test with invalid emoji name.
	data = ReactionImportData{
		User:      ptrStr("username"),
		EmojiName: ptrStr(strings.Repeat("1234567890", 500)),
		CreateAt:  ptrInt64(model.GetMillis()),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due to too long emoji name.")

	// Test with invalid CreateAt
	data = ReactionImportData{
		User:      ptrStr("username"),
		EmojiName: ptrStr("emoji"),
		CreateAt:  ptrInt64(0),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due to 0 create-at value.")

	data = ReactionImportData{
		User:      ptrStr("username"),
		EmojiName: ptrStr("emoji"),
		CreateAt:  ptrInt64(parentCreateAt - 100),
	}
	err = validateReactionImportData(&data, parentCreateAt)
	require.NotNil(t, err, "Should have failed due parent with newer create-at value.")
}

func TestImportValidateReplyImportData(t *testing.T) {
	// Test with minimum required valid properties.
	parentCreateAt := model.GetMillis() - 100
	maxPostSize := 10000
	data := ReplyImportData{
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err := validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with missing required properties.
	data = ReplyImportData{
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = ReplyImportData{
		User:     ptrStr("username"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = ReplyImportData{
		User:    ptrStr("username"),
		Message: ptrStr("message"),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	// Test with invalid message.
	data = ReplyImportData{
		User:     ptrStr("username"),
		Message:  ptrStr(strings.Repeat("0", maxPostSize+1)),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due to too long message.")

	// Test with invalid CreateAt
	data = ReplyImportData{
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(0),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due to 0 create-at value.")

	data = ReplyImportData{
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(parentCreateAt - 100),
	}
	err = validateReplyImportData(&data, parentCreateAt, maxPostSize)
	require.NotNil(t, err, "Should have failed due parent with newer create-at value.")
}

func TestImportValidatePostImportData(t *testing.T) {
	maxPostSize := 10000

	t.Run("Test with minimum required valid properties", func(t *testing.T) {
		data := PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err := validatePostImportData(&data, maxPostSize)
		require.Nil(t, err, "Validation failed but should have been valid.")
	})

	t.Run("Test with missing required properties", func(t *testing.T) {
		data := PostImportData{
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err := validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to missing required property.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.team_missing.error")

		data = PostImportData{
			Team:     ptrStr("teamname"),
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err = validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to missing required property.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.channel_missing.error")

		data = PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err = validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to missing required property.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.user_missing.error")

		data = PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err = validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to missing required property.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.message_missing.error")

		data = PostImportData{
			Team:    ptrStr("teamname"),
			Channel: ptrStr("channelname"),
			User:    ptrStr("username"),
			Message: ptrStr("message"),
		}
		err = validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to missing required property.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.create_at_missing.error")
	})

	t.Run("Test with invalid message", func(t *testing.T) {
		data := PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			Message:  ptrStr(strings.Repeat("0", maxPostSize+1)),
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err := validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to too long message.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.message_length.error")
	})

	t.Run("Test with invalid CreateAt", func(t *testing.T) {
		data := PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(0),
		}
		err := validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to 0 create-at value.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.create_at_zero.error")
	})

	t.Run("Test with valid all optional parameters", func(t *testing.T) {
		reactions := []ReactionImportData{{
			User:      ptrStr("username"),
			EmojiName: ptrStr("emoji"),
			CreateAt:  ptrInt64(model.GetMillis()),
		}}

		replies := []ReplyImportData{{
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			CreateAt: ptrInt64(model.GetMillis()),
		}}

		data := PostImportData{
			Team:      ptrStr("teamname"),
			Channel:   ptrStr("channelname"),
			User:      ptrStr("username"),
			Message:   ptrStr("message"),
			CreateAt:  ptrInt64(model.GetMillis()),
			Reactions: &reactions,
			Replies:   &replies,
		}
		err := validatePostImportData(&data, maxPostSize)
		require.Nil(t, err, "Should have succeeded.")
	})

	t.Run("Test with props too large", func(t *testing.T) {
		props := model.StringInterface{
			"attachment": strings.Repeat("a", model.PostPropsMaxRunes),
		}

		data := PostImportData{
			Team:     ptrStr("teamname"),
			Channel:  ptrStr("channelname"),
			User:     ptrStr("username"),
			Message:  ptrStr("message"),
			Props:    &props,
			CreateAt: ptrInt64(model.GetMillis()),
		}
		err := validatePostImportData(&data, maxPostSize)
		require.NotNil(t, err, "Should have failed due to long props.")
		assert.Equal(t, err.ID, "app.import.validate_post_import_data.props_too_large.error")
	})
}

func TestImportValidateDirectChannelImportData(t *testing.T) {

	// Test with valid number of members for direct message.
	data := DirectChannelImportData{
		Members: &[]string{
			model.NewID(),
			model.NewID(),
		},
	}
	err := validateDirectChannelImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with valid number of members for group message.
	data = DirectChannelImportData{
		Members: &[]string{
			model.NewID(),
			model.NewID(),
			model.NewID(),
		},
	}
	err = validateDirectChannelImportData(&data)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with all the combinations of optional parameters.
	data = DirectChannelImportData{
		Members: &[]string{
			model.NewID(),
			model.NewID(),
		},
		Header: ptrStr("Channel Header Here"),
	}
	err = validateDirectChannelImportData(&data)
	require.Nil(t, err, "Should have succeeded with valid optional properties.")

	// Test with invalid Header.
	data.Header = ptrStr(strings.Repeat("abcdefghij ", 103))
	err = validateDirectChannelImportData(&data)
	require.NotNil(t, err, "Should have failed due to too long header.")

	// Test with different combinations of invalid member counts.
	data = DirectChannelImportData{
		Members: &[]string{},
	}
	err = validateDirectChannelImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid number of members.")

	data = DirectChannelImportData{
		Members: &[]string{
			model.NewID(),
		},
	}
	err = validateDirectChannelImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid number of members.")

	data = DirectChannelImportData{
		Members: &[]string{
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
		},
	}
	err = validateDirectChannelImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to invalid number of members.")

	// Test with invalid FavoritedBy
	member1 := model.NewID()
	member2 := model.NewID()
	data = DirectChannelImportData{
		Members: &[]string{
			member1,
			member2,
		},
		FavoritedBy: &[]string{
			member1,
			model.NewID(),
		},
	}
	err = validateDirectChannelImportData(&data)
	require.NotNil(t, err, "Validation should have failed due to non-member favorited.")

	// Test with valid FavoritedBy
	data = DirectChannelImportData{
		Members: &[]string{
			member1,
			member2,
		},
		FavoritedBy: &[]string{
			member1,
			member2,
		},
	}
	err = validateDirectChannelImportData(&data)
	require.Nil(t, err, "Validation should succeed with valid favorited member")
}

func TestImportValidateDirectPostImportData(t *testing.T) {
	maxPostSize := 10000

	// Test with minimum required valid properties.
	data := DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err := validateDirectPostImportData(&data, maxPostSize)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with missing required properties.
	data = DirectPostImportData{
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		User:    ptrStr("username"),
		Message: ptrStr("message"),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to missing required property.")

	// Test with invalid numbers of channel members.
	data = DirectPostImportData{
		ChannelMembers: &[]string{},
		User:           ptrStr("username"),
		Message:        ptrStr("message"),
		CreateAt:       ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to unsuitable number of members.")

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to unsuitable number of members.")

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to unsuitable number of members.")

	// Test with group message number of members.
	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.Nil(t, err, "Validation failed but should have been valid.")

	// Test with invalid message.
	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr(strings.Repeat("0", maxPostSize+1)),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to too long message.")

	// Test with invalid CreateAt
	data = DirectPostImportData{
		ChannelMembers: &[]string{
			model.NewID(),
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(0),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Should have failed due to 0 create-at value.")

	// Test with invalid FlaggedBy
	member1 := model.NewID()
	member2 := model.NewID()
	data = DirectPostImportData{
		ChannelMembers: &[]string{
			member1,
			member2,
		},
		FlaggedBy: &[]string{
			member1,
			model.NewID(),
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.NotNil(t, err, "Validation should have failed due to non-member flagged.")

	// Test with valid FlaggedBy
	data = DirectPostImportData{
		ChannelMembers: &[]string{
			member1,
			member2,
		},
		FlaggedBy: &[]string{
			member1,
			member2,
		},
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}
	err = validateDirectPostImportData(&data, maxPostSize)
	require.Nil(t, err, "Validation should succeed with post flagged by members")

	// Test with valid all optional parameters.
	reactions := []ReactionImportData{{
		User:      ptrStr("username"),
		EmojiName: ptrStr("emoji"),
		CreateAt:  ptrInt64(model.GetMillis()),
	}}

	replies := []ReplyImportData{{
		User:     ptrStr("username"),
		Message:  ptrStr("message"),
		CreateAt: ptrInt64(model.GetMillis()),
	}}

	data = DirectPostImportData{
		ChannelMembers: &[]string{
			member1,
			member2,
		},
		FlaggedBy: &[]string{
			member1,
			member2,
		},
		User:      ptrStr("username"),
		Message:   ptrStr("message"),
		CreateAt:  ptrInt64(model.GetMillis()),
		Reactions: &reactions,
		Replies:   &replies,
	}

	err = validateDirectPostImportData(&data, maxPostSize)
	require.Nil(t, err, "Validation should succeed with valid optional parameters")
}

func TestImportValidateEmojiImportData(t *testing.T) {
	data := EmojiImportData{
		Name:  ptrStr("parrot2"),
		Image: ptrStr("/path/to/image"),
	}

	err := validateEmojiImportData(&data)
	assert.Nil(t, err, "Validation should succeed")

	*data.Name = "smiley"
	err = validateEmojiImportData(&data)
	assert.NotNil(t, err)

	*data.Name = ""
	err = validateEmojiImportData(&data)
	assert.NotNil(t, err)

	*data.Name = ""
	*data.Image = ""
	err = validateEmojiImportData(&data)
	assert.NotNil(t, err)

	*data.Image = "/path/to/image"
	data.Name = nil
	err = validateEmojiImportData(&data)
	assert.NotNil(t, err)

	data.Name = ptrStr("parrot")
	data.Image = nil
	err = validateEmojiImportData(&data)
	assert.NotNil(t, err)
}
