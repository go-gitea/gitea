// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSrvHost(t *testing.T) {
	assert.Equal(t, "avatars.example.com", srvHost(&net.SRV{Target: "avatars.example.com.", Port: 443}, 443))
	assert.Equal(t, "avatars.example.com:8443", srvHost(&net.SRV{Target: "avatars.example.com.", Port: 8443}, 443))
	assert.Empty(t, srvHost(&net.SRV{Target: ".", Port: 443}, 443))
}
