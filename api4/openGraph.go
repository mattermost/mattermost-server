// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/cache"
)

const OpenGraphMetadataCacheSize = 10000

var openGraphDataCache = cache.NewLRU(cache.LRUOptions{
	Size: OpenGraphMetadataCacheSize,
})

func (api *API) InitOpenGraph() {
	api.BaseRoutes.OpenGraph.Handle("", api.ApiSessionRequired(getOpenGraphMetadata)).Methods("POST")

	// Dump the image cache if the proxy settings have changed. (need switch URLs to the correct proxy)
	api.app.AddConfigListener(func(before, after *model.Config) {
		if (before.ImageProxySettings.Enable != after.ImageProxySettings.Enable) ||
			(before.ImageProxySettings.ImageProxyType != after.ImageProxySettings.ImageProxyType) ||
			(before.ImageProxySettings.RemoteImageProxyURL != after.ImageProxySettings.RemoteImageProxyURL) ||
			(before.ImageProxySettings.RemoteImageProxyOptions != after.ImageProxySettings.RemoteImageProxyOptions) {
			openGraphDataCache.Purge()
		}
	})
}

func getOpenGraphMetadata(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().ServiceSettings.EnableLinkPreviews {
		c.Err = model.NewAppError("getOpenGraphMetadata", "api.post.link_preview_disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	props := model.StringInterfaceFromJSON(r.Body)

	url := ""
	ok := false
	if url, ok = props["url"].(string); url == "" || !ok {
		c.SetInvalidParam("url")
		return
	}

	var ogJSONGeneric []byte
	err := openGraphDataCache.Get(url, &ogJSONGeneric)
	if err == nil {
		w.Write(ogJSONGeneric)
		return
	}

	og := c.App.GetOpenGraphMetadata(url)
	ogJSON, err := og.ToJSON()
	openGraphDataCache.SetWithExpiry(url, ogJSON, 1*time.Hour)
	if err != nil {
		w.Write([]byte(`{"url": ""}`))
		return
	}

	w.Write(ogJSON)
}
