// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTreeEntriesLong(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected []*TreeEntry
	}{
		{
			Input: `100644 blob ea0d83c9081af9500ac9f804101b3fd0a5c293af    8218	README.md
100644 blob 037f27dc9d353ae4fd50f0474b2194c593914e35    4681	README_ZH.md
100644 blob 9846a94f7e8350a916632929d0fda38c90dd2ca8     429	SECURITY.md
040000 tree 84b90550547016f73c5dd3f50dea662389e67b6d       -	assets
`,
			Expected: []*TreeEntry{
				{
					ID:        MustIDFromString("ea0d83c9081af9500ac9f804101b3fd0a5c293af"),
					name:      "README.md",
					entryMode: EntryModeBlob,
					size:      8218,
					sized:     true,
				},
				{
					ID:        MustIDFromString("037f27dc9d353ae4fd50f0474b2194c593914e35"),
					name:      "README_ZH.md",
					entryMode: EntryModeBlob,
					size:      4681,
					sized:     true,
				},
				{
					ID:        MustIDFromString("9846a94f7e8350a916632929d0fda38c90dd2ca8"),
					name:      "SECURITY.md",
					entryMode: EntryModeBlob,
					size:      429,
					sized:     true,
				},
				{
					ID:        MustIDFromString("84b90550547016f73c5dd3f50dea662389e67b6d"),
					name:      "assets",
					entryMode: EntryModeTree,
					sized:     true,
				},
			},
		},
	}
	for _, testCase := range testCases {
		entries, err := ParseTreeEntries([]byte(testCase.Input))
		assert.NoError(t, err)
		assert.Len(t, entries, len(testCase.Expected))
		for i, entry := range entries {
			assert.EqualValues(t, testCase.Expected[i], entry)
		}
	}
}

func TestParseTreeEntriesShort(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected []*TreeEntry
	}{
		{
			Input: `100644 blob ea0d83c9081af9500ac9f804101b3fd0a5c293af	README.md
040000 tree 84b90550547016f73c5dd3f50dea662389e67b6d	assets
`,
			Expected: []*TreeEntry{
				{
					ID:        MustIDFromString("ea0d83c9081af9500ac9f804101b3fd0a5c293af"),
					name:      "README.md",
					entryMode: EntryModeBlob,
				},
				{
					ID:        MustIDFromString("84b90550547016f73c5dd3f50dea662389e67b6d"),
					name:      "assets",
					entryMode: EntryModeTree,
				},
			},
		},
	}
	for _, testCase := range testCases {
		entries, err := ParseTreeEntries([]byte(testCase.Input))
		assert.NoError(t, err)
		assert.Len(t, entries, len(testCase.Expected))
		for i, entry := range entries {
			assert.EqualValues(t, testCase.Expected[i], entry)
		}
	}
}

func TestParseTreeEntriesInvalid(t *testing.T) {
	// there was a panic: "runtime error: slice bounds out of range" when the input was invalid: #20315
	entries, err := ParseTreeEntries([]byte("100644 blob ea0d83c9081af9500ac9f804101b3fd0a5c293af"))
	assert.Error(t, err)
	assert.Len(t, entries, 0)
}
