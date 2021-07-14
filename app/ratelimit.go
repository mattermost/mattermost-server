// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type RateLimiter struct {
	throttledRateLimiter *throttled.GCRARateLimiter
	useAuth              bool
	useIP                bool
	header               string
	trustedProxyIPHeader []string
}

func NewRateLimiter(settings *model.RateLimitSettings, trustedProxyIPHeader []string) (*RateLimiter, error) {
	store, err := memstore.New(*settings.MemoryStoreSize)
	if err != nil {
		return nil, errors.Wrap(err, i18n.T("api.server.start_server.rate_limiting_memory_store"))
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(*settings.PerSec),
		MaxBurst: *settings.MaxBurst,
	}

	throttledRateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		return nil, errors.Wrap(err, i18n.T("api.server.start_server.rate_limiting_rate_limiter"))
	}

	return &RateLimiter{
		throttledRateLimiter: throttledRateLimiter,
		useAuth:              *settings.VaryByUser,
		useIP:                *settings.VaryByRemoteAddr,
		header:               settings.VaryByHeader,
		trustedProxyIPHeader: trustedProxyIPHeader,
	}, nil
}

func (rl *RateLimiter) GenerateKey(r *http.Request) string {
	key := ""

	if rl.useAuth {
		token, tokenLocation := ParseAuthTokenFromRequest(r)
		if tokenLocation != TokenLocationNotFound {
			key += token
		} else if rl.useIP { // If we don't find an authentication token and IP based is enabled, fall back to IP
			key += utils.GetIPAddress(r, rl.trustedProxyIPHeader)
		}
	} else if rl.useIP { // Only if Auth based is not enabed do we use a plain IP based
		key += utils.GetIPAddress(r, rl.trustedProxyIPHeader)
	}

	// Note that most of the time the user won't have to set this because the utils.GetIpAddress above tries the
	// most common headers anyway.
	if rl.header != "" {
		key += strings.ToLower(r.Header.Get(rl.header))
	}

	return key
}

func (rl *RateLimiter) RateLimitWriter(key string, w http.ResponseWriter) bool {
	limited, context, err := rl.throttledRateLimiter.RateLimit(key, 1)
	if err != nil {
		mlog.Error("Internal server error when rate limiting. Rate Limiting broken.", mlog.Err(err))
		return false
	}

	setRateLimitHeaders(w, context)

	if limited {
		mlog.Debug("Denied due to throttling settings code=429", mlog.String("key", key))
		http.Error(w, "limit exceeded", http.StatusTooManyRequests)
	}

	return limited
}

func (rl *RateLimiter) UserIdRateLimit(userID string, w http.ResponseWriter) bool {
	if rl.useAuth {
		return rl.RateLimitWriter(userID, w)
	}
	return false
}

func (rl *RateLimiter) RateLimitHandler(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.GenerateKey(r)

		if !rl.RateLimitWriter(key, w) {
			wrappedHandler.ServeHTTP(w, r)
		}
	})
}

// Copied from https://github.com/throttled/throttled http.go
func setRateLimitHeaders(w http.ResponseWriter, context throttled.RateLimitResult) {
	if v := context.Limit; v >= 0 {
		w.Header().Add("X-RateLimit-Limit", strconv.Itoa(v))
	}

	if v := context.Remaining; v >= 0 {
		w.Header().Add("X-RateLimit-Remaining", strconv.Itoa(v))
	}

	if v := context.ResetAfter; v >= 0 {
		vi := int(math.Ceil(v.Seconds()))
		w.Header().Add("X-RateLimit-Reset", strconv.Itoa(vi))
	}

	if v := context.RetryAfter; v >= 0 {
		vi := int(math.Ceil(v.Seconds()))
		w.Header().Add("Retry-After", strconv.Itoa(vi))
	}
}
