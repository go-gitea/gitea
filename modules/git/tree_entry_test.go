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
	rand.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	assert.NotEqual(t, expected, entries)
	entries.CustomSort(strings.Compare)
	assert.Equal(t, expected, entries)
}
