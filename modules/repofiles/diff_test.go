// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
)

func TestGetDiffPreview(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	expectedDiff := &models.Diff{
		TotalAddition: 2,
		TotalDeletion: 1,
		Files: []*models.DiffFile{
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
				Sections: []*models.DiffSection{
					{
						Name: "",
						Lines: []*models.DiffLine{
							{
								LeftIdx:  0,
								RightIdx: 0,
								Type:     4,
								Content:  "@@ -1,3 +1,4 @@",
								Comments: nil,
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

	// Test with given branch
	diff, err := GetDiffPreview(ctx.Repo.Repository, branch, treePath, content)
	assert.Nil(t, err)
	assert.EqualValues(t, expectedDiff, diff)

	// Test empty branch, same results
	diff, err = GetDiffPreview(ctx.Repo.Repository, "", treePath, content)
	assert.Nil(t, err)
	assert.EqualValues(t, expectedDiff, diff)
}

func TestGetDiffPreviewErrors(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	branch := ctx.Repo.Repository.DefaultBranch
	treePath := "README.md"
	content := "# repo1\n\nDescription for repo1\nthis is a new line"

	// Test nil repo
	diff, err := GetDiffPreview(nil, branch, treePath, content)
	assert.Nil(t, diff)
	assert.EqualError(t, err, "repository cannot be nil")

	// Test empty repo
	diff, err = GetDiffPreview(&models.Repository{}, branch, treePath, content)
	assert.Nil(t, diff)
	assert.EqualError(t, err, "repository does not exist [id: 0, uid: 0, owner_name: , name: ]")

	// Test bad branch
	badBranch := "bad_branch"
	diff, err = GetDiffPreview(ctx.Repo.Repository, badBranch, treePath, content)
	assert.Nil(t, diff)
	assert.EqualError(t, err, "branch does not exist [name: "+badBranch+"]")

	// Test empty treePath
	diff, err = GetDiffPreview(ctx.Repo.Repository, branch, "", content)
	assert.Nil(t, diff)
	assert.EqualError(t, err, "path is invalid [path: ]")
}
