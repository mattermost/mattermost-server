// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

var publicKey []byte = []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwVMaUtQMFtQsRoa2FQd+
17+acMRMqzPsFTJEix7n+tA8ieCAX0lvNBOjVh8lrt/AUe7B3ZJ0HE6v8xuKj9YA
zUqV8R27UmnIxf5TCXFWrt+fnzX31yLjqK4Fd6JmiDheUatd1hG+gkScMAPuj+Xw
4O+V5sMwaVusytVgmehqcVKYrKpDyhBDUEbDRbKKbP4YAHbmNs3AzlGBbQtvc1fi
0ww7oNKs9cZVcCIeNdwbkXmf8pK5zSXqVsmyPyH3Ue8M6JMNGuPGh1fyHRhFdIiD
lxd7LBKSh7BTjbRtG9DEt1dyMnYQDgRVAdpururuK/otowCowr6X/Etnk2NNeXcZ
4QIDAQAB
-----END PUBLIC KEY-----`)

var LicenseValidator LicenseValidatorIface

func init() {
	if LicenseValidator == nil {
		LicenseValidator = &LicenseValidatorImpl{}
	}
}

type LicenseValidatorIface interface {
	LicenseFromBytes(licenseBytes []byte) (*model.License, *model.AppError)
	ValidateLicense(signed []byte) (bool, string)
}

type LicenseValidatorImpl struct {
}

func (l *LicenseValidatorImpl) LicenseFromBytes(licenseBytes []byte) (*model.License, *model.AppError) {
	success, licenseStr := l.ValidateLicense(licenseBytes)
	if !success {
		return nil, model.NewAppError("LicenseFromBytes", model.INVALID_LICENSE_ERROR, nil, "", http.StatusBadRequest)
	}

	license := model.LicenseFromJson(strings.NewReader(licenseStr))
	return license, nil
}

func (l *LicenseValidatorImpl) ValidateLicense(signed []byte) (bool, string) {
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(signed)))

	_, err := base64.StdEncoding.Decode(decoded, signed)
	if err != nil {
		mlog.Error("Encountered error decoding license", mlog.Err(err))
		return false, ""
	}

	if len(decoded) <= 256 {
		mlog.Error("Signed license not long enough")
		return false, ""
	}

	// remove null terminator
	for decoded[len(decoded)-1] == byte(0) {
		decoded = decoded[:len(decoded)-1]
	}

	plaintext := decoded[:len(decoded)-256]
	signature := decoded[len(decoded)-256:]

	block, _ := pem.Decode(publicKey)

	public, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		mlog.Error("Encountered error signing license", mlog.Err(err))
		return false, ""
	}

	rsaPublic := public.(*rsa.PublicKey)

	h := sha512.New()
	h.Write(plaintext)
	d := h.Sum(nil)

	err = rsa.VerifyPKCS1v15(rsaPublic, crypto.SHA512, d, signature)
	if err != nil {
		mlog.Error("Invalid signature", mlog.Err(err))
		return false, ""
	}

	return true, string(plaintext)
}

func GetAndValidateLicenseFileFromDisk(location string) (*model.License, []byte) {
	fileName := GetLicenseFileLocation(location)

	if _, err := os.Stat(fileName); err != nil {
		mlog.Debug("We could not find the license key in the database or on disk at", mlog.String("filename", fileName))
		return nil, nil
	}

	mlog.Info("License key has not been uploaded.  Loading license key from disk at", mlog.String("filename", fileName))
	licenseBytes := GetLicenseFileFromDisk(fileName)

	success, licenseStr := LicenseValidator.ValidateLicense(licenseBytes)
	if !success {
		mlog.Error("Found license key at %v but it appears to be invalid.", mlog.String("filename", fileName))
		return nil, nil
	}
	return model.LicenseFromJson(strings.NewReader(licenseStr)), licenseBytes
}

func GetLicenseFileFromDisk(fileName string) []byte {
	file, err := os.Open(fileName)
	if err != nil {
		mlog.Error("Failed to open license key from disk at", mlog.String("filename", fileName), mlog.Err(err))
		return nil
	}
	defer file.Close()

	licenseBytes, err := ioutil.ReadAll(file)
	if err != nil {
		mlog.Error("Failed to read license key from disk at", mlog.String("filename", fileName), mlog.Err(err))
		return nil
	}

	return licenseBytes
}

func GetLicenseFileLocation(fileLocation string) string {
	if fileLocation == "" {
		configDir, _ := fileutils.FindDir("config")
		return filepath.Join(configDir, "mattermost.mattermost-license")
	}
	return fileLocation
}

func GetClientLicense(l *model.License) map[string]string {
	props := make(map[string]string)

	props["IsLicensed"] = strconv.FormatBool(l != nil)

	if l != nil {
		props["Id"] = l.Id
		props["SkuName"] = l.SkuName
		props["SkuShortName"] = l.SkuShortName
		props["Users"] = strconv.Itoa(*l.Features.Users)
		props["LDAP"] = strconv.FormatBool(*l.Features.LDAP)
		props["LDAPGroups"] = strconv.FormatBool(*l.Features.LDAPGroups)
		props["MFA"] = strconv.FormatBool(*l.Features.MFA)
		props["SAML"] = strconv.FormatBool(*l.Features.SAML)
		props["Cluster"] = strconv.FormatBool(*l.Features.Cluster)
		props["Metrics"] = strconv.FormatBool(*l.Features.Metrics)
		props["GoogleOAuth"] = strconv.FormatBool(*l.Features.GoogleOAuth)
		props["Office365OAuth"] = strconv.FormatBool(*l.Features.Office365OAuth)
		props["OpenId"] = strconv.FormatBool(*l.Features.OpenId)
		props["Compliance"] = strconv.FormatBool(*l.Features.Compliance)
		props["MHPNS"] = strconv.FormatBool(*l.Features.MHPNS)
		props["Announcement"] = strconv.FormatBool(*l.Features.Announcement)
		props["Elasticsearch"] = strconv.FormatBool(*l.Features.Elasticsearch)
		props["DataRetention"] = strconv.FormatBool(*l.Features.DataRetention)
		props["IDLoadedPushNotifications"] = strconv.FormatBool(*l.Features.IDLoadedPushNotifications)
		props["IssuedAt"] = strconv.FormatInt(l.IssuedAt, 10)
		props["StartsAt"] = strconv.FormatInt(l.StartsAt, 10)
		props["ExpiresAt"] = strconv.FormatInt(l.ExpiresAt, 10)
		props["Name"] = l.Customer.Name
		props["Email"] = l.Customer.Email
		props["Company"] = l.Customer.Company
		props["EmailNotificationContents"] = strconv.FormatBool(*l.Features.EmailNotificationContents)
		props["MessageExport"] = strconv.FormatBool(*l.Features.MessageExport)
		props["CustomPermissionsSchemes"] = strconv.FormatBool(*l.Features.CustomPermissionsSchemes)
		props["GuestAccounts"] = strconv.FormatBool(*l.Features.GuestAccounts)
		props["GuestAccountsPermissions"] = strconv.FormatBool(*l.Features.GuestAccountsPermissions)
		props["CustomTermsOfService"] = strconv.FormatBool(*l.Features.CustomTermsOfService)
		props["LockTeammateNameDisplay"] = strconv.FormatBool(*l.Features.LockTeammateNameDisplay)
		props["Cloud"] = strconv.FormatBool(*l.Features.Cloud)
		props["SharedChannels"] = strconv.FormatBool(*l.Features.SharedChannels)
		props["RemoteClusterService"] = strconv.FormatBool(*l.Features.RemoteClusterService)
		props["IsTrial"] = strconv.FormatBool(l.IsTrial)
	}

	return props
}

func GetSanitizedClientLicense(l map[string]string) map[string]string {
	sanitizedLicense := make(map[string]string)

	for k, v := range l {
		sanitizedLicense[k] = v
	}

	delete(sanitizedLicense, "Id")
	delete(sanitizedLicense, "Name")
	delete(sanitizedLicense, "Email")
	delete(sanitizedLicense, "IssuedAt")
	delete(sanitizedLicense, "StartsAt")
	delete(sanitizedLicense, "ExpiresAt")
	delete(sanitizedLicense, "SkuName")
	delete(sanitizedLicense, "SkuShortName")

	return sanitizedLicense
}
