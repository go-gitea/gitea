// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubServer returns an httptest server that impersonates a minimal ES API.
// Each matching request invokes the supplied handler; unmatched requests 404.
// The X-Elastic-Product header is always set so the v8 client accepts replies.
func stubServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		handler(w, r)
	}))
}

func TestBulkReportsItemFailure(t *testing.T) {
	server := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, `{"version":{"number":"8.0.0"}}`)
		case r.Method == http.MethodHead && strings.HasPrefix(r.URL.Path, "/test"):
			w.WriteHeader(http.StatusOK) // index exists, skip create
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "_bulk"):
			io.WriteString(w, `{"errors":true,"items":[{"index":{"status":400,"error":{"type":"illegal_argument_exception","reason":"bad field"}}}]}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	ix := NewIndexer(server.URL, "test", 1, "{}")
	_, err := ix.Init(t.Context())
	require.NoError(t, err)
	defer ix.Close()

	err = ix.Bulk(t.Context(), []BulkOp{IndexOp("1", map[string]string{"x": "y"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk item failed")
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "illegal_argument_exception")
}

func TestPing(t *testing.T) {
	var status string
	server := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, `{"version":{"number":"8.0.0"}}`)
		case r.Method == http.MethodHead && strings.HasPrefix(r.URL.Path, "/test"):
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/_cluster/health"):
			fmt.Fprintf(w, `{"status":%q}`, status)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	ix := NewIndexer(server.URL, "test", 1, "{}")
	_, err := ix.Init(t.Context())
	require.NoError(t, err)
	defer ix.Close()

	for _, tc := range []struct {
		clusterStatus string
		wantErr       bool
	}{
		{"green", false},
		{"yellow", false},
		{"red", true},
	} {
		status = tc.clusterStatus
		err := ix.Ping(t.Context())
		if tc.wantErr {
			assert.Error(t, err, "status %q", tc.clusterStatus)
		} else {
			assert.NoError(t, err, "status %q", tc.clusterStatus)
		}
	}
}
