// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestDownloadFromURL(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	app := th.App
	app.Config().PluginSettings.AllowInsecureDownloadURL = model.NewBool(true)

	// To keep track of how many times an endpoint is retried. This needs to be reset
	// for each test run.
	retries := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/succeeds-after-retry", func(w http.ResponseWriter, r *http.Request) {
		if retries < 2 {
			http.Error(w, "Request Timed out", http.StatusGatewayTimeout)
			retries++
			return
		}

		_, _ = w.Write([]byte("Your request is successful."))
	})

	mux.HandleFunc("/fails-forever", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "This would fail forever", http.StatusInternalServerError)
	})

	testServer := httptest.NewServer(mux)

	tests := []struct {
		name        string
		downloadURL string
		wantErr     bool
	}{
		{
			name:        "Should succeed after two retries",
			downloadURL: fmt.Sprintf("%s/succeeds-after-retry", testServer.URL),
			wantErr:     false,
		},
		{
			name:        "Should not retry forever",
			downloadURL: fmt.Sprintf("%s/fails-forever", testServer.URL),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retries = 0 // reset the retires
			_, err := th.App.DownloadFromURL(tt.downloadURL)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
