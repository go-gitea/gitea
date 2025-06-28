// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"fmt"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
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
			Input: "100644 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c    1022\texample/file2.txt\n",
			Expected: []*TreeEntry{
				{
					ID: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					gogitTreeEntry: &object.TreeEntry{
						Hash: plumbing.Hash(MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c").RawValue()),
						Name: "example/file2.txt",
						Mode: filemode.Regular,
					},
					size:  1022,
					sized: true,
				},
			},
		},
		{
			Input: "120000 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c  234131\t\"example/\\n.txt\"\n" +
				"040000 tree 1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8       -\texample\n",
			Expected: []*TreeEntry{
				{
					ID: MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c"),
					gogitTreeEntry: &object.TreeEntry{
						Hash: plumbing.Hash(MustIDFromString("61ab7345a1a3bbc590068ccae37b8515cfc5843c").RawValue()),
						Name: "example/\n.txt",
						Mode: filemode.Symlink,
					},
					size:  234131,
					sized: true,
				},
				{
					ID:    MustIDFromString("1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8"),
					sized: true,
					gogitTreeEntry: &object.TreeEntry{
						Hash: plumbing.Hash(MustIDFromString("1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8").RawValue()),
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
		if len(entries) > 1 {
			fmt.Println(testCase.Expected[0].ID)
			fmt.Println(entries[0].ID)
		}
		assert.EqualValues(t, testCase.Expected, entries)
	}
}
