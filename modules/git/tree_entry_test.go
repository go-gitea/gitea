// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntriesCustomSort(t *testing.T) {
	entries := Entries{
		&TreeEntry{name: "a-dir", entryMode: EntryModeTree},
		&TreeEntry{name: "a-submodule", entryMode: EntryModeCommit},
		&TreeEntry{name: "b-dir", entryMode: EntryModeTree},
		&TreeEntry{name: "b-submodule", entryMode: EntryModeCommit},
		&TreeEntry{name: "a-file", entryMode: EntryModeBlob},
		&TreeEntry{name: "b-file", entryMode: EntryModeBlob},
	}
	expected := slices.Clone(entries)
	for slices.Equal(expected, entries) {
		rand.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	}
	entries.CustomSort(strings.Compare)
	assert.Equal(t, expected, entries)
}

func TestParseEntryMode(t *testing.T) {
	tests := []struct {
		modeStr   string
		expectMod EntryMode
	}{
		{"000000", EntryModeNoEntry},
		{"000755", EntryModeNoEntry},

		{"100644", EntryModeBlob},
		{"100755", EntryModeExec},

		{"120000", EntryModeSymlink},
		{"120755", EntryModeSymlink},
		{"160000", EntryModeCommit},
		{"160755", EntryModeCommit},

		{"040000", EntryModeTree},
		{"040755", EntryModeTree},

		{"777777", EntryModeNoEntry}, // invalid mode
	}
	for _, test := range tests {
		mod := ParseEntryMode(test.modeStr)
		assert.Equal(t, test.expectMod, mod, "modeStr: %s", test.modeStr)
	}
}
