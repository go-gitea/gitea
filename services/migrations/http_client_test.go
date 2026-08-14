// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	"gitea.dev/modules/hostmatcher"

	"github.com/stretchr/testify/assert"
)

// TestShouldRetryRequestPermanentErrors: a permanent transport failure never
// succeeds on a retry, so retrying it only spends the whole backoff budget (and,
// with the retrying downloader on top, three times over) before surfacing. A
// migration pointed at a blocked host must fail immediately, not 31s later.
func TestShouldRetryRequestPermanentErrors(t *testing.T) {
	// as they arrive at the transport: wrapped by the dialer and the http client
	wrapped := func(err error) error {
		return &url.Error{Op: "Post", URL: "https://example.com", Err: &net.OpError{Op: "dial", Err: err}}
	}

	for _, tc := range []struct {
		name  string
		err   error
		retry bool
	}{
		{"blocked by policy", wrapped(fmt.Errorf("migration can not call blocked HTTP servers: %w", hostmatcher.ErrDialNotAllowed)), false},
		{"host does not resolve", wrapped(&net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}), false},
		{"TLS verification failed", wrapped(&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}), false},
		{"DNS server failure", wrapped(&net.DNSError{Err: "server misbehaving", Name: "example.com", IsTemporary: true}), true},
		{"connection reset", wrapped(errors.New("connection reset by peer")), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.retry, shouldRetryRequest(nil, tc.err))
		})
	}
}

// TestShouldRetryRequestStatuses documents which responses are the transport's to
// retry: any 5xx, and a secondary rate limit only when it says how long to wait.
// The primary rate limit is the downloader's to handle.
func TestShouldRetryRequestStatuses(t *testing.T) {
	retryAfter := http.Header{"Retry-After": {"30"}}
	for _, tc := range []struct {
		code   int
		header http.Header
		retry  bool
	}{
		{http.StatusBadGateway, nil, true},
		{http.StatusGatewayTimeout, nil, true},
		{http.StatusTooManyRequests, retryAfter, true},
		{http.StatusTooManyRequests, nil, false},
		{http.StatusForbidden, nil, false},
		{http.StatusNotFound, nil, false},
		{http.StatusOK, nil, false},
	} {
		header := tc.header
		if header == nil {
			header = http.Header{}
		}
		resp := &http.Response{StatusCode: tc.code, Header: header, Body: http.NoBody}
		assert.Equal(t, tc.retry, shouldRetryRequest(resp, nil), "HTTP %d, Retry-After=%q", tc.code, header.Get("Retry-After"))
	}
}
