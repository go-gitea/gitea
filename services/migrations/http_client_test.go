// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationHTTPTransportRetryAfter(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch requests.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.Header().Set("Retry-After", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusForbidden)
		default:
			_, _ = io.Copy(w, r.Body)
		}
	}))
	defer server.Close()

	resp, err := getMigrationHTTPClient().Post(server.URL, "text/plain", strings.NewReader("payload"))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "payload", string(body))
	assert.EqualValues(t, 3, requests.Load())
}

func TestAuthTransportSendsCredentialsToSameHostOnly(t *testing.T) {
	for _, testCase := range []struct{ baseURL, target, expected string }{
		{"http://source.example/sub", "https://source.example/api", "token secret"},
		{"https://source.example", "https://source.example/api", "token secret"},
		{"https://source.example", "http://source.example/api", ""},
		{"https://source.example", "https://other.example/api", ""},
		{"https://source.example:3000", "https://source.example:8443/api", ""},
	} {
		req, err := http.NewRequest(http.MethodGet, testCase.target, nil)
		require.NoError(t, err)
		resp, err := authTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, testCase.expected, req.Header.Get("Authorization"), testCase.target)
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}), testCase.baseURL, "token secret").RoundTrip(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Empty(t, req.Header.Get("Authorization"))
	}
}
