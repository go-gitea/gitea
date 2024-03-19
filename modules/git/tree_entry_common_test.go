// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFollowLink(t *testing.T) {
	r, err := openRepositoryWithDefaultContext("tests/repos/repo1_bare")
	assert.NoError(t, err)
	defer r.Close()

	commit, err := r.GetCommit("37991dec2c8e592043f47155ce4808d4580f9123")
	assert.NoError(t, err)

	// get the symlink
	lnk, err := commit.Tree.GetTreeEntryByPath("foo/bar/link_to_hello")
	assert.NoError(t, err)
	assert.True(t, lnk.IsLink())

	// should be able to dereference to target
	target, err := lnk.FollowLink()
	assert.NoError(t, err)
	assert.Equal(t, "hello", target.Name())
	assert.Equal(t, "foo/nar/hello", target.FullPath())
	assert.False(t, target.IsLink())
	assert.Equal(t, "b14df6442ea5a1b382985a6549b85d435376c351", target.ID.String())

	// should error when called on normal file
	target, err = commit.Tree.GetTreeEntryByPath("file1.txt")
	assert.NoError(t, err)
	_, err = target.FollowLink()
	assert.ErrorAs(t, err, ErrBadLink{})

	// should error for broken links
	target, err = commit.Tree.GetTreeEntryByPath("foo/broken_link")
	assert.NoError(t, err)
	assert.True(t, target.IsLink())
	_, err = target.FollowLink()
	assert.ErrorAs(t, err, ErrBadLink{})

	// should error for external links
	target, err = commit.Tree.GetTreeEntryByPath("foo/outside_repo")
	assert.NoError(t, err)
	assert.True(t, target.IsLink())
	_, err = target.FollowLink()
	assert.ErrorAs(t, err, ErrBadLink{})

	// testing fix for short link bug
	target, err = commit.Tree.GetTreeEntryByPath("foo/link_short")
	assert.NoError(t, err)
	_, err = target.FollowLink()
	assert.ErrorAs(t, err, ErrBadLink{})
}

func TestTryFollowingLinks(t *testing.T) {
	r, err := openRepositoryWithDefaultContext("tests/repos/repo1_bare")
	assert.NoError(t, err)
	defer r.Close()

	commit, err := r.GetCommit("37991dec2c8e592043f47155ce4808d4580f9123")
	assert.NoError(t, err)

	// get the symlink
	list, err := commit.Tree.GetTreeEntryByPath("foo/bar/link_to_hello")
	assert.NoError(t, err)
	assert.True(t, list.IsLink())

	// should be able to dereference to target
	target := list.TryFollowingLinks()
	assert.NotEqual(t, target, list)
	assert.Equal(t, "hello", target.Name())
	assert.Equal(t, "foo/nar/hello", target.FullPath())
	assert.False(t, target.IsLink())
	assert.Equal(t, "b14df6442ea5a1b382985a6549b85d435376c351", target.ID.String())

	// should default to original when called on normal file
	link, err := commit.Tree.GetTreeEntryByPath("file1.txt")
	assert.NoError(t, err)
	target = link.TryFollowingLinks()
	assert.Same(t, link, target)

	// should default to original for broken links
	link, err = commit.Tree.GetTreeEntryByPath("foo/broken_link")
	assert.NoError(t, err)
	assert.True(t, link.IsLink())
	target = link.TryFollowingLinks()
	assert.Same(t, link, target)

	// should default to original for external links
	link, err = commit.Tree.GetTreeEntryByPath("foo/outside_repo")
	assert.NoError(t, err)
	assert.True(t, link.IsLink())
	target = link.TryFollowingLinks()
	assert.Same(t, link, target)

	// testing fix for short link bug
	link, err = commit.Tree.GetTreeEntryByPath("foo/link_short")
	assert.NoError(t, err)
	target = link.TryFollowingLinks()
	assert.Same(t, link, target)
}
