// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalServiceHTTPMemoizes(t *testing.T) {
	requests := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)
	t.Setenv("TEST_EXTERNAL_SERVICE_URL", server.URL)

	for range 2 {
		require.Equal(t, server.URL, ExternalServiceHTTP(t, "TEST_EXTERNAL_SERVICE_URL", ""))
	}
	require.EqualValues(t, 1, requests.Load())
}
