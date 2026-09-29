// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
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
var migrationHTTPClient = util.OnceValue[*http.Client]{Func: newMigrationHTTPClient}

// newMigrationHTTPClient returns a HTTP client for migration with retry support
func newMigrationHTTPClient() *http.Client {
	return &http.Client{
		Transport: newRetryTransport(NewMigrationHTTPTransport()),
	}
}

// getMigrationHTTPClient returns the shared migration client, building it on first use so no request
// escapes the SSRF-validated transport even before Init has run.
func getMigrationHTTPClient() *http.Client {
	return migrationHTTPClient.Value()
}

// NewMigrationHTTPTransport returns a HTTP transport for migration. The target is validated against the
// allow/block lists on both the direct-dial and proxy paths, so a configured proxy cannot be used to
// reach an otherwise-forbidden target (SSRF).
func NewMigrationHTTPTransport() *http.Transport {
	return hostmatcher.NewHTTPTransport("migration", allowList, blockList, proxy.Proxy(), setting.Proxy.ProxyURLFixed,
		&tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify})
}

const (
	retryMaxRetries = 5
	// cap an honored Retry-After so a hostile/huge value can't stall a sync
	retryMaxRetryAfter = 5 * time.Minute
)

// retryBaseDelay is the initial backoff between transport-level retries;
// doubled each retry. A variable so tests can shrink it.
var retryBaseDelay = time.Second

// retryTransport wraps an http.RoundTripper and transparently retries migration
// API requests that fail with transient errors: transient network failures (see
// isTransientTransportError) and 5xx responses (e.g. a 502/503/504 from GitHub),
// plus secondary-rate-limit responses (403/429) that carry a Retry-After. Without
// this, a single transient hiccup — such as a 504 while fetching one issue's
// reactions midway through a large repository's metadata sweep — aborts the entire
// sync. Primary rate limit
// (403 with X-RateLimit-Remaining: 0 and no Retry-After) is intentionally NOT
// retried here; the downloader already waits for the reset window before its
// calls, so we leave that handling to it.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
}

func newRetryTransport(base http.RoundTripper) http.RoundTripper {
	return &retryTransport{base: base, maxRetries: retryMaxRetries, baseDelay: retryBaseDelay}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body so the request can be replayed on each retry.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	delay := t.baseDelay
	for attempt := 0; ; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := t.base.RoundTrip(req)
		if attempt >= t.maxRetries || !shouldRetryRequest(resp, err) {
			return resp, err
		}

		wait := delay
		if resp != nil {
			if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra > 0 {
				wait = min(ra, retryMaxRetryAfter)
			}
			// drain and close so the underlying connection can be reused
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		log.Warn("migration: transient request failure (%s), retry %d/%d in %s", retryReason(resp, err), attempt+1, t.maxRetries, wait)

		timer := time.NewTimer(wait)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}

		delay *= 2
	}
}

// shouldRetryRequest reports whether a request should be retried given its result.
func shouldRetryRequest(resp *http.Response, err error) bool {
	if err != nil {
		// network error, timeout, connection reset, unexpected EOF, etc.
		return isTransientTransportError(err)
	}
	switch resp.StatusCode {
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	case http.StatusForbidden, http.StatusTooManyRequests:
		// Secondary/abuse rate limit signals a Retry-After; retry only then.
		// Primary rate limit (no Retry-After) is handled by the downloader.
		return resp.Header.Get("Retry-After") != ""
	}
	return false
}

// isTransientTransportError reports whether a transport-level failure can plausibly
// succeed on a retry. A permanent one — the target is refused by the allow/block
// lists, does not resolve, or fails TLS verification — never will, so it must
// surface immediately instead of spending the whole backoff budget first (and, with
// the retrying downloader on top, three times over). Anything unrecognized stays
// retryable: a reset connection or a truncated body is the common case.
func isTransientTransportError(err error) bool {
	if errors.Is(err, hostmatcher.ErrDialNotAllowed) {
		return false
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return false
	}
	if dnsErr, ok := errors.AsType[*net.DNSError](err); ok {
		return !dnsErr.IsNotFound
	}
	return true
}

func retryReason(resp *http.Response, err error) string {
	if err != nil {
		return err.Error()
	}
	return "HTTP " + strconv.Itoa(resp.StatusCode)
}

// parseRetryAfter parses a Retry-After header expressed in seconds. GitHub uses
// the delta-seconds form; an HTTP-date form (or anything unparsable) yields 0 so
// the caller falls back to exponential backoff.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
