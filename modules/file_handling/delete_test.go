// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/sdk/gitea"
)

func getDeleteReportFileOptions() (*DeleteRepoFileOptions) {
	return &DeleteRepoFileOptions{
		LastCommitID: "",
		OldBranch:    "master",
		NewBranch:    "master",
		TreePath:     "README.md",
		Message:      "Deletes README.md",
		SHA:          "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Author:       nil,
		Committer:    nil,
	}
}

func getExpectedDeleteFileResponse() (*gitea.FileResponse) {
	return &gitea.FileResponse{
		Content: nil,
		Commit: &gitea.FileCommitResponse{
			CommitMeta: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
				SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d",
			Author: &gitea.CommitUser{
				Name:  "user1",
				Email: "address1@example.com",
				Date:  "2017-03-19T20:47:59Z",
			},
			Committer: &gitea.CommitUser{
				Name:  "Ethan Koenig",
				Email: "ethantkoenig@gmail.com",
				Date:  "2017-03-19T20:47:59Z",
			},
			Parents: &[]gitea.CommitMeta{},
			Message: "Initial commit\n",
			Tree: &gitea.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/2a2f1d4670728a2e10049e345bd7a276468beab6",
				SHA: "2a2f1d4670728a2e10049e345bd7a276468beab6",
			},
		},
		Verification: &gitea.PayloadCommitVerification{
			Verified:  false,
			Reason:    "",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestDelete(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	opts := getDeleteReportFileOptions()
	expectedFileResponse := getExpectedDeleteFileResponse()
	fileResponse, err := DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, err)
	assert.EqualValues(t, fileResponse, expectedFileResponse)

	// Verify deleted by trying to delete again
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.EqualError(t, err, "object does not exist [id: , rel_path: "+opts.TreePath+"]")
}

// Test opts with branch names removed, same results
func TestDeleteWithoutBranchNames(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	opts := getDeleteReportFileOptions()
	expectedFileResponse := getExpectedDeleteFileResponse()
	opts.OldBranch = ""
	opts.NewBranch = ""
	fileResponse, err := DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.EqualValues(t, fileResponse, expectedFileResponse)
	assert.Nil(t, err)
}

func TestDeleteRepoFileErrors(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)

	// Test bad branch:
	opts := getDeleteReportFileOptions()
	opts.OldBranch = "bad_branch"
	fileResponse, err := DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "branch does not exist [name: "+opts.OldBranch+"]")

	// Test bad SHA:
	opts = getDeleteReportFileOptions()
	origSHA := opts.SHA
	opts.SHA = "bad_sha"
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "file sha does not match ["+opts.SHA+" != "+origSHA+"]")

	// Test new branch already exists:
	opts = getDeleteReportFileOptions()
	opts.NewBranch = "develop"
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "branch already exists [name: "+opts.NewBranch+"]")

	// Test passed repo is nil:
	opts = getDeleteReportFileOptions()
	fileResponse, err = DeleteRepoFile(nil, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "repo not passed to DeleteRepoFile")

	// Test passed doer is nil:
	opts = getDeleteReportFileOptions()
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, nil, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "doer not passed to DeleteRepoFile")

	// Test passed opts is nil:
	opts = getDeleteReportFileOptions()
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, nil)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "opts not passed to DeleteRepoFile")

	// Test treePath is empty:
	opts = getDeleteReportFileOptions()
	opts.TreePath = ""
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "file name is invalid: ")

	// Test treePath is a git directory:
	opts = getDeleteReportFileOptions()
	opts.TreePath = ".git"
	fileResponse, err = DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	assert.EqualError(t, err, "file name is invalid: "+opts.TreePath)
}
