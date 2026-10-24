// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowLink(t *testing.T) {
	r, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer r.Close()

	commit, err := r.GetCommit(t.Context(), "37991dec2c8e592043f47155ce4808d4580f9123")
	require.NoError(t, err)

	// get the symlink
	{
		lnkFullPath := "foo/bar/link_to_hello"
		lnk, err := commit.Tree().GetTreeEntryByPath(t.Context(), r, "foo/bar/link_to_hello")
		require.NoError(t, err)
		assert.True(t, lnk.IsLink())

		// should be able to dereference to target
		res, err := EntryFollowLink(t.Context(), r, commit, lnkFullPath, lnk)
		require.NoError(t, err)
		assert.Equal(t, "hello", res.TargetEntry.Name())
		assert.Equal(t, "foo/nar/hello", res.TargetFullPath)
		assert.False(t, res.TargetEntry.IsLink())
		assert.Equal(t, "b14df6442ea5a1b382985a6549b85d435376c351", res.TargetEntry.ID.String())
	}

	{
		// should error when called on a normal file
		entry, err := commit.Tree().GetTreeEntryByPath(t.Context(), r, "file1.txt")
		require.NoError(t, err)
		res, err := EntryFollowLink(t.Context(), r, commit, "file1.txt", entry)
		assert.ErrorIs(t, err, util.ErrUnprocessableContent)
		assert.Nil(t, res)
	}

	for _, test := range []struct {
		fullPath       string
		symlinkContent string
		checkLink      bool
	}{
		{"foo/broken_link", "nar/broken_link", true},     // should error for broken links
		{"foo/outside_repo", "../../outside_repo", true}, // should error for external links
		{"foo/link_short", "a", false},                   // testing fix for short link bug
	} {
		entry, err := commit.Tree().GetTreeEntryByPath(t.Context(), r, test.fullPath)
		require.NoError(t, err)
		if test.checkLink {
			assert.True(t, entry.IsLink())
		}
		res, err := EntryFollowLink(t.Context(), r, commit, test.fullPath, entry)
		assert.ErrorIs(t, err, util.ErrNotExist)
		assert.Equal(t, test.symlinkContent, res.SymlinkContent)
	}
}
