// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func getTestEntries() Entries {
	return Entries{
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v1.0", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v2.0", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v2.1", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v2.12", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v2.2", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "v12.0", Mode: filemode.Dir}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "abc", Mode: filemode.Regular}},
		&TreeEntry{gogitTreeEntry: &object.TreeEntry{Name: "bcd", Mode: filemode.Regular}},
	}
}

func TestEntriesSort(t *testing.T) {
	entries := getTestEntries()
	entries.Sort()
	assert.Equal(t, "v1.0", entries[0].Name())
	assert.Equal(t, "v12.0", entries[1].Name())
	assert.Equal(t, "v2.0", entries[2].Name())
	assert.Equal(t, "v2.1", entries[3].Name())
	assert.Equal(t, "v2.12", entries[4].Name())
	assert.Equal(t, "v2.2", entries[5].Name())
	assert.Equal(t, "abc", entries[6].Name())
	assert.Equal(t, "bcd", entries[7].Name())
}

func TestEntriesCustomSort(t *testing.T) {
	entries := getTestEntries()
	entries.CustomSort(func(s1, s2 string) bool {
		return s1 > s2
	})
	assert.Equal(t, "v2.2", entries[0].Name())
	assert.Equal(t, "v2.12", entries[1].Name())
	assert.Equal(t, "v2.1", entries[2].Name())
	assert.Equal(t, "v2.0", entries[3].Name())
	assert.Equal(t, "v12.0", entries[4].Name())
	assert.Equal(t, "v1.0", entries[5].Name())
	assert.Equal(t, "bcd", entries[6].Name())
	assert.Equal(t, "abc", entries[7].Name())
}

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
