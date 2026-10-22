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

	"gitea.dev/modules/egress/policy"

	"github.com/stretchr/testify/assert"
)

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
		{"blocked by policy", wrapped(fmt.Errorf("migration can not call blocked HTTP servers: %w", policy.ErrDenied)), false},
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
