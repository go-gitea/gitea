// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsMiddlewere(t *testing.T) {
	middleware := RouteMetrics()
	r := chi.NewRouter()
	r.Use(middleware)
	r.Get("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
		time.Sleep(5 * time.Millisecond)
	}))
	testServer := httptest.NewServer(r)
	// Check all defined metrics
	verify := func(i int) {
		assert.Equal(t, testutil.CollectAndCount(reqDurationHistogram, "http_server_request_duration_seconds"), i)
		assert.Equal(t, testutil.CollectAndCount(reqSizeHistogram, "http_server_request_body_size"), i)
		assert.Equal(t, testutil.CollectAndCount(respSizeHistogram, "http_server_response_body_size"), i)
		assert.Equal(t, testutil.CollectAndCount(reqInflightGauge, "http_server_active_requests"), i)
	}

	// Check they don't exist before making a request
	verify(0)
	_, err := http.Get(testServer.URL)
	require.NoError(t, err)
	// Check they do exist after making the request
	verify(1)
}
