// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitSubmoduleLink(t *testing.T) {
	sf := NewCommitSubmoduleFile("git@github.com:user/repo.git", "aaaa")

	wl := sf.SubmoduleWebLink(t.Context())
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/tree/aaaa", wl.CommitWebLink)

	wl = sf.SubmoduleWebLink(t.Context(), "1111")
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/tree/1111", wl.CommitWebLink)

	wl = sf.SubmoduleWebLink(t.Context(), "1111", "2222")
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/compare/1111...2222", wl.CommitWebLink)

	wl = (*CommitSubmoduleFile)(nil).SubmoduleWebLink(t.Context())
	assert.Nil(t, wl)
}
