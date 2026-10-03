// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	for _, request := range []string{"protocol=https\nhost=other.com\n", "protocol=https\nhost=example.com\n", "protocol=http\nhost=example.com:3000\n"} {
		assert.NotContains(t, fill(addr, request), "password=", request)
	}
	assert.Contains(t, fill("HTTPS://user:token@example.com/owner/repo.git", "protocol=https\nhost=example.com\n"), "username=user\npassword=token\n")
	assert.Contains(t, fill("https://example.com/owner/repo.git", "protocol=https\nhost=example.com\n"), "password=other\n", "no credentials, no helper")

	tokenAddr := "https://:to=ken@exämple.com/owner/repo.git"
	stdout, _, err := NewCommand("ls-remote", "--get-url").AddDynamicArguments(RemoteAddressWithoutCredentials(tokenAddr)).WithRemoteCredentials(tokenAddr).
		WithEnv(append(os.Environ(), "GIT_CONFIG_PARAMETERS=")).RunStdString(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "https://:to%3Dken@ex%C3%A4mple.com/owner/repo.git\n", stdout)
	stdout, _, err = NewCommand("config", "http.proxy").WithRemoteCredentials(tokenAddr).
		WithEnv(append(os.Environ(), "GIT_CONFIG_PARAMETERS='http.proxy=http://proxy.example'")).RunStdString(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example\n", stdout)

	for _, brokenAddr := range []string{"https://user:a%0Ab@example.com/owner/repo.git", "https://user:token@a.b=c/owner/repo.git"} {
		_, _, err = NewCommand("fetch").WithRemoteCredentials(brokenAddr).RunStdString(t.Context())
		assert.ErrorIs(t, err, ErrBrokenCommand, brokenAddr)
	}

	for _, credentialsAddr := range []string{addr, tokenAddr} {
		cmd := NewCommand("fetch").WithRemoteCredentials(credentialsAddr)
		assert.NotContains(t, strings.Join(append(cmd.configArgs, cmd.args...), " "), "@", credentialsAddr)
	}

	assert.Equal(t, "https://example.com:3000/owner/repo.git", RemoteAddressWithoutCredentials(addr))
	assert.Equal(t, "https://example.com/", RemoteAddressWithoutCredentials("HTTPS://:token@example.com"))
	for _, keptAddr := range []string{"https://example.com/owner/repo.git", "git@example.com:owner/repo.git", "/data/owner/repo.git"} {
		assert.Equal(t, keptAddr, RemoteAddressWithoutCredentials(keptAddr))
	}
}
