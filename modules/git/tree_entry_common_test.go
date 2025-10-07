// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowLink(t *testing.T) {
	r, err := OpenRepository("tests/repos/repo1_bare")
	require.NoError(t, err)
	defer r.Close()

	commit, err := r.GetCommit(t.Context(), "37991dec2c8e592043f47155ce4808d4580f9123")
	require.NoError(t, err)

	// get the symlink
	{
		lnkFullPath := "foo/bar/link_to_hello"
		lnk, err := commit.Tree.GetTreeEntryByPath(t.Context(), "foo/bar/link_to_hello")
		require.NoError(t, err)
		assert.True(t, lnk.IsLink())

		// should be able to dereference to target
		res, err := EntryFollowLink(t.Context(), commit, lnkFullPath, lnk)
		require.NoError(t, err)
		assert.Equal(t, "hello", res.TargetEntry.Name())
		assert.Equal(t, "foo/nar/hello", res.TargetFullPath)
		assert.False(t, res.TargetEntry.IsLink())
		assert.Equal(t, "b14df6442ea5a1b382985a6549b85d435376c351", res.TargetEntry.ID.String())
	}

	{
		// should error when called on a normal file
		entry, err := commit.Tree.GetTreeEntryByPath(t.Context(), "file1.txt")
		require.NoError(t, err)
		res, err := EntryFollowLink(t.Context(), commit, "file1.txt", entry)
		assert.ErrorIs(t, err, util.ErrUnprocessableContent)
		assert.Nil(t, res)
	}

	{
		// should error for broken links
		entry, err := commit.Tree.GetTreeEntryByPath(t.Context(), "foo/broken_link")
		require.NoError(t, err)
		assert.True(t, entry.IsLink())
		res, err := EntryFollowLink(t.Context(), commit, "foo/broken_link", entry)
		assert.ErrorIs(t, err, util.ErrNotExist)
		assert.Equal(t, "nar/broken_link", res.SymlinkContent)
	}

	{
		// should error for external links
		entry, err := commit.Tree.GetTreeEntryByPath(t.Context(), "foo/outside_repo")
		require.NoError(t, err)
		assert.True(t, entry.IsLink())
		res, err := EntryFollowLink(t.Context(), commit, "foo/outside_repo", entry)
		assert.ErrorIs(t, err, util.ErrNotExist)
		assert.Equal(t, "../../outside_repo", res.SymlinkContent)
	}

	{
		// testing fix for short link bug
		entry, err := commit.Tree.GetTreeEntryByPath(t.Context(), "foo/link_short")
		require.NoError(t, err)
		res, err := EntryFollowLink(t.Context(), commit, "foo/link_short", entry)
		assert.ErrorIs(t, err, util.ErrNotExist)
		assert.Equal(t, "a", res.SymlinkContent)
	}
}
