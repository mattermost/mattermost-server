// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
)

var spPrivateKey = `-----BEGIN PRIVATE KEY-----
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
kSU80FrXITorOgZogkDk0FVpJA3qvQ==
-----END PRIVATE KEY-----`

var spPublicCertificate = `-----BEGIN CERTIFICATE-----
MIIFijCCA3KgAwIBAgIJAIRQ3EwrvOprMA0GCSqGSIb3DQEBCwUAMFwxCzAJBgNV
BAYTAlVTMRIwEAYDVQQHDAlQYWxvIEFsdG8xEzARBgNVBAoMCk1hdHRlcm1vc3Qx
DzANBgNVBAsMBkRldk9wczETMBEGA1UEAwwKY2xpZW50LmNvbTAeFw0xOTA5MTIx
NzM1MzdaFw0yOTA5MDkxNzM1MzdaMFwxCzAJBgNVBAYTAlVTMRIwEAYDVQQHDAlQ
YWxvIEFsdG8xEzARBgNVBAoMCk1hdHRlcm1vc3QxDzANBgNVBAsMBkRldk9wczET
MBEGA1UEAwwKY2xpZW50LmNvbTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoC
ggIBANtVtR87yAUOCrHDdnuBfmVrJMopdBPrbeHR5Qdej3mMh0CkzIJ2FFihjP05
S+jTVpDRTIStzJYpQkTcTRgQ/8yO8EZMh6aaSP81Ba0uojFlkWeXy5VybmA9ZMmA
SHPL2Lj8Pj2qiVhqQk5WQr5+cBZTMiI7kYgzoi3zO4Mgr1ryWcsroIZaFZQNHyzq
eXE6aIbfYtEo8A4muXE09sMk7lJX9aS17/VR1UJH1xbkWugnlp79xQSkx42mIUwJ
zj2AM8Q4s5MCuhizVs6FLiYYCuU7RZlTPftzJlKj8y1YmjZjtuRV6MC3ql0xya9g
3LnJ12uyUD7Q6K15lLh03TW0drhCIRo1Vn4N0pZ+kkViqpU02r9gy95SOTshuecm
nP7KU5zzM2d6iLATkIj6aYWnRfT5m3UgkPsFdkTDfsmGT5h4F1BSeQc9YoePh2Qo
+l+M9f8rWvYbUJjuIMbdXV8+wgUB8JS8ttYf33iFp7PP1T6Fe7z0uaGcqrrfx8N2
4ZgpA7JjPz8GxDUxA/mQKsODkeGepTH90QISUWUcKerdn+AehA6JzEDx/UZ4IOIg
TZUVhlFAsMvbWVv2jNAEVstll8ZosaZQ+F5xDRGtD9bhg82EG/JcQs6MKwN+7qW2
ehvoTwu7Huw2iEW4tpiDQDMNF97OFTRGpZdREqWmKobSpqhbAgMBAAGjTzBNMBIG
A1UdEwEB/wQIMAYBAf8CAQAwNwYDVR0RBDAwLoIOd3d3LmNsaWVudC5jb22CEGFk
bWluLmNsaWVudC5jb22HBMCoAQqHBAoAAOowDQYJKoZIhvcNAQELBQADggIBAFEI
D1ySRS+lQYVm24PPIUH5OmBEJUsVKI/zUXEQ4hdqEqN4UA3NGKkujajTz2fStaOj
LfGDup1ZQRYG6VVvNwbZHX9G9mb8TyZ12XFLVjPTbxoG+NZb3ipue9S6qZcT9WEF
sjaXhkVNhhVc1GOMnv/FNiclLPWLMnR8WST+Y+WSsT59wP40kJynaT7wQt2TmImg
kQfM69jQNgAkyrFwO8y1YcnH7Avrw9YvzhUWG2FfNCTTVNb+StxNtqGwvDV33iZ2
bBUWIy2fsNUA4tUYK31Ye6thJiKmvy/LqVJ415gPsI3zHzTCLU/GBUCNCNnEDnhU
KO2K3mk1wK3sshMGcda/Xz2a9TfkIxs0pkenS57bZ8xT7mxBzXsZGm7Mnb2fujmX
fBEyxQ2ot0Nl9Lp26WrBjQZojJ10Ic2IRxU3spC/FYK7BenQEAdnNHkyQ3lowAto
NpOL+j+1ooksPQbp4DeIBbrZDNKvFot+ja2aDJ738sgXf8ht7kGXA5DPNtPLsmUr
wpZrhxKD6pXVPhA6EeG2efdUP1ODslmehl4t2yX+FqHChnl7E012W8Cf0Ugybp1t
15IXg8GxCRENSNAwpOvTMkoonHqNvBkaCDZHtxeyJMJWQW1B0Xek1JY3CNHvnY7I
MCOV5SHi05kD42JSSbmw190VAa4QRGikaeWRhDsj
-----END CERTIFICATE-----`

