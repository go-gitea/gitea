// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsOpenSearch(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		status   int
		expected bool
	}{
		{"opensearch", `{"version":{"distribution":"opensearch","number":"2.18.0"}}`, http.StatusOK, true},
		{"elasticsearch", `{"version":{"number":"8.17.0"}}`, http.StatusOK, false},
		{"malformed body", `not json`, http.StatusOK, false},
		{"empty body", ``, http.StatusOK, false},
		{"http error", ``, http.StatusInternalServerError, false}, // server still replies, but no distribution field
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			assert.Equal(t, tc.expected, isOpenSearch(server.URL))
		})
	}
}

func TestIsOpenSearchUnreachable(t *testing.T) {
	// Port 1 is reserved and refuses connections; we expect false, not a panic.
	assert.False(t, isOpenSearch("http://127.0.0.1:1"))
}

func TestIsOpenSearchWithBasicAuth(t *testing.T) {
	// Serve the OpenSearch root payload only when a specific Basic Auth is present.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"version":{"distribution":"opensearch","number":"2.18.0"}}`)
	}))
	defer server.Close()
	// Re-inject the creds into the URL as the configured conn string would.
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("admin", "secret")
	assert.True(t, isOpenSearch(u.String()), "creds in URL should reach the server")

	// Sanity: without creds the probe gets 401 and reports false.
	assert.False(t, isOpenSearch(server.URL))
}
