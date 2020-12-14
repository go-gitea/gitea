// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"

	"github.com/stretchr/testify/assert"
)

func TestParseTreeEntries(t *testing.T) {
	type TreeEntryValue struct {
		ID   string
		Name string
		Mode service.EntryMode
	}

	testCases := []struct {
		Input    string
		Expected []TreeEntryValue
	}{
		{
			Input:    "",
			Expected: []TreeEntryValue{},
		},
		{
			Input: "100644 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c\texample/file2.txt\n",
			Expected: []TreeEntryValue{
				{
					ID:   "61ab7345a1a3bbc590068ccae37b8515cfc5843c",
					Name: "example/file2.txt",
					Mode: service.EntryModeBlob,
				},
			},
		},
		{
			Input: "120000 blob 61ab7345a1a3bbc590068ccae37b8515cfc5843c\t\"example/\\n.txt\"\n" +
				"040000 tree 1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8\texample\n",
			Expected: []TreeEntryValue{
				{
					ID:   "61ab7345a1a3bbc590068ccae37b8515cfc5843c",
					Name: "example/\n.txt",
					Mode: service.EntryModeSymlink,
				},
				{
					ID:   "1d01fb729fb0db5881daaa6030f9f2d3cd3d5ae8",
					Name: "example",
					Mode: service.EntryModeTree,
				},
			},
		},
	}

	for _, testCase := range testCases {
		entries, err := native.ParseTreeEntries([]byte(testCase.Input))
		assert.NoError(t, err)
		for i, entry := range entries {
			assert.EqualValues(t, testCase.Expected[i].ID, entry.ID().String())
			assert.EqualValues(t, testCase.Expected[i].Name, entry.Name())
			assert.EqualValues(t, testCase.Expected[i].Mode, entry.Mode().String())
		}
	}
}
