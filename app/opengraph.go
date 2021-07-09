// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"html"
	"io"
	"net/url"

	"github.com/dyatlov/go-opengraph/opengraph"
	"golang.org/x/net/html/charset"

	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const MaxOpenGraphResponseSize = 1024 * 1024 * 50

func (a *App) GetOpenGraphMetadata(requestURL string) *opengraph.OpenGraph {
	res, err := a.HTTPService().MakeClient(false).Get(requestURL)
	if err != nil {
		mlog.Debug("GetOpenGraphMetadata request failed", mlog.String("requestURL", requestURL), mlog.Err(err))
		return nil
	}
	defer res.Body.Close()
	return a.parseOpenGraphMetadata(requestURL, res.Body, res.Header.Get("Content-Type"))
}

func (a *App) parseOpenGraphMetadata(requestURL string, body io.Reader, contentType string) *opengraph.OpenGraph {
	og := opengraph.NewOpenGraph()
	body = forceHTMLEncodingToUTF8(io.LimitReader(body, MaxOpenGraphResponseSize), contentType)

	if err := og.ProcessHTML(body); err != nil {
		mlog.Warn("parseOpenGraphMetadata processing failed", mlog.String("requestURL", requestURL), mlog.Err(err))
	}

	makeOpenGraphURLsAbsolute(og, requestURL)

	openGraphDecodeHTMLEntities(og)

	// If image proxy enabled modify open graph data to feed though proxy
	if toProxyURL := a.ImageProxyAdder(); toProxyURL != nil {
		og = openGraphDataWithProxyAddedToImageURLs(og, toProxyURL)
	}

	// The URL should be the link the user provided in their message, not a redirected one.
	if og.URL != "" {
		og.URL = requestURL
	}

	return og
}

func forceHTMLEncodingToUTF8(body io.Reader, contentType string) io.Reader {
	r, err := charset.NewReader(body, contentType)
	if err != nil {
		mlog.Warn("forceHTMLEncodingToUTF8 failed to convert", mlog.String("contentType", contentType), mlog.Err(err))
		return body
	}
	return r
}

func makeOpenGraphURLsAbsolute(og *opengraph.OpenGraph, requestURL string) {
	parsedRequestURL, err := url.Parse(requestURL)
	if err != nil {
		mlog.Warn("makeOpenGraphURLsAbsolute failed to parse url", mlog.String("requestURL", requestURL), mlog.Err(err))
		return
	}

	makeURLAbsolute := func(resultURL string) string {
		if resultURL == "" {
			return resultURL
		}

		parsedResultURL, err := url.Parse(resultURL)
		if err != nil {
			mlog.Warn("makeOpenGraphURLsAbsolute failed to parse result", mlog.String("requestURL", requestURL), mlog.Err(err))
			return resultURL
		}

		if parsedResultURL.IsAbs() {
			return resultURL
		}

		return parsedRequestURL.ResolveReference(parsedResultURL).String()
	}

	og.URL = makeURLAbsolute(og.URL)

	for _, image := range og.Images {
		image.URL = makeURLAbsolute(image.URL)
		image.SecureURL = makeURLAbsolute(image.SecureURL)
	}

	for _, audio := range og.Audios {
		audio.URL = makeURLAbsolute(audio.URL)
		audio.SecureURL = makeURLAbsolute(audio.SecureURL)
	}

	for _, video := range og.Videos {
		video.URL = makeURLAbsolute(video.URL)
		video.SecureURL = makeURLAbsolute(video.SecureURL)
	}
}

func openGraphDataWithProxyAddedToImageURLs(ogdata *opengraph.OpenGraph, toProxyURL func(string) string) *opengraph.OpenGraph {
	for _, image := range ogdata.Images {
		var url string
		if image.SecureURL != "" {
			url = image.SecureURL
		} else {
			url = image.URL
		}

		image.URL = ""
		image.SecureURL = toProxyURL(url)
	}

	return ogdata
}

func openGraphDecodeHTMLEntities(og *opengraph.OpenGraph) {
	og.Title = html.UnescapeString(og.Title)
	og.Description = html.UnescapeString(og.Description)
}
