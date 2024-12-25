// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestGetDiffPreview(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetPathParam("id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	expectedDiff := &gitdiff.Diff{
		TotalAddition: 2,
		TotalDeletion: 1,
		Files: []*gitdiff.DiffFile{
			{
				Name:        "README.md",
				OldName:     "README.md",
				NameHash:    "8ec9a00bfd09b3190ac6b22251dbb1aa95a0579d",
				Index:       1,
				Addition:    2,
				Deletion:    1,
				Type:        2,
				IsCreated:   false,
				IsDeleted:   false,
				IsBin:       false,
				IsLFSFile:   false,
				IsRenamed:   false,
				IsSubmodule: false,
				Sections: []*gitdiff.DiffSection{
					{
						FileName: "README.md",
						Name:     "",
						Lines: []*gitdiff.DiffLine{
							{
								LeftIdx:  0,
								RightIdx: 0,
								Type:     4,
								Content:  "@@ -1,3 +1,4 @@",
								Comments: nil,
								SectionInfo: &gitdiff.DiffLineSectionInfo{
									Path:          "README.md",
									LastLeftIdx:   0,
									LastRightIdx:  0,
									LeftIdx:       1,
									RightIdx:      1,
									LeftHunkSize:  3,
									RightHunkSize: 4,
								},
							},
							{
								LeftIdx:  1,
								RightIdx: 1,
								Type:     1,
								Content:  " # repo1",
								Comments: nil,
							},
							{
								LeftIdx:  2,
								RightIdx: 2,
								Type:     1,
								Content:  " ",
								Comments: nil,
							},
							{
								LeftIdx:  3,
								RightIdx: 0,
								Match:    4,
								Type:     3,
								Content:  "-Description for repo1",
								Comments: nil,
							},
							{
								LeftIdx:  0,
								RightIdx: 3,
								Match:    3,
								Type:     2,
								Content:  "+Description for repo1",
								Comments: nil,
							},
							{
								LeftIdx:  0,
								RightIdx: 4,
								Match:    -1,
								Type:     2,
								Content:  "+this is a new line",
								Comments: nil,
							},
						},
					},
				},
				IsIncomplete: false,
			},
		},
		IsIncomplete: false,
	}
	expectedDiff.NumFiles = len(expectedDiff.Files)

	t.Run("with given branch", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, branch, treePath, content)
		assert.NoError(t, err)
		expectedBs, err := json.Marshal(expectedDiff)
		assert.NoError(t, err)
		bs, err := json.Marshal(diff)
		assert.NoError(t, err)
		assert.EqualValues(t, string(expectedBs), string(bs))
	})

	t.Run("empty branch, same results", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, "", treePath, content)
		assert.NoError(t, err)
		expectedBs, err := json.Marshal(expectedDiff)
		assert.NoError(t, err)
		bs, err := json.Marshal(diff)
		assert.NoError(t, err)
		assert.EqualValues(t, expectedBs, bs)
	})
}

func TestGetDiffPreviewErrors(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetPathParam("id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	t.Run("empty repo", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx, &repo_model.Repository{}, branch, treePath, content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "repository does not exist [id: 0, uid: 0, owner_name: , name: ]")
	})

	t.Run("bad branch", func(t *testing.T) {
		badBranch := "bad_branch"
		diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, badBranch, treePath, content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "branch does not exist [name: "+badBranch+"]")
	})

	t.Run("empty treePath", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx, ctx.Repo.Repository, branch, "", content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "path is invalid [path: ]")
	})
}
