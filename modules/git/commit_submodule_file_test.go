// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitSubmoduleLink(t *testing.T) {
	sf := NewCommitSubmoduleFile("git@github.com:user/repo.git", "aaaa")

	wl := sf.SubmoduleWebLink(context.Background())
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/commit/aaaa", wl.CommitWebLink)

	wl = sf.SubmoduleWebLink(context.Background(), "1111")
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/commit/1111", wl.CommitWebLink)

	wl = sf.SubmoduleWebLink(context.Background(), "1111", "2222")
	assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
	assert.Equal(t, "https://github.com/user/repo/compare/1111...2222", wl.CommitWebLink)

	wl = (*CommitSubmoduleFile)(nil).SubmoduleWebLink(context.Background())
	assert.Nil(t, wl)
}
