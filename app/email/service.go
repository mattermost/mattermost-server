// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"net/url"
	"path"

	"github.com/pkg/errors"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/templates"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	emailRateLimitingMemstoreSize = 65536
	emailRateLimitingPerHour      = 20
	emailRateLimitingMaxBurst     = 20

	TokenTypePasswordRecovery = "password_recovery"
	TokenTypeVerifyEmail      = "verify_email"
	TokenTypeTeamInvitation   = "team_invitation"
	TokenTypeGuestInvitation  = "guest_invitation"
	TokenTypeCWSAccess        = "cws_access_token"
)

func condenseSiteURL(siteURL string) string {
	parsedSiteURL, _ := url.Parse(siteURL)
	if parsedSiteURL.Path == "" || parsedSiteURL.Path == "/" {
		return parsedSiteURL.Host
	}

	return path.Join(parsedSiteURL.Host, parsedSiteURL.Path)
}

type Service struct {
	config  func() *model.Config
	goFn    func(f func())
	license func() *model.License

	userService *users.UserService
	store       store.Store

	templatesContainer      *templates.Container
	PerHourEmailRateLimiter *throttled.GCRARateLimiter
	PerDayEmailRateLimiter  *throttled.GCRARateLimiter
	EmailBatching           *EmailBatchingJob
}

type ServiceConfig struct {
	ConfigFn  func() *model.Config
	LicenseFn func() *model.License
	GoFn      func(f func())

	TemplatesContainer *templates.Container
	UserService        *users.UserService
	Store              store.Store
}

func NewService(config ServiceConfig) (*Service, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	service := &Service{
		config:             config.ConfigFn,
		templatesContainer: config.TemplatesContainer,
		license:            config.LicenseFn,
		goFn:               config.GoFn,
		store:              config.Store,
		userService:        config.UserService,
	}
	if err := service.setUpRateLimiters(); err != nil {
		return nil, err
	}
	service.InitEmailBatching()
	return service, nil
}

func (c *ServiceConfig) validate() error {
	if c.ConfigFn == nil || c.GoFn == nil || c.Store == nil || c.LicenseFn == nil || c.TemplatesContainer == nil {
		return errors.New("invalid service config")
	}
	return nil
}

func (es *Service) setUpRateLimiters() error {
	store, err := memstore.New(emailRateLimitingMemstoreSize)
	if err != nil {
		return errors.Wrap(err, "Unable to setup email rate limiting memstore.")
	}

	perHourQuota := throttled.RateQuota{
		MaxRate:  throttled.PerHour(emailRateLimitingPerHour),
		MaxBurst: emailRateLimitingMaxBurst,
	}

	perDayQuota := throttled.RateQuota{
		MaxRate:  throttled.PerDay(1),
		MaxBurst: 0,
	}

	perHourRateLimiter, err := throttled.NewGCRARateLimiter(store, perHourQuota)
	if err != nil || perHourRateLimiter == nil {
		return errors.Wrap(err, "Unable to setup email rate limiting GCRA rate limiter.")
	}

	perDayRateLimiter, err := throttled.NewGCRARateLimiter(store, perDayQuota)
	if err != nil || perDayRateLimiter == nil {
		return errors.Wrap(err, "Unable to setup per day email rate limiting GCRA rate limiter.")
	}

	es.PerHourEmailRateLimiter = perHourRateLimiter
	es.PerDayEmailRateLimiter = perDayRateLimiter
	return nil
}
