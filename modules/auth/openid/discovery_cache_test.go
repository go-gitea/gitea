// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDiscoveredInfo struct{}

func (s *testDiscoveredInfo) ClaimedID() string {
	return "claimedID"
}

func (s *testDiscoveredInfo) OpEndpoint() string {
	return "opEndpoint"
}

func (s *testDiscoveredInfo) OpLocalID() string {
	return "opLocalID"
}

func TestTimedDiscoveryCache(t *testing.T) {
	ttl := 50 * time.Millisecond
	dc := newTimedDiscoveryCache(ttl)

	// Put some initial values
	dc.Put("foo", &testDiscoveredInfo{}) // openid.opEndpoint: "a", openid.opLocalID: "b", openid.claimedID: "c"})

	// Make sure we can retrieve them
	di := dc.Get("foo")
	require.NotNil(t, di)
	assert.Equal(t, "opEndpoint", di.OpEndpoint())
	assert.Equal(t, "opLocalID", di.OpLocalID())
	assert.Equal(t, "claimedID", di.ClaimedID())

	// Attempt to get a non-existent value
	assert.Nil(t, dc.Get("bar"))

	// Sleep for a while and try to retrieve again
	time.Sleep(ttl * 3 / 2)

	assert.Nil(t, dc.Get("foo"))
}
