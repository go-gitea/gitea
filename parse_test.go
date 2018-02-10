// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTreeEntries(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected []*TreeEntry
	}{
		{
			Input:    "",
			Expected: []*TreeEntry{},
		},
		{
			Input: "100644 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c\texample/file2.txt\n",
			Expected: []*TreeEntry{
				{
					mode: EntryModeBlob,
					Type: ObjectBlob,
					ID:   MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					name: "example/file2.txt",
				},
			},
		},
		{
			Input: "120000 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c\t\"example/\\n.txt\"\n" +
				"040000 tree 1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8\texample\n",
			Expected: []*TreeEntry{
				{
					ID:   MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					Type: ObjectBlob,
					mode: EntryModeSymlink,
					name: "example/\n.txt",
				},
				{
					ID:   MustIDFromString("1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8"),
					Type: ObjectTree,
					mode: EntryModeTree,
					name: "example",
				},
			},
		},
	}

	for _, testCase := range testCases {
		entries, err := ParseTreeEntries([]byte(testCase.Input))
		assert.NoError(t, err)
		assert.EqualValues(t, testCase.Expected, entries)
	}
}
