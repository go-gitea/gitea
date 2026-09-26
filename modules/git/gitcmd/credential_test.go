// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRemoteCredentials(t *testing.T) {
	fill := func(addr, request string) string {
		cmd := NewCommand("credential", "fill").AddConfig("credential.helper", "!f() { echo username=other; echo password=other; }; f").
			WithRemoteCredentials(addr).WithStdinBytes([]byte(request))
		stdout, _, _ := cmd.RunStdString(t.Context())
		return stdout
	}

	addr := "https://user:p%40ss@example.com:3000/owner/repo.git"
	assert.Contains(t, fill(addr, "protocol=https\nhost=example.com:3000\n"), "username=user\npassword=p@ss\n")
	assert.NotContains(t, fill(addr, "protocol=https\nhost=other.com\n"), "password=")
	assert.Contains(t, fill("https://:token@example.com/owner/repo.git", "protocol=https\nhost=example.com\n"), "username=\npassword=token\n")
	assert.Contains(t, fill("https://example.com/owner/repo.git", "protocol=https\nhost=example.com\n"), "password=other\n", "no credentials, no helper")

	cmd := NewCommand("fetch").WithRemoteCredentials(addr)
	assert.NotContains(t, cmd.LogString(), "p@ss")
}
