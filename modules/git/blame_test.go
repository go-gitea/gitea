// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestLineBlameCommitID(t *testing.T) {
	storage := mockRepository("repo5_pulls")
	const revision = "f32b0a9dfd09a60f616f29158f772cedd89942d2"
	commitID, err := LineBlameCommitID(t.Context(), storage, revision, "README.md", 1)
	assert.NoError(t, err)
	assert.Equal(t, "72866af952e98d02a73003501836074b286a78f6", commitID)

	commitID, err = LineBlameCommitID(t.Context(), storage, revision, "missing.txt", 1)
	assert.Error(t, err)
	assert.Empty(t, commitID)
}

func TestReadingBlameOutput(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		repositorySuffix string
		objectFormat     ObjectFormat
		readmeCommit     string
		readmePrevious   string
		blameCommit      string
		blameChanged     string
		blameOriginal    string
	}{
		{
			name:           "sha1",
			objectFormat:   Sha1ObjectFormat,
			readmeCommit:   "f32b0a9dfd09a60f616f29158f772cedd89942d2",
			readmePrevious: "72866af952e98d02a73003501836074b286a78f6",
			blameCommit:    "544d8f7a3b15927cddf2299b4b562d6ebd71b6a7",
			blameChanged:   "45fb6cbc12f970b04eacd5cd4165edd11c8d7376",
			blameOriginal:  "af7486bd54cfc39eea97207ca666aa69c9d6df93",
		},
		{
			name:             "sha256",
			repositorySuffix: "_sha256",
			objectFormat:     Sha256ObjectFormat,
			readmeCommit:     "0b69b7bb649b5d46e14cabb6468685e5dd721290acc7ffe604d37cde57927345",
			readmePrevious:   "1e35a51dc00fd7de730344c07061acfe80e8117e075ac979b6a29a3a045190ca",
			blameCommit:      "e2f5660e15159082902960af0ed74fc144921d2b0c80e069361853b3ece29ba3",
			blameChanged:     "9347b0198cd1f25017579b79d0938fa89dba34ad2514f0dd92f6bc975ed1a2fe",
			blameOriginal:    "ab2b57a4fa476fb2edb74dafa577caf918561abbaa8fba0c8dc63c412e17a7cc",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			setting.AppDataPath = t.TempDir()
			ctx := t.Context()

			t.Run("Without .git-blame-ignore-revs", func(t *testing.T) {
				storage := mockRepository("repo5_pulls" + scenario.repositorySuffix)
				repo, err := OpenRepository(ctx, storage)
				assert.NoError(t, err)
				defer repo.Close()
				commit, err := repo.GetCommit(t.Context(), scenario.readmeCommit)
				assert.NoError(t, err)

				parts := []*BlamePart{
					{
						Sha: scenario.readmePrevious,
						Lines: []string{
							"# test_repo",
							"Test repository for testing migration from github to gitea",
						},
					},
					{
						Sha:          scenario.readmeCommit,
						Lines:        []string{"", "Do not make any changes to this repo it is used for unit testing"},
						PreviousSha:  scenario.readmePrevious,
						PreviousPath: "README.md",
					},
				}

				for _, bypass := range []bool{false, true} {
					blameReader, err := CreateBlameReader(ctx, scenario.objectFormat, storage, repo, commit, "README.md", bypass)
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
				storage := mockRepository("repo6_blame" + scenario.repositorySuffix)
				repo, err := OpenRepository(ctx, storage)
				assert.NoError(t, err)
				defer repo.Close()

				full := []*BlamePart{
					{
						Sha:   scenario.blameOriginal,
						Lines: []string{"line", "line"},
					},
					{
						Sha:          scenario.blameChanged,
						Lines:        []string{"changed line"},
						PreviousSha:  scenario.blameOriginal,
						PreviousPath: "blame.txt",
					},
					{
						Sha:   scenario.blameOriginal,
						Lines: []string{"line", "line", ""},
					},
				}

				cases := []struct {
					CommitID       string
					UsesIgnoreRevs bool
					Bypass         bool
					Parts          []*BlamePart
				}{
					{
						CommitID:       scenario.blameCommit,
						UsesIgnoreRevs: true,
						Bypass:         false,
						Parts: []*BlamePart{
							{
								Sha:   scenario.blameOriginal,
								Lines: []string{"line", "line", "changed line", "line", "line", ""},
							},
						},
					},
					{
						CommitID:       scenario.blameCommit,
						UsesIgnoreRevs: false,
						Bypass:         true,
						Parts:          full,
					},
					{
						CommitID:       scenario.blameChanged,
						UsesIgnoreRevs: false,
						Bypass:         false,
						Parts:          full,
					},
					{
						CommitID:       scenario.blameChanged,
						UsesIgnoreRevs: false,
						Bypass:         false,
						Parts:          full,
					},
				}

				objectFormat, err := repo.GetObjectFormat(t.Context())
				assert.NoError(t, err)
				for _, c := range cases {
					commit, err := repo.GetCommit(t.Context(), c.CommitID)
					assert.NoError(t, err)

					blameReader, err := CreateBlameReader(ctx, objectFormat, storage, repo, commit, "blame.txt", c.Bypass)
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
		})
	}
}
