// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uri

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadURI(t *testing.T) {
	p, err := filepath.Abs("./uri.go")
	assert.NoError(t, err)
	f, err := Open("file://" + p)
	assert.NoError(t, err)
	defer f.Close()
}

// TestOpenWithClientValidatesRedirectTarget verifies OpenWithClient routes the
// whole request chain (including redirects) through the provided client, so a
// client whose transport refuses to dial an internal target blocks a redirect to
// it — whereas the default client (old Open behavior) follows it.
func TestOpenWithClientValidatesRedirectTarget(t *testing.T) {
	var internalHit atomic.Bool
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internalHit.Store(true)
		_, _ = w.Write([]byte("secret"))
	}))
	defer internal.Close()

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer front.Close()

	internalAddr := strings.TrimPrefix(internal.URL, "http://")

	// a client that refuses to dial the internal target, mimicking the migration
	// hostmatcher dialer that re-validates every hop
	blockingClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == internalAddr {
				return nil, errors.New("blocked internal address")
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}}

	_, err := OpenWithClient(front.URL, blockingClient)
	require.Error(t, err)
	assert.False(t, internalHit.Load(), "the redirect target must not be reached through the validating client")

	// the default client (the previous behavior) follows the redirect to the internal target
	internalHit.Store(false)
	rc, err := Open(front.URL)
	require.NoError(t, err)
	_ = rc.Close()
	assert.True(t, internalHit.Load(), "sanity check: the default client follows the redirect")
}
