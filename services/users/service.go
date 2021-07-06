// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package users

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/cache"
	"github.com/mattermost/mattermost-server/v5/store"
)

type UserService struct {
	store        store.UserStore
	sessionStore store.SessionStore
	oAuthStore   store.OAuthStore
	sessionCache cache.Cache
	sessionPool  sync.Pool
	metrics      einterfaces.MetricsInterface
	cluster      einterfaces.ClusterInterface
	config       func() *model.Config
	license      func() *model.License
}

// ServiceConfig is used to initialize the UserService.
type ServiceConfig struct {
	// Mandatory fields
	UserStore    store.UserStore
	SessionStore store.SessionStore
	OAuthStore   store.OAuthStore
	ConfigFn     func() *model.Config
	LicenseFn    func() *model.License
	// Optional fields
	Metrics einterfaces.MetricsInterface
	Cluster einterfaces.ClusterInterface
}

func New(c ServiceConfig) (*UserService, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	cacheProvider := cache.NewProvider()
	if err := cacheProvider.Connect(); err != nil {
		return nil, fmt.Errorf("could not connect to cache provider: %w", err)
	}

	sessionCache, err := cacheProvider.NewCache(&cache.CacheOptions{
		Size:           model.SESSION_CACHE_SIZE,
		Striped:        true,
		StripedBuckets: maxInt(runtime.NumCPU()-1, 1),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create session cache: %w", err)
	}

	return &UserService{
		store:        c.UserStore,
		sessionStore: c.SessionStore,
		oAuthStore:   c.OAuthStore,
		config:       c.ConfigFn,
		license:      c.LicenseFn,
		metrics:      c.Metrics,
		cluster:      c.Cluster,
		sessionCache: sessionCache,
		sessionPool: sync.Pool{
			New: func() interface{} {
				return &model.Session{}
			},
		},
	}, nil
}

func (c *ServiceConfig) validate() error {
	if c.ConfigFn == nil || c.UserStore == nil || c.SessionStore == nil || c.OAuthStore == nil || c.LicenseFn == nil {
		return errors.New("required parameters are not provided")
	}

	return nil
}
