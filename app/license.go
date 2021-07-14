// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/jobs"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const (
	requestTrialURL           = "https://customers.mattermost.com/api/v1/trials"
	LicenseEnv                = "MM_LICENSE"
	LicenseRenewalURL         = "https://portal.test.cloud.mattermost.com/subscribe/renew"
	JWTDefaultTokenExpiration = 7 * 24 * time.Hour // 7 days of expiration
)

// JWTClaims custom JWT claims with the needed information for the
// renewal process
type JWTClaims struct {
	LicenseID   string `json:"license_id"`
	ActiveUsers int64  `json:"active_users"`
	jwt.StandardClaims
}

func (s *Server) LoadLicense() {
	// ENV var overrides all other sources of license.
	licenseStr := os.Getenv(LicenseEnv)
	if licenseStr != "" {
		license, err := utils.LicenseValidator.LicenseFromBytes([]byte(licenseStr))
		if err != nil {
			mlog.Error("Failed to read license set in environment.", mlog.Err(err))
			return
		}

		// skip the restrictions if license is a sanctioned trial
		if !license.IsSanctionedTrial() && license.IsTrialLicense() {
			canStartTrialLicense, err := s.LicenseManager.CanStartTrial()
			if err != nil {
				mlog.Info("Failed to validate trial eligibility.", mlog.Err(err))
				return
			}

			if !canStartTrialLicense {
				mlog.Info("Cannot start trial multiple times.")
				return
			}
		}

		if s.ValidateAndSetLicenseBytes([]byte(licenseStr)) {
			mlog.Info("License key from ENV is valid, unlocking enterprise features.")
		}
		return
	}

	licenseId := ""
	props, nErr := s.Store.System().Get()
	if nErr == nil {
		licenseId = props[model.SYSTEM_ACTIVE_LICENSE_ID]
	}

	if !model.IsValidId(licenseId) {
		// Lets attempt to load the file from disk since it was missing from the DB
		license, licenseBytes := utils.GetAndValidateLicenseFileFromDisk(*s.Config().ServiceSettings.LicenseFileLocation)

		if license != nil {
			if _, err := s.SaveLicense(licenseBytes); err != nil {
				mlog.Info("Failed to save license key loaded from disk.", mlog.Err(err))
			} else {
				licenseId = license.Id
			}
		}
	}

	record, nErr := s.Store.License().Get(licenseId)
	if nErr != nil {
		mlog.Info("License key from https://mattermost.com required to unlock enterprise features.")
		s.SetLicense(nil)
		return
	}

	s.ValidateAndSetLicenseBytes([]byte(record.Bytes))
	mlog.Info("License key valid unlocking enterprise features.")
}

