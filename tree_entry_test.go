// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func getTestEntries() Entries {
	return Entries{
		&TreeEntry{name: "v1.0", mode: EntryModeTree},
		&TreeEntry{name: "v2.0", mode: EntryModeTree},
		&TreeEntry{name: "v2.1", mode: EntryModeTree},
		&TreeEntry{name: "v2.12", mode: EntryModeTree},
		&TreeEntry{name: "v2.2", mode: EntryModeTree},
		&TreeEntry{name: "v12.0", mode: EntryModeTree},
		&TreeEntry{name: "abc", mode: EntryModeBlob},
		&TreeEntry{name: "bcd", mode: EntryModeBlob},
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
