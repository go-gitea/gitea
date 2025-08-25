// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmoduleLink(t *testing.T) {
	assert.Nil(t, (*SubmoduleFile)(nil).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (*SubmoduleFile)(nil).SubmoduleWebLinkCompare(t.Context(), "", ""))
	assert.Nil(t, (&SubmoduleFile{}).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (&SubmoduleFile{}).SubmoduleWebLinkCompare(t.Context(), "", ""))

	t.Run("GitHubRepo", func(t *testing.T) {
		sf := NewSubmoduleFile("/any/repo-link", "full-path", "git@github.com:user/repo.git", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/tree/aaaa", wl.CommitWebLink)

		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/compare/1111...2222", wl.CommitWebLink)
	})

	t.Run("RelativePath", func(t *testing.T) {
		sf := NewSubmoduleFile("/subpath/any/repo-home-link", "full-path", "../../user/repo", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/tree/aaaa", wl.CommitWebLink)

		sf = NewSubmoduleFile("/subpath/any/repo-home-link", "dir/submodule", "../../user/repo", "aaaa")
		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/compare/1111...2222", wl.CommitWebLink)
	})
}