func (s *Server) SaveLicense(licenseBytes []byte) (*model.License, *model.AppError) {
	success, licenseStr := utils.LicenseValidator.ValidateLicense(licenseBytes)
	if !success {
		return nil, model.NewAppError("addLicense", model.INVALID_LICENSE_ERROR, nil, "", http.StatusBadRequest)
	}
	license := model.LicenseFromJson(strings.NewReader(licenseStr))

	uniqueUserCount, err := s.Store.User().Count(model.UserCountOptions{})
	if err != nil {
		return nil, model.NewAppError("addLicense", "api.license.add_license.invalid_count.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	if uniqueUserCount > int64(*license.Features.Users) {
		return nil, model.NewAppError("addLicense", "api.license.add_license.unique_users.app_error", map[string]interface{}{"Users": *license.Features.Users, "Count": uniqueUserCount}, "", http.StatusBadRequest)
	}

	if license != nil && license.IsExpired() {
		return nil, model.NewAppError("addLicense", model.EXPIRED_LICENSE_ERROR, nil, "", http.StatusBadRequest)
	}

	if ok := s.SetLicense(license); !ok {
		return nil, model.NewAppError("addLicense", model.EXPIRED_LICENSE_ERROR, nil, "", http.StatusBadRequest)
	}

	record := &model.LicenseRecord{}
	record.Id = license.Id
	record.Bytes = string(licenseBytes)

	_, nErr := s.Store.License().Save(record)
	if nErr != nil {
		s.RemoveLicense()
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("addLicense", "api.license.add_license.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	sysVar := &model.System{}
	sysVar.Name = model.SYSTEM_ACTIVE_LICENSE_ID
	sysVar.Value = license.Id
	if err := s.Store.System().SaveOrUpdate(sysVar); err != nil {
		s.RemoveLicense()
		return nil, model.NewAppError("addLicense", "api.license.add_license.save_active.app_error", nil, "", http.StatusInternalServerError)
	}

	s.ReloadConfig()
	s.InvalidateAllCaches()

	// restart job server workers - this handles the edge case where a license file is uploaded, but the job server
	// doesn't start until the server is restarted, which prevents the 'run job now' buttons in system console from
	// functioning as expected
	if *s.Config().JobSettings.RunJobs && s.Jobs != nil {
		if err := s.Jobs.StopWorkers(); err != nil && !errors.Is(err, jobs.ErrWorkersNotRunning) {
			mlog.Warn("Stopping job server workers failed", mlog.Err(err))
		}
		if err := s.Jobs.InitWorkers(); err != nil {
			mlog.Error("Initializing job server workers failed", mlog.Err(err))
		} else if err := s.Jobs.StartWorkers(); err != nil {
			mlog.Error("Starting job server workers failed", mlog.Err(err))
		}
	}
	if *s.Config().JobSettings.RunScheduler && s.Jobs != nil {
		if err := s.Jobs.StartSchedulers(); err != nil && !errors.Is(err, jobs.ErrSchedulersRunning) {
			mlog.Error("Starting job server schedulers failed", mlog.Err(err))
		}
	}

	return license, nil
}

func (s *Server) SetLicense(license *model.License) bool {
	oldLicense := s.licenseValue.Load()

	defer func() {
		for _, listener := range s.licenseListeners {
			if oldLicense == nil {
				listener(nil, license)
			} else {
				listener(oldLicense.(*model.License), license)
			}
		}
	}()

	if license != nil {
		license.Features.SetDefaults()

		s.licenseValue.Store(license)
		s.clientLicenseValue.Store(utils.GetClientLicense(license))
		return true
	}

	s.licenseValue.Store((*model.License)(nil))
	s.clientLicenseValue.Store(map[string]string(nil))
	return false
}

func (s *Server) ValidateAndSetLicenseBytes(b []byte) bool {
	if success, licenseStr := utils.LicenseValidator.ValidateLicense(b); success {
		license := model.LicenseFromJson(strings.NewReader(licenseStr))
		s.SetLicense(license)
		return true
	}

	mlog.Warn("No valid enterprise license found")
	return false
}

func (s *Server) SetClientLicense(m map[string]string) {
	s.clientLicenseValue.Store(m)
}

func (s *Server) ClientLicense() map[string]string {
	if clientLicense, _ := s.clientLicenseValue.Load().(map[string]string); clientLicense != nil {
		return clientLicense
	}
	return map[string]string{"IsLicensed": "false"}
}

func (s *Server) RemoveLicense() *model.AppError {
	if license, _ := s.licenseValue.Load().(*model.License); license == nil {
		return nil
	}

	mlog.Info("Remove license.", mlog.String("id", model.SYSTEM_ACTIVE_LICENSE_ID))

	sysVar := &model.System{}
	sysVar.Name = model.SYSTEM_ACTIVE_LICENSE_ID
	sysVar.Value = ""

	if err := s.Store.System().SaveOrUpdate(sysVar); err != nil {
		return model.NewAppError("RemoveLicense", "app.system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	s.SetLicense(nil)
	s.ReloadConfig()
	s.InvalidateAllCaches()

	return nil
}

func (s *Server) AddLicenseListener(listener func(oldLicense, newLicense *model.License)) string {
	id := model.NewId()
	s.licenseListeners[id] = listener
	return id
}

func (s *Server) RemoveLicenseListener(id string) {
	delete(s.licenseListeners, id)
}

func (s *Server) GetSanitizedClientLicense() map[string]string {
	return utils.GetSanitizedClientLicense(s.ClientLicense())
}

// RequestTrialLicense request a trial license from the mattermost official license server
func (s *Server) RequestTrialLicense(trialRequest *model.TrialLicenseRequest) *model.AppError {
	resp, err := http.Post(requestTrialURL, "application/json", bytes.NewBuffer([]byte(trialRequest.ToJson())))
	if err != nil {
		return model.NewAppError("RequestTrialLicense", "api.license.request_trial_license.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	defer resp.Body.Close()
	licenseResponse := model.MapFromJson(resp.Body)

	if _, ok := licenseResponse["license"]; !ok {
		return model.NewAppError("RequestTrialLicense", "api.license.request_trial_license.app_error", nil, licenseResponse["message"], http.StatusBadRequest)
	}

	if _, err := s.SaveLicense([]byte(licenseResponse["license"])); err != nil {
		return err
	}

	s.ReloadConfig()
	s.InvalidateAllCaches()

	return nil
}

// GenerateRenewalToken returns the current active token or generate a new one if
// the current active one has expired
func (s *Server) GenerateRenewalToken(expiration time.Duration) (string, *model.AppError) {
	license := s.License()
	if license == nil {
		// Clean renewal token if there is no license present
		if _, err := s.Store.System().PermanentDeleteByName(model.SYSTEM_LICENSE_RENEWAL_TOKEN); err != nil {
			mlog.Warn("error removing the renewal token", mlog.Err(err))
		}
		return "", model.NewAppError("GenerateRenewalToken", "app.license.generate_renewal_token.no_license", nil, "", http.StatusBadRequest)
	}

	if *license.Features.Cloud {
		return "", model.NewAppError("GenerateRenewalToken", "app.license.generate_renewal_token.bad_license", nil, "", http.StatusBadRequest)
	}

	currentToken, _ := s.Store.System().GetByName(model.SYSTEM_LICENSE_RENEWAL_TOKEN)
	if currentToken != nil {
		tokenIsValid, err := s.renewalTokenValid(currentToken.Value, license.Customer.Email)
		if err != nil {
			mlog.Warn("error checking license renewal token validation", mlog.Err(err))
		}
		if currentToken.Value != "" && tokenIsValid {
			return currentToken.Value, nil
		}
	}

	activeUsers, err := s.Store.User().Count(model.UserCountOptions{})
	if err != nil {
		return "", model.NewAppError("GenerateRenewalToken", "app.license.generate_renewal_token.app_error",
			nil, err.Error(), http.StatusInternalServerError)
	}

	expirationTime := time.Now().UTC().Add(expiration)
	claims := &JWTClaims{
		LicenseID:   license.Id,
		ActiveUsers: activeUsers,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(license.Customer.Email))
	if err != nil {
		return "", model.NewAppError("GenerateRenewalToken", "app.license.generate_renewal_token.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	err = s.Store.System().SaveOrUpdate(&model.System{
		Name:  model.SYSTEM_LICENSE_RENEWAL_TOKEN,
		Value: tokenString,
	})
	if err != nil {
		return "", model.NewAppError("GenerateRenewalToken", "app.license.generate_renewal_token.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return tokenString, nil
}

func (s *Server) renewalTokenValid(tokenString, signingKey string) (bool, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(signingKey), nil
	})
	if err != nil && !token.Valid {
		return false, errors.Wrapf(err, "Error validating JWT token")
	}
	expirationTime := time.Unix(claims.ExpiresAt, 0)
	if expirationTime.Before(time.Now().UTC()) {
		return false, nil
	}
	return true, nil
}

// GenerateLicenseRenewalLink returns a link that points to the CWS where clients can renew license
func (s *Server) GenerateLicenseRenewalLink() (string, *model.AppError) {
	renewalToken, err := s.GenerateRenewalToken(JWTDefaultTokenExpiration)
	if err != nil {
		return "", err
	}
	renewalLink := LicenseRenewalURL + "?token=" + renewalToken
	return renewalLink, nil
}
