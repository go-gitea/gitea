// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// getMigrationHTTPClient returns the shared migration client, so downloads from one host reuse its connections
var getMigrationHTTPClient = sync.OnceValue(func() *http.Client {
	return &http.Client{Transport: retryAfterTransport{getMigrationTransport()}}
})

var getMigrationTransport = sync.OnceValue(newMigrationTransport)

func newMigrationHTTPClient(baseURL, authorization string) *http.Client {
	return &http.Client{Transport: authTransport(getMigrationHTTPClient().Transport, baseURL, authorization)}
}

// NewMigrationHTTPTransport returns a HTTP transport for migration, enforcing the migration policy on its direct dials.
func NewMigrationHTTPTransport() http.RoundTripper {
	return retryAfterTransport{newMigrationTransport()}
}

func newMigrationTransport() *http.Transport {
	t := egress.NewMigrationPolicy().NewHTTPTransport()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify}
	return t
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
		if !ok || delay > time.Hour-waited {
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
