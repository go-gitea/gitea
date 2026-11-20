// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatestVersion(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"latest":{"version":"1.2.3"}}`))
		}))
		defer srv.Close()

		version, err := fetchLatestVersion(srv.URL)
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", version)
	})

	t.Run("ResponseTooLarge", func(t *testing.T) {
		// valid JSON, but padded with whitespace past the limit
		body := `{"latest":{"version":"1.2.3"}}` + strings.Repeat(" ", maxResponseSize)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL)
		assert.ErrorContains(t, err, "exceeds")
	})
}
