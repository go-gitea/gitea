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

func getDeleteRepoFileOptions(repo *models.Repository) *DeleteRepoFileOptions {
	return &DeleteRepoFileOptions{
		LastCommitID: "",
		OldBranch:    repo.DefaultBranch,
		NewBranch:    repo.DefaultBranch,
		TreePath:     "README.md",
		Message:      "Deletes README.md",
		SHA:          "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Author:       nil,
		Committer:    nil,
	}
}

func getExpectedDeleteFileResponse() *gitea.FileResponse {
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
			Parents: []*gitea.CommitMeta{},
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

func TestDeleteRepoFile(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getDeleteRepoFileOptions(repo)

	// Test #1 - Delete the README.md file
	// actual test
	fileResponse, err := DeleteRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	expectedFileResponse := getExpectedDeleteFileResponse()
	assert.EqualValues(t, expectedFileResponse, fileResponse)

	// Test #2 - Verify deleted by trying to delete again
	fileResponse, err = DeleteRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	expectedError := "object does not exist [id: , rel_path: " + opts.TreePath + "]"
	assert.EqualError(t, err, expectedError)
}

// Test opts with branch names removed, same results
func TestDeleteRepoFileWithoutBranchNames(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getDeleteRepoFileOptions(repo)
	opts.OldBranch = ""
	opts.NewBranch = ""

	// Test #1 - Delete README.md file
	fileResponse, err := DeleteRepoFile(repo, doer, opts)

	// asserts
	assert.Nil(t, err)
	expectedFileResponse := getExpectedDeleteFileResponse()
	assert.EqualValues(t, expectedFileResponse, fileResponse)
}

func TestDeleteRepoFileErrors(t *testing.T) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	doer := ctx.User

	// Test #1 - bad branch
	opts := getDeleteRepoFileOptions(repo)
	opts.OldBranch = "bad_branch"
	fileResponse, err := DeleteRepoFile(repo, doer, opts)
	assert.Error(t, err)
	assert.Nil(t, fileResponse)
	expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
	assert.EqualError(t, err, expectedError)

	// Test #2 - bad SHA
	opts = getDeleteRepoFileOptions(repo)
	origSHA := opts.SHA
	opts.SHA = "bad_sha"
	fileResponse, err = DeleteRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file sha does not match [given: " + opts.SHA + ", expected: " + origSHA + "]"
	assert.EqualError(t, err, expectedError)

	// Test #3 - new branch already exists
	opts = getDeleteRepoFileOptions(repo)
	opts.NewBranch = "develop"
	fileResponse, err = DeleteRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "branch already exists [name: " + opts.NewBranch + "]"
	assert.EqualError(t, err, expectedError)

	// Test #4 - repo is nil
	opts = getDeleteRepoFileOptions(repo)
	fileResponse, err = DeleteRepoFile(nil, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "repo cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #5 - doer is nil
	opts = getDeleteRepoFileOptions(repo)
	fileResponse, err = DeleteRepoFile(repo, nil, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "doer cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #6 - opts is nil:
	opts = getDeleteRepoFileOptions(repo)
	fileResponse, err = DeleteRepoFile(repo, doer, nil)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "opts cannot be nil"
	assert.EqualError(t, err, expectedError)

	// Test #7 - treePath is empty:
	opts = getDeleteRepoFileOptions(repo)
	opts.TreePath = ""
	fileResponse, err = DeleteRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: "
	assert.EqualError(t, err, expectedError)

	// Test #8 - treePath is a git directory:
	opts = getDeleteRepoFileOptions(repo)
	opts.TreePath = ".git"
	fileResponse, err = DeleteRepoFile(repo, doer, opts)
	assert.Nil(t, fileResponse)
	assert.Error(t, err)
	expectedError = "file name is invalid: " + opts.TreePath
	assert.EqualError(t, err, expectedError)
}
