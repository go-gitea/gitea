// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

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
