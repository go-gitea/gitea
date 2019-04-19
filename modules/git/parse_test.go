// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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
					ID: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					gogitTreeEntry: &object.TreeEntry{
						Hash: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
						Name: "example/file2.txt",
						Mode: filemode.Regular,
					},
				},
			},
		},
		{
			Input: "120000 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c\t\"example/\\n.txt\"\n" +
				"040000 tree 1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8\texample\n",
			Expected: []*TreeEntry{
				{
					ID: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					gogitTreeEntry: &object.TreeEntry{
						Hash: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
						Name: "example/\n.txt",
						Mode: filemode.Symlink,
					},
				},
				{
					ID: MustIDFromString("1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8"),
					gogitTreeEntry: &object.TreeEntry{
						Hash: MustIDFromString("1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8"),
						Name: "example",
						Mode: filemode.Dir,
					},
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
