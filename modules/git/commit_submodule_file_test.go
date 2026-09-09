// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitSubmoduleLink(t *testing.T) {
	assert.Nil(t, (*CommitSubmoduleFile)(nil).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (*CommitSubmoduleFile)(nil).SubmoduleWebLinkCompare(t.Context(), "", ""))
	assert.Nil(t, (&CommitSubmoduleFile{}).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (&CommitSubmoduleFile{}).SubmoduleWebLinkCompare(t.Context(), "", ""))

	t.Run("GitHubRepo", func(t *testing.T) {
		sf := NewCommitSubmoduleFile("/any/repo-link", "full-path", "git@github.com:user/repo.git", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/tree/aaaa", wl.CommitWebLink)

		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/compare/1111...2222", wl.CommitWebLink)
	})

	t.Run("RelativePath", func(t *testing.T) {
		sf := NewCommitSubmoduleFile("/subpath/any/repo-home-link", "full-path", "../../user/repo", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/tree/aaaa", wl.CommitWebLink)

		sf = NewCommitSubmoduleFile("/subpath/any/repo-home-link", "dir/submodule", "../../user/repo", "aaaa")
		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/compare/1111...2222", wl.CommitWebLink)
	})

	t.Run("UnparsableURL", func(t *testing.T) {
		// both calls share one instance on purpose: the second one used to see the cached parse result
		sf := NewCommitSubmoduleFile("/any/repo-link", "full-path", "git@github.com:", "aaaa")
		assert.Nil(t, sf.SubmoduleWebLinkTree(t.Context()))
		assert.Nil(t, sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222"))
	})
}
