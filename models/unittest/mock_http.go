// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockServerOptions tweaks NewMockWebServer behavior.
type MockServerOptions struct {
	// Routes installs extra handlers on the mux before the fixture fallback;
	// more specific patterns win.
	Routes func(mux *http.ServeMux)
	// StripPrefix is trimmed from the request path before forwarding upstream,
	// useful when the client prepends a prefix the real upstream does not use
	// (e.g. go-github prepends "/api/v3").
	StripPrefix string
}

// NewMockWebServer returns a test HTTP server that records upstream responses on demand
// and replays them from disk on subsequent runs.
//
//   - liveMode=true: requests are forwarded to liveServerBaseURL and responses written as
//     fixture files under testDataDir.
//   - liveMode=false: responses come from existing fixture files.
//
// Fixture format: header lines ("Name: value"), a blank line, then the body. Before
// replay, occurrences of liveServerBaseURL in the body are swapped for the mock URL.
//
// The typical switch is an env var holding an API token; fixtures ship committed so the
// default run (no token) works offline.
//
//	token := os.Getenv("GITEA_TOKEN")
//	mock := NewMockWebServer(t, "https://gitea.com", fixtureDir, token != "")
func NewMockWebServer(t *testing.T, liveServerBaseURL, testDataDir string, liveMode bool, opts ...MockServerOptions) *httptest.Server {
	t.Helper()

	var opt MockServerOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	ignoredHeaders := []string{"cf-ray", "server", "date", "report-to", "nel", "x-request-id", "set-cookie"}

	var mockURL string

	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			reqPath += "?" + r.URL.RawQuery
		}
		log.Info("mock server: %s %s", r.Method, reqPath)

		fixturePath := fmt.Sprintf("%s/%s_%s", testDataDir, r.Method, url.QueryEscape(reqPath))
		if strings.Contains(r.URL.Path, ".git/") {
			fixturePath = fmt.Sprintf("%s/%s_%s", testDataDir, r.Method, url.QueryEscape(r.URL.Path))
		}

		if liveMode {
			require.NoError(t, os.MkdirAll(testDataDir, 0o755))

			liveURL := liveServerBaseURL + strings.TrimPrefix(reqPath, opt.StripPrefix)
			req, err := http.NewRequest(r.Method, liveURL, r.Body)
			require.NoError(t, err, "building upstream request to %s", liveURL)
			for name, values := range r.Header {
				if strings.EqualFold(name, "accept-encoding") {
					continue
				}
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "upstream request to %s failed", liveURL)
			defer resp.Body.Close()
			assert.Less(t, resp.StatusCode, 400, "upstream %s returned status %d", liveURL, resp.StatusCode)

			out, err := os.Create(fixturePath)
			require.NoError(t, err, "creating fixture %s", fixturePath)
			defer out.Close()

			for _, name := range slices.Sorted(maps.Keys(resp.Header)) {
				if slices.Contains(ignoredHeaders, strings.ToLower(name)) {
					continue
				}
				for _, value := range resp.Header[name] {
					_, err := fmt.Fprintf(out, "%s: %s\n", name, value)
					require.NoError(t, err)
				}
			}
			_, err = out.WriteString("\n")
			require.NoError(t, err)

			_, err = io.Copy(out, resp.Body)
			require.NoError(t, err, "writing fixture body for %s", liveURL)
			require.NoError(t, out.Sync())
		}

		raw, err := os.ReadFile(fixturePath)
		require.NoError(t, err, "missing fixture: %s", fixturePath)

		replayed := strings.ReplaceAll(string(raw), liveServerBaseURL, mockURL)
		headers, body, _ := strings.Cut(replayed, "\n\n")
		for line := range strings.SplitSeq(headers, "\n") {
			name, value, ok := strings.Cut(line, ": ")
			if !ok || strings.EqualFold(name, "Content-Length") {
				continue
			}
			w.Header().Set(name, value)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	})

	mux := http.NewServeMux()
	if opt.Routes != nil {
		opt.Routes(mux)
	}
	mux.Handle("/", fallback)

	server := httptest.NewServer(mux)
	mockURL = server.URL
	t.Cleanup(server.Close)
	return server
}
