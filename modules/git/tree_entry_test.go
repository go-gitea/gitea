// Copyright 2017 The Gitea Authors. All rights reserved.
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
	assert.False(t, target.IsLink())
	assert.Equal(t, "b14df6442ea5a1b382985a6549b85d435376c351", target.ID.String())

	// should error when called on normal file
	target, err = commit.Tree.GetTreeEntryByPath("file1.txt")
	assert.NoError(t, err)
	_, err = target.FollowLink()
	assert.EqualError(t, err, "file1.txt: not a symlink")

	// should error for broken links
	target, err = commit.Tree.GetTreeEntryByPath("foo/broken_link")
	assert.NoError(t, err)
	assert.True(t, target.IsLink())
	_, err = target.FollowLink()
	assert.EqualError(t, err, "broken_link: broken link")

	// should error for external links
	target, err = commit.Tree.GetTreeEntryByPath("foo/outside_repo")
	assert.NoError(t, err)
	assert.True(t, target.IsLink())
	_, err = target.FollowLink()
	assert.EqualError(t, err, "outside_repo: points outside of repo")

	// testing fix for short link bug
	target, err = commit.Tree.GetTreeEntryByPath("foo/link_short")
	assert.NoError(t, err)
	_, err = target.FollowLink()
	assert.EqualError(t, err, "link_short: broken link")
}

func TestGetPathInRepo(t *testing.T) {
	r, err := openRepositoryWithDefaultContext("tests/repos/repo1_bare")
	assert.NoError(t, err)
	defer r.Close()

	commit, err := r.GetCommit("37991dec2c8e592043f47155ce4808d4580f9123")
	assert.NoError(t, err)

	// nested entry
	entry, err := commit.Tree.GetTreeEntryByPath("foo/bar/link_to_hello")
	assert.NoError(t, err)
	path := entry.GetPathInRepo()
	assert.Equal(t, "foo/bar/link_to_hello", path)

	// folder
	entry, err = commit.Tree.GetTreeEntryByPath("foo/bar")
	assert.NoError(t, err)
	path = entry.GetPathInRepo()
	assert.Equal(t, "foo/bar", path)

	// top level file
	entry, err = commit.Tree.GetTreeEntryByPath("file2.txt")
	assert.NoError(t, err)
	path = entry.GetPathInRepo()
	assert.Equal(t, "file2.txt", path)
}
