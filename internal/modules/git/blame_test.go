// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadingBlameOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Without .git-blame-ignore-revs", func(t *testing.T) {
		repo, err := OpenRepository(ctx, "./tests/repos/repo5_pulls")
		assert.NoError(t, err)
		defer repo.Close()

		commit, err := repo.GetCommit("f32b0a9dfd09a60f616f29158f772cedd89942d2")
		assert.NoError(t, err)

		parts := []*BlamePart{
			{
				"72866af952e98d02a73003501836074b286a78f6",
				[]string{
					"# test_repo",
					"Test repository for testing migration from github to gitea",
				},
			},
			{
				"f32b0a9dfd09a60f616f29158f772cedd89942d2",
				[]string{"", "Do not make any changes to this repo it is used for unit testing"},
			},
		}

		for _, bypass := range []bool{false, true} {
			blameReader, err := CreateBlameReader(ctx, "./tests/repos/repo5_pulls", commit, "README.md", bypass)
			assert.NoError(t, err)
			assert.NotNil(t, blameReader)
			defer blameReader.Close()

			assert.False(t, blameReader.UsesIgnoreRevs())

			for _, part := range parts {
				actualPart, err := blameReader.NextPart()
				assert.NoError(t, err)
				assert.Equal(t, part, actualPart)
			}

			// make sure all parts have been read
			actualPart, err := blameReader.NextPart()
			assert.Nil(t, actualPart)
			assert.NoError(t, err)
		}
	})

	t.Run("With .git-blame-ignore-revs", func(t *testing.T) {
		repo, err := OpenRepository(ctx, "./tests/repos/repo6_blame")
		assert.NoError(t, err)
		defer repo.Close()

		full := []*BlamePart{
			{
				"af7486bd54cfc39eea97207ca666aa69c9d6df93",
				[]string{"line", "line"},
			},
			{
				"45fb6cbc12f970b04eacd5cd4165edd11c8d7376",
				[]string{"changed line"},
			},
			{
				"af7486bd54cfc39eea97207ca666aa69c9d6df93",
				[]string{"line", "line", ""},
			},
		}

		cases := []struct {
			CommitID       string
			UsesIgnoreRevs bool
			Bypass         bool
			Parts          []*BlamePart
		}{
			{
				CommitID:       "544d8f7a3b15927cddf2299b4b562d6ebd71b6a7",
				UsesIgnoreRevs: true,
				Bypass:         false,
				Parts: []*BlamePart{
					{
						"af7486bd54cfc39eea97207ca666aa69c9d6df93",
						[]string{"line", "line", "changed line", "line", "line", ""},
					},
				},
			},
			{
				CommitID:       "544d8f7a3b15927cddf2299b4b562d6ebd71b6a7",
				UsesIgnoreRevs: false,
				Bypass:         true,
				Parts:          full,
			},
			{
				CommitID:       "45fb6cbc12f970b04eacd5cd4165edd11c8d7376",
				UsesIgnoreRevs: false,
				Bypass:         false,
				Parts:          full,
			},
			{
				CommitID:       "45fb6cbc12f970b04eacd5cd4165edd11c8d7376",
				UsesIgnoreRevs: false,
				Bypass:         false,
				Parts:          full,
			},
		}

		for _, c := range cases {
			commit, err := repo.GetCommit(c.CommitID)
			assert.NoError(t, err)

			blameReader, err := CreateBlameReader(ctx, "./tests/repos/repo6_blame", commit, "blame.txt", c.Bypass)
			assert.NoError(t, err)
			assert.NotNil(t, blameReader)
			defer blameReader.Close()

			assert.Equal(t, c.UsesIgnoreRevs, blameReader.UsesIgnoreRevs())

			for _, part := range c.Parts {
				actualPart, err := blameReader.NextPart()
				assert.NoError(t, err)
				assert.Equal(t, part, actualPart)
			}

			// make sure all parts have been read
			actualPart, err := blameReader.NextPart()
			assert.Nil(t, actualPart)
			assert.NoError(t, err)
		}
	})
}
