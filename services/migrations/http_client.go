// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/log"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// migrationHTTPClient is the shared migration client. Callers that would otherwise build a client per
// request use it (via getMigrationHTTPClient) so a single connection pool is reused across downloads —
// e.g. many release assets from the same host — instead of a fresh pool and TLS handshake each time. It
// is built lazily on first use and reset by Init whenever the allow/block lists change; OnceValue keeps
// concurrent callers sharing a single client instead of racing to create their own.
var migrationHTTPClient = util.OnceValue[*http.Client]{Func: func() *http.Client {
	return &http.Client{Transport: NewMigrationHTTPTransport()}
}}

func newMigrationHTTPClient(baseURL, authorization string) *http.Client {
	return &http.Client{Transport: authTransport(getMigrationHTTPClient().Transport, baseURL, authorization)}
}

// getMigrationHTTPClient returns the shared migration client, building it on first use so no request
// escapes the SSRF-validated transport even before Init has run.
func getMigrationHTTPClient() *http.Client {
	return migrationHTTPClient.Value()
}

// NewMigrationHTTPTransport returns a HTTP transport for migration. The target is validated against the
// allow/block lists on both the direct-dial and proxy paths, so a configured proxy cannot be used to
// reach an otherwise-forbidden target (SSRF).
func NewMigrationHTTPTransport() http.RoundTripper {
	return retryAfterTransport{hostmatcher.NewHTTPTransport("migration", allowList, blockList, proxy.Proxy(), setting.Proxy.ProxyURLFixed,
		&tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify})}
}

type retryAfterTransport struct {
	http.RoundTripper
}

func (t retryAfterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var waited time.Duration
	for retries := 0; ; retries++ {
		resp, err := t.RoundTripper.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusForbidden) || retries == 5 || (req.Body != nil && req.GetBody == nil) { // GitHub's secondary rate limits answer 403
			return resp, nil
		}
		delay, ok := parseRetryAfter(resp.Header.Get("Retry-After"))
		if !ok || waited+delay > time.Hour {
			return resp, nil
		}
		waited += delay
		resp.Body.Close()
		log.Info("Rate limited by %s, retrying in %s", req.URL.Host, delay)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req = req.Clone(req.Context())
			req.Body = body
		}
	}
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil { // 32 bits can't overflow the Duration
		return time.Duration(seconds) * time.Second, true
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		return max(time.Until(retryAt), 0), true
	}
	return 0, false
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

func authTransport(transport http.RoundTripper, baseURL, authorization string) http.RoundTripper {
	base, err := url.Parse(baseURL)
	if authorization == "" || err != nil {
		return transport
	}
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == base.Host && (req.URL.Scheme == "https" || base.Scheme == "http") { // the source host only, never downgraded to http
			req = req.Clone(req.Context())
			req.Header.Set("Authorization", authorization)
		}
		return transport.RoundTrip(req)
	})
}

func basicAuthorization(username, password string) string {
	if username == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}