func TestTestLdap(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.TestLdap()
		CheckNotImplementedStatus(t, resp)
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.ldap_groups.license_error", resp.Error.ID)
	})
	th.App.Srv().SetLicense(model.NewTestLicense("ldap_groups"))

	_, resp := th.Client.TestLdap()
	CheckForbiddenStatus(t, resp)
	require.NotNil(t, resp.Error)
	require.Equal(t, "api.context.permissions.app_error", resp.Error.ID)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.TestLdap()
		CheckNotImplementedStatus(t, resp)
		require.NotNil(t, resp.Error)
		require.Equal(t, "ent.ldap.disabled.app_error", resp.Error.ID)
	})
}

func TestSyncLdap(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.TestLdap()
		CheckNotImplementedStatus(t, resp)
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.ldap_groups.license_error", resp.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap_groups"))
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.LdapSettings.EnableSync = true
	})

	ldapMock := &mocks.LdapInterface{}
	mockCall := ldapMock.On(
		"StartSynchronizeJob",
		mock.AnythingOfType("bool"),
		mock.AnythingOfType("bool"),
	).Return(nil, nil)
	ready := make(chan bool)
	includeRemovedMembers := false
	mockCall.RunFn = func(args mock.Arguments) {
		includeRemovedMembers = args[1].(bool)
		ready <- true
	}
	th.App.Srv().Ldap = ldapMock

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.SyncLdap(false)
		<-ready
		CheckNoError(t, resp)
		require.Equal(t, false, includeRemovedMembers)

		_, resp = client.SyncLdap(true)
		<-ready
		CheckNoError(t, resp)
		require.Equal(t, true, includeRemovedMembers)
	})

	_, resp := th.Client.SyncLdap(false)
	CheckForbiddenStatus(t, resp)
}

func TestGetLdapGroups(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.GetLdapGroups()
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.GetLdapGroups()
		CheckNotImplementedStatus(t, resp)
	})
}

func TestLinkLdapGroup(t *testing.T) {
	const entryUUID string = "foo"

	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.LinkLdapGroup(entryUUID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.LinkLdapGroup(entryUUID)
	CheckNotImplementedStatus(t, resp)
}

func TestUnlinkLdapGroup(t *testing.T) {
	const entryUUID string = "foo"

	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.UnlinkLdapGroup(entryUUID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.UnlinkLdapGroup(entryUUID)
	CheckNotImplementedStatus(t, resp)
}

func TestMigrateIDLdap(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.MigrateIDLdap("objectGUID")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.MigrateIDLdap("")
		CheckBadRequestStatus(t, resp)

		_, resp = client.MigrateIDLdap("objectGUID")
		CheckNotImplementedStatus(t, resp)
	})
}

func TestUploadPublicCertificate(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.UploadLdapPublicCertificate([]byte(spPublicCertificate))
	require.NotNil(t, resp.Error, "Should have failed. No System Admin privileges")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.UploadLdapPublicCertificate([]byte(spPrivateKey))
		require.Nil(t, resp.Error, "Should have passed. System Admin privileges %v", resp.Error)
	})

	_, resp = th.Client.DeleteLdapPublicCertificate()
	require.NotNil(t, resp.Error, "Should have failed. No System Admin privileges")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.DeleteLdapPublicCertificate()
		require.Nil(t, resp.Error, "Should have passed. System Admin privileges %v", resp.Error)
	})
}

func TestUploadPrivateCertificate(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	_, resp := th.Client.UploadLdapPrivateCertificate([]byte(spPrivateKey))
	require.NotNil(t, resp.Error, "Should have failed. No System Admin privileges")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.UploadLdapPrivateCertificate([]byte(spPrivateKey))
		require.Nil(t, resp.Error, "Should have passed. System Admin privileges %v", resp.Error)
	})

	_, resp = th.Client.DeleteLdapPrivateCertificate()
	require.NotNil(t, resp.Error, "Should have failed. No System Admin privileges")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.DeleteLdapPrivateCertificate()
		require.Nil(t, resp.Error, "Should have passed. System Admin privileges %v", resp.Error)
	})
}
