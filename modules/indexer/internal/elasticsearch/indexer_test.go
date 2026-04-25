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

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStubIndexer spins up an httptest server whose root HEAD says "index exists"
// (so Init doesn't try to create it), and returns an initialized Indexer plus
// the server. The handler dispatches everything except that HEAD.
func newStubIndexer(t *testing.T, handler http.HandlerFunc) (*Indexer, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/test.v1" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	ix := NewIndexer(server.URL, "test", 1, "{}")
	_, err := ix.Init(t.Context())
	require.NoError(t, err)
	t.Cleanup(ix.Close)
	return ix, server
}

func TestBulkReportsItemFailure(t *testing.T) {
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "_bulk") {
			io.WriteString(w, `{"errors":true,"items":[{"index":{"status":400,"error":{"type":"illegal_argument_exception","reason":"bad field"}}}]}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	err := ix.Bulk(t.Context(), []BulkOp{IndexOp("1", map[string]string{"x": "y"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk index failed")
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "illegal_argument_exception")
}

func TestPing(t *testing.T) {
	var status string
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_cluster/health") {
			fmt.Fprintf(w, `{"status":%q}`, status)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
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

func TestBulkAcceptsDelete404(t *testing.T) {
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "_bulk") {
			io.WriteString(w, `{"errors":true,"items":[{"delete":{"status":404,"result":"not_found"}}]}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	require.NoError(t, ix.Bulk(t.Context(), []BulkOp{DeleteOp("missing")}))
}

func TestBulkRejectsNon2xxWithoutErrorField(t *testing.T) {
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "_bulk") {
			io.WriteString(w, `{"errors":true,"items":[{"index":{"status":429}}]}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	err := ix.Bulk(t.Context(), []BulkOp{IndexOp("1", map[string]string{"x": "y"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestBulkWireShape(t *testing.T) {
	var got string
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "_bulk") {
			b, _ := io.ReadAll(r.Body)
			got = string(b)
			assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
			io.WriteString(w, `{"errors":false,"items":[]}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	require.NoError(t, ix.Bulk(t.Context(), []BulkOp{
		IndexOp("a", map[string]int{"v": 1}),
		DeleteOp("b"),
	}))
	want := `{"index":{"_id":"a","_index":"test.v1"}}` + "\n" +
		`{"v":1}` + "\n" +
		`{"delete":{"_id":"b","_index":"test.v1"}}` + "\n"
	assert.Equal(t, want, got)
}

func TestSearchSendsTrackTotalAndBody(t *testing.T) {
	var query string
	var body map[string]any
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "_search") {
			query = r.URL.RawQuery
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			io.WriteString(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	_, err := ix.Search(t.Context(), SearchRequest{
		Query:      TermQuery("repo_id", 7),
		Size:       20,
		From:       40,
		TrackTotal: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "track_total_hits=true", query)
	assert.InDelta(t, 20, body["size"], 0)
	assert.InDelta(t, 40, body["from"], 0)
}

func TestDeleteByQueryBodyShape(t *testing.T) {
	var body map[string]any
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "_delete_by_query") {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			io.WriteString(w, `{"deleted":1}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	require.NoError(t, ix.DeleteByQuery(t.Context(), TermQuery("repo_id", 42)))
	assert.Equal(t, map[string]any{"query": map[string]any{"term": map[string]any{"repo_id": float64(42)}}}, body)
}

func TestDeleteSwallows404(t *testing.T) {
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	require.NoError(t, ix.Delete(t.Context(), "missing"))
}

func TestDeleteEscapesPath(t *testing.T) {
	var path string
	ix, _ := newStubIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			path = r.URL.EscapedPath()
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	require.NoError(t, ix.Delete(t.Context(), "1/foo bar/baz.go"))
	assert.Equal(t, "/test.v1/_doc/1%2Ffoo%20bar%2Fbaz.go", path)
}

func TestBasicAuthFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(r.URL.Path, "/test"):
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	authURL := strings.Replace(server.URL, "http://", "http://admin:secret@", 1)
	ix := NewIndexer(authURL, "test", 1, "{}")
	existed, err := ix.Init(t.Context())
	require.NoError(t, err)
	assert.True(t, existed)
}
