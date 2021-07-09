// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	SamlPublicCertificateName = "saml-public.crt"
	SamlPrivateKeyName        = "saml-private.key"
	SamlIDpCertificateName    = "saml-idp.crt"
)

func (a *App) GetSamlMetadata() (string, *model.AppError) {
	if a.Saml() == nil {
		err := model.NewAppError("GetSamlMetadata", "api.admin.saml.not_available.app_error", nil, "", http.StatusNotImplemented)
		return "", err
	}

	result, err := a.Saml().GetMetadata()
	if err != nil {
		return "", model.NewAppError("GetSamlMetadata", "api.admin.saml.metadata.app_error", nil, "err="+err.Message, err.StatusCode)
	}
	return result, nil
}

func (a *App) writeSamlFile(filename string, fileData *multipart.FileHeader) *model.AppError {
	file, err := fileData.Open()
	if err != nil {
		return model.NewAppError("AddSamlCertificate", "api.admin.add_certificate.open.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return model.NewAppError("AddSamlCertificate", "api.admin.add_certificate.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	err = a.Srv().configStore.SetFile(filename, data)
	if err != nil {
		return model.NewAppError("AddSamlCertificate", "api.admin.add_certificate.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) AddSamlPublicCertificate(fileData *multipart.FileHeader) *model.AppError {
	if err := a.writeSamlFile(SamlPublicCertificateName, fileData); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.PublicCertificateFile = SamlPublicCertificateName

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) AddSamlPrivateCertificate(fileData *multipart.FileHeader) *model.AppError {
	if err := a.writeSamlFile(SamlPrivateKeyName, fileData); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.PrivateKeyFile = SamlPrivateKeyName

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) AddSamlIDpCertificate(fileData *multipart.FileHeader) *model.AppError {
	if err := a.writeSamlFile(SamlIDpCertificateName, fileData); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.IDpCertificateFile = SamlIDpCertificateName

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) removeSamlFile(filename string) *model.AppError {
	if err := a.Srv().configStore.RemoveFile(filename); err != nil {
		return model.NewAppError("RemoveSamlFile", "api.admin.remove_certificate.delete.app_error", map[string]interface{}{"Filename": filename}, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) RemoveSamlPublicCertificate() *model.AppError {
	if err := a.removeSamlFile(*a.Config().SamlSettings.PublicCertificateFile); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.PublicCertificateFile = ""
	*cfg.SamlSettings.Encrypt = false

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) RemoveSamlPrivateCertificate() *model.AppError {
	if err := a.removeSamlFile(*a.Config().SamlSettings.PrivateKeyFile); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.PrivateKeyFile = ""
	*cfg.SamlSettings.Encrypt = false

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) RemoveSamlIDpCertificate() *model.AppError {
	if err := a.removeSamlFile(*a.Config().SamlSettings.IDpCertificateFile); err != nil {
		return err
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.IDpCertificateFile = ""
	*cfg.SamlSettings.Enable = false

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) GetSamlCertificateStatus() *model.SamlCertificateStatus {
	status := &model.SamlCertificateStatus{}

	status.IDpCertificateFile, _ = a.Srv().configStore.HasFile(*a.Config().SamlSettings.IDpCertificateFile)
	status.PrivateKeyFile, _ = a.Srv().configStore.HasFile(*a.Config().SamlSettings.PrivateKeyFile)
	status.PublicCertificateFile, _ = a.Srv().configStore.HasFile(*a.Config().SamlSettings.PublicCertificateFile)

	return status
}

func (a *App) GetSamlMetadataFromIDp(idpMetadataURL string) (*model.SamlMetadataResponse, *model.AppError) {
	if a.Saml() == nil {
		err := model.NewAppError("GetSamlMetadataFromIdp", "api.admin.saml.not_available.app_error", nil, "", http.StatusNotImplemented)
		return nil, err
	}

	if !strings.HasPrefix(idpMetadataURL, "http://") && !strings.HasPrefix(idpMetadataURL, "https://") {
		idpMetadataURL = "https://" + idpMetadataURL
	}

	idpMetadataRaw, err := a.FetchSamlMetadataFromIDp(idpMetadataURL)
	if err != nil {
		return nil, err
	}

	data, err := a.BuildSamlMetadataObject(idpMetadataRaw)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (a *App) FetchSamlMetadataFromIDp(url string) ([]byte, *model.AppError) {
	resp, err := a.HTTPService().MakeClient(false).Get(url)
	if err != nil {
		return nil, model.NewAppError("FetchSamlMetadataFromIdp", "app.admin.saml.invalid_response_from_idp.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, model.NewAppError("FetchSamlMetadataFromIdp", "app.admin.saml.invalid_response_from_idp.app_error", nil, fmt.Sprintf("status_code=%d", resp.StatusCode), http.StatusBadRequest)
	}
	defer resp.Body.Close()

	bodyXML, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, model.NewAppError("FetchSamlMetadataFromIdp", "app.admin.saml.failure_read_response_body_from_idp.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return bodyXML, nil
}

func (a *App) BuildSamlMetadataObject(idpMetadata []byte) (*model.SamlMetadataResponse, *model.AppError) {
	entityDescriptor := model.EntityDescriptor{}
	err := xml.Unmarshal(idpMetadata, &entityDescriptor)
	if err != nil {
		return nil, model.NewAppError("BuildSamlMetadataObject", "app.admin.saml.failure_decode_metadata_xml_from_idp.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	data := &model.SamlMetadataResponse{}
	data.IDpDescriptorURL = entityDescriptor.EntityID

	if entityDescriptor.IDPSSODescriptors == nil || len(entityDescriptor.IDPSSODescriptors) == 0 {
		err := model.NewAppError("BuildSamlMetadataObject", "api.admin.saml.invalid_xml_missing_idpssodescriptors.app_error", nil, "", http.StatusInternalServerError)
		return nil, err
	}

	idpSSODescriptor := entityDescriptor.IDPSSODescriptors[0]
	if idpSSODescriptor.SingleSignOnServices == nil || len(idpSSODescriptor.SingleSignOnServices) == 0 {
		err := model.NewAppError("BuildSamlMetadataObject", "api.admin.saml.invalid_xml_missing_ssoservices.app_error", nil, "", http.StatusInternalServerError)
		return nil, err
	}

	data.IDpURL = idpSSODescriptor.SingleSignOnServices[0].Location
	if idpSSODescriptor.SSODescriptor.RoleDescriptor.KeyDescriptors == nil || len(idpSSODescriptor.SSODescriptor.RoleDescriptor.KeyDescriptors) == 0 {
		err := model.NewAppError("BuildSamlMetadataObject", "api.admin.saml.invalid_xml_missing_keydescriptor.app_error", nil, "", http.StatusInternalServerError)
		return nil, err
	}
	keyDescriptor := idpSSODescriptor.SSODescriptor.RoleDescriptor.KeyDescriptors[0]
	data.IDpPublicCertificate = keyDescriptor.KeyInfo.X509Data.X509Certificate.Cert

	return data, nil
}

func (a *App) SetSamlIDpCertificateFromMetadata(data []byte) *model.AppError {
	const certPrefix = "-----BEGIN CERTIFICATE-----\n"
	const certSuffix = "\n-----END CERTIFICATE-----"
	fixedCertTxt := certPrefix + string(data) + certSuffix

	block, _ := pem.Decode([]byte(fixedCertTxt))
	if _, e := x509.ParseCertificate(block.Bytes); e != nil {
		return model.NewAppError("SetSamlIdpCertificateFromMetadata", "api.admin.saml.failure_parse_idp_certificate.app_error", nil, e.Error(), http.StatusInternalServerError)
	}

	data = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: block.Bytes,
	})

	if err := a.Srv().configStore.SetFile(SamlIDpCertificateName, data); err != nil {
		return model.NewAppError("SetSamlIdpCertificateFromMetadata", "api.admin.saml.failure_save_idp_certificate_file.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	cfg := a.Config().Clone()
	*cfg.SamlSettings.IDpCertificateFile = SamlIDpCertificateName

	if err := cfg.IsValid(); err != nil {
		return err
	}

	a.UpdateConfig(func(dest *model.Config) { *dest = *cfg })

	return nil
}

func (a *App) ResetSamlAuthDataToEmail(includeDeleted bool, dryRun bool, userIDs []string) (numAffected int, appErr *model.AppError) {
	if a.Saml() == nil {
		appErr = model.NewAppError("ResetAuthDataToEmail", "api.admin.saml.not_available.app_error", nil, "", http.StatusNotImplemented)
		return
	}
	numAffected, err := a.srv.Store.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, userIDs, includeDeleted, dryRun)
	if err != nil {
		appErr = model.NewAppError("ResetAuthDataToEmail", "api.admin.saml.failure_reset_authdata_to_email.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}
