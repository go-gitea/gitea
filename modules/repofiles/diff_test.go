// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestGetDiffPreview(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
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
						Name: "",
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
								Type:     3,
								Content:  "-Description for repo1",
								Comments: nil,
							},
							{
								LeftIdx:  0,
								RightIdx: 3,
								Type:     2,
								Content:  "+Description for repo1",
								Comments: nil,
							},
							{
								LeftIdx:  0,
								RightIdx: 4,
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

	t.Run("with given branch", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx.Repo.Repository, branch, treePath, content)
		assert.Nil(t, err)
		assert.EqualValues(t, expectedDiff, diff)
	})

	t.Run("empty branch, same results", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx.Repo.Repository, "", treePath, content)
		assert.Nil(t, err)
		assert.EqualValues(t, expectedDiff, diff)
	})
}

func TestGetDiffPreviewErrors(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	t.Run("empty repo", func(t *testing.T) {
		diff, err := GetDiffPreview(&models.Repository{}, branch, treePath, content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "repository does not exist [id: 0, uid: 0, owner_name: , name: ]")
	})

	t.Run("bad branch", func(t *testing.T) {
		badBranch := "bad_branch"
		diff, err := GetDiffPreview(ctx.Repo.Repository, badBranch, treePath, content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "branch does not exist [name: "+badBranch+"]")
	})

	t.Run("empty treePath", func(t *testing.T) {
		diff, err := GetDiffPreview(ctx.Repo.Repository, branch, "", content)
		assert.Nil(t, diff)
		assert.EqualError(t, err, "path is invalid [path: ]")
	})
}
