// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const (
	// HTTPRequestTimeout defines a high timeout for downloading large files
	// from an external URL to avoid slow connections from failing to install.
	HTTPRequestTimeout = 1 * time.Hour
)

func (a *App) DownloadFromURL(downloadURL string) ([]byte, error) {
	return a.Srv().downloadFromURL(downloadURL)
}

func (s *Server) downloadFromURL(downloadURL string) ([]byte, error) {
	if !model.IsValidHttpURL(downloadURL) {
		return nil, errors.Errorf("invalid url %s", downloadURL)
	}

	u, err := url.ParseRequestURI(downloadURL)
	if err != nil {
		return nil, errors.Errorf("failed to parse url %s", downloadURL)
	}
	if !*s.Config().PluginSettings.AllowInsecureDownloadURL && u.Scheme != "https" {
		return nil, errors.Errorf("insecure url not allowed %s", downloadURL)
	}

	client := s.HTTPService.MakeClient(true)
	client.Timeout = HTTPRequestTimeout

	var resp *http.Response
	err = utils.ProgressiveRetry(func() error {
		resp, err = client.Get(downloadURL)

		if err != nil {
			return errors.Wrapf(err, "failed to fetch from %s", downloadURL)
		}

		if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
			_, _ = io.Copy(ioutil.Discard, resp.Body)
			_ = resp.Body.Close()
			return errors.Errorf("failed to fetch from %s", downloadURL)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "download failed after multiple retries.")
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
