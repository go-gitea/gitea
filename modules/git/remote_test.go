// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemoteAddr(t *testing.T) {
	for _, c := range []struct{ addr, user, pass, expected string }{
		{"git@github.com:user/repo.git", "", "", "git@github.com:user/repo.git"},
		{"ssh://git@example.com:2222/user/repo.git", "", "", "ssh://git@example.com:2222/user/repo.git"},
		{"https://github.com/user/repo.git", "user", "pass", "https://user:pass@github.com/user/repo.git"},
		{"/srv/git/repo.git", "", "", "/srv/git/repo.git"},
		{"github.com:user/repo.git", "", "", "github.com:user/repo.git"},
		{"/tmp/me@example.com:repo.git", "", "", "/tmp/me@example.com:repo.git"},
	} {
		addr, err := ParseRemoteAddr(c.addr, c.user, c.pass)
		require.NoError(t, err)
		assert.Equal(t, c.expected, addr)
	}

	for _, c := range []struct{ addr, user, pass string }{
		{"git@github.com:user/repo.git", "user", ""},
		{"ssh://git@github.com/user/repo.git", "", "pass"},
		{"ssh://git:secret@github.com/user/repo.git", "", ""},
	} {
		_, err := ParseRemoteAddr(c.addr, c.user, c.pass)
		var addrErr *ErrInvalidCloneAddr
		require.ErrorAs(t, err, &addrErr)
		assert.True(t, addrErr.IsAuthNotSupported)
	}

	for _, addr := range []string{"ssh://git%40127.0.0.1%3A2222%2Fx@example.com/repo.git", "ssh://git@example.com?@127.0.0.1/repo.git", "ssh://git@example.com#@127.0.0.1/repo.git"} {
		_, err := ParseRemoteAddr(addr, "", "")
		var addrErr *ErrInvalidCloneAddr
		require.ErrorAs(t, err, &addrErr)
		assert.True(t, addrErr.IsURLError)
	}
}

func TestIsRemoteNotExistError(t *testing.T) {
	_, err := GetRemoteAddress(t.Context(), mockRepository("repo1_bare"), "no-such-remote")
	assert.True(t, IsRemoteNotExistError(err))
}
