// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
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

func getExpectedDeleteFileResponse() *api.FileResponse {
	return &api.FileResponse{
		Content: nil,
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d",
				SHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
			},
			HTMLURL: "https://try.gitea.io/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d",
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "user1",
					Email: "address1@example.com",
				},
				Date: "2017-03-19T20:47:59Z",
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Ethan Koenig",
					Email: "ethantkoenig@gmail.com",
				},
				Date: "2017-03-19T20:47:59Z",
			},
			Parents: []*api.CommitMeta{},
			Message: "Initial commit\n",
			Tree: &api.CommitMeta{
				URL: "https://try.gitea.io/api/v1/repos/user2/repo1/git/trees/2a2f1d4670728a2e10049e345bd7a276468beab6",
				SHA: "2a2f1d4670728a2e10049e345bd7a276468beab6",
			},
		},
		Verification: &api.PayloadCommitVerification{
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

	t.Run("Delete README.md file", func(t *testing.T) {
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse()
		assert.EqualValues(t, expectedFileResponse, fileResponse)
	})

	t.Run("Verify README.md has been deleted", func(t *testing.T) {
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		expectedError := "repository file does not exist [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
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

	t.Run("Delete README.md without Branch Name", func(t *testing.T) {
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse()
		assert.EqualValues(t, expectedFileResponse, fileResponse)
	})
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

	t.Run("Bad branch", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.OldBranch = "bad_branch"
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Error(t, err)
		assert.Nil(t, fileResponse)
		expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("Bad SHA", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		origSHA := opts.SHA
		opts.SHA = "bad_sha"
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "sha does not match [given: " + opts.SHA + ", expected: " + origSHA + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("New branch already exists", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.NewBranch = "develop"
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "branch already exists [name: " + opts.NewBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is empty:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.TreePath = ""
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: ]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is a git directory:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.TreePath = ".git"
		fileResponse, err := DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}
