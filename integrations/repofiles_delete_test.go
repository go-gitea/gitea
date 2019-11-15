// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func getDeleteRepoFileOptions(repo *models.Repository) *repofiles.DeleteRepoFileOptions {
	return &repofiles.DeleteRepoFileOptions{
		LastCommitID: "",
		OldBranch:    repo.DefaultBranch,
		NewBranch:    repo.DefaultBranch,
		TreePath:     "README.md",
		Message:      "Deletes README.md",
		SHA:          "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Author: &repofiles.IdentityOptions{
			Name:  "Bob Smith",
			Email: "bob@smith.com",
		},
		Committer: nil,
	}
}

func getExpectedDeleteFileResponse(u *url.URL) *api.FileResponse {
	// Just returns fields that don't change, i.e. fields with commit SHAs and dates can't be determined
	return &api.FileResponse{
		Content: nil,
		Commit: &api.FileCommitResponse{
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Bob Smith",
					Email: "bob@smith.com",
				},
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Bob Smith",
					Email: "bob@smith.com",
				},
			},
			Message: "Deletes README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestDeleteRepoFile(t *testing.T) {
	onGiteaRun(t, testDeleteRepoFile)
}

func testDeleteRepoFile(t *testing.T, u *url.URL) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()
	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getDeleteRepoFileOptions(repo)

	t.Run("Delete README.md file", func(t *testing.T) {
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse(u)
		assert.NotNil(t, fileResponse)
		assert.Nil(t, fileResponse.Content)
		assert.EqualValues(t, expectedFileResponse.Commit.Message, fileResponse.Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, fileResponse.Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, fileResponse.Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, fileResponse.Verification)
	})

	t.Run("Verify README.md has been deleted", func(t *testing.T) {
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		expectedError := "repository file does not exist [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}

// Test opts with branch names removed, same results
func TestDeleteRepoFileWithoutBranchNames(t *testing.T) {
	onGiteaRun(t, testDeleteRepoFileWithoutBranchNames)
}

func testDeleteRepoFileWithoutBranchNames(t *testing.T, u *url.URL) {
	// setup
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	doer := ctx.User
	opts := getDeleteRepoFileOptions(repo)
	opts.OldBranch = ""
	opts.NewBranch = ""

	t.Run("Delete README.md without Branch Name", func(t *testing.T) {
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse(u)
		assert.NotNil(t, fileResponse)
		assert.Nil(t, fileResponse.Content)
		assert.EqualValues(t, expectedFileResponse.Commit.Message, fileResponse.Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, fileResponse.Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, fileResponse.Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, fileResponse.Verification)
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
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	doer := ctx.User

	t.Run("Bad branch", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.OldBranch = "bad_branch"
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Error(t, err)
		assert.Nil(t, fileResponse)
		expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("Bad SHA", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		origSHA := opts.SHA
		opts.SHA = "bad_sha"
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "sha does not match [given: " + opts.SHA + ", expected: " + origSHA + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("New branch already exists", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.NewBranch = "develop"
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "branch already exists [name: " + opts.NewBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is empty:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.TreePath = ""
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: ]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is a git directory:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.TreePath = ".git"
		fileResponse, err := repofiles.DeleteRepoFile(repo, doer, opts)
		assert.Nil(t, fileResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: " + opts.TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}
