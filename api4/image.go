// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"net/url"

	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitImage() {
	api.BaseRoutes.Image.Handle("", api.APISessionRequiredTrustRequester(getImage)).Methods("GET")
}

func getImage(c *Context, w http.ResponseWriter, r *http.Request) {
	actualURL := r.URL.Query().Get("url")
	parsedURL, err := url.Parse(actualURL)
	if err != nil {
		c.Err = model.NewAppError("getImage", "api.image.get.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	} else if parsedURL.Opaque != "" {
		c.Err = model.NewAppError("getImage", "api.image.get.app_error", nil, "", http.StatusBadRequest)
		return
	}
	siteURL, err := url.Parse(*c.App.Config().ServiceSettings.SiteURL)
	if err != nil {
		c.Err = model.NewAppError("getImage", "model.config.is_valid.site_url.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = siteURL.Scheme
	}
	if parsedURL.Host == "" {
		parsedURL.Host = siteURL.Host
	}

	// in case image proxy is enabled and we are fetching a remote image (NOT static or served by plugins), pass request to proxy
	if *c.App.Config().ImageProxySettings.Enable && parsedURL.Host != siteURL.Host {
		c.App.ImageProxy().GetImage(w, r, parsedURL.String())
	} else {
		http.Redirect(w, r, parsedURL.String(), http.StatusFound)
	}
}
