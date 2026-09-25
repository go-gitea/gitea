// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitProxyURL(t *testing.T) {
	t.Cleanup(func() { SetGitProxyURL("") })

	assert.Empty(t, GitProxyURL())

	SetGitProxyURL("http://127.0.0.1:37891")
	assert.Equal(t, "http://127.0.0.1:37891", GitProxyURL())

	SetGitProxyURL("")
	assert.Empty(t, GitProxyURL())
}
