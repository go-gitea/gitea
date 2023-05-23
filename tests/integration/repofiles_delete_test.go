// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

/*
import (
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func getDeleteRepoFileOptions(repo *repo_model.Repository) *files_service.DeleteRepoFilesOptions {
	return &files_service.DeleteRepoFilesOptions{
		Files: []*files_service.DeleteRepoFile{
			{
				TreePath: "README.md",
				SHA:      "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			},
		},
		LastCommitID: "",
		OldBranch:    repo.DefaultBranch,
		NewBranch:    repo.DefaultBranch,
		Message:      "Deletes README.md",
		Author: &files_service.IdentityOptions{
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
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()
	repo := ctx.Repo.Repository
	doer := ctx.Doer
	opts := getDeleteRepoFileOptions(repo)

	t.Run("Delete README.md file", func(t *testing.T) {
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.NoError(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse(u)
		assert.NotNil(t, filesResponse)
		assert.Nil(t, filesResponse[0].Content)
		assert.EqualValues(t, expectedFileResponse.Commit.Message, filesResponse[0].Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, filesResponse[0].Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, filesResponse[0].Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, filesResponse[0].Verification)
	})

	t.Run("Verify README.md has been deleted", func(t *testing.T) {
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		expectedError := "repository file does not exist [path: " + opts.Files[0].TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}

// Test opts with branch names removed, same results
func TestDeleteRepoFileWithoutBranchNames(t *testing.T) {
	onGiteaRun(t, testDeleteRepoFileWithoutBranchNames)
}

func testDeleteRepoFileWithoutBranchNames(t *testing.T, u *url.URL) {
	// setup
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	doer := ctx.Doer
	opts := getDeleteRepoFileOptions(repo)
	opts.OldBranch = ""
	opts.NewBranch = ""

	t.Run("Delete README.md without Branch Name", func(t *testing.T) {
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.NoError(t, err)
		expectedFileResponse := getExpectedDeleteFileResponse(u)
		assert.NotNil(t, filesResponse)
		assert.Nil(t, filesResponse[0].Content)
		assert.EqualValues(t, expectedFileResponse.Commit.Message, filesResponse[0].Commit.Message)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Identity, filesResponse[0].Commit.Author.Identity)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Identity, filesResponse[0].Commit.Committer.Identity)
		assert.EqualValues(t, expectedFileResponse.Verification, filesResponse[0].Verification)
	})
}

func TestDeleteRepoFileErrors(t *testing.T) {
	// setup
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	doer := ctx.Doer

	t.Run("Bad branch", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.OldBranch = "bad_branch"
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Error(t, err)
		assert.Nil(t, filesResponse)
		expectedError := "branch does not exist [name: " + opts.OldBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("Bad SHA", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		origSHA := opts.Files[0].SHA
		opts.Files[0].SHA = "bad_sha"
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		assert.Error(t, err)
		expectedError := "sha does not match [given: " + opts.Files[0].SHA + ", expected: " + origSHA + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("New branch already exists", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.NewBranch = "develop"
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		assert.Error(t, err)
		expectedError := "branch already exists [name: " + opts.NewBranch + "]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is empty:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.Files[0].TreePath = ""
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: ]"
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TreePath is a git directory:", func(t *testing.T) {
		opts := getDeleteRepoFileOptions(repo)
		opts.Files[0].TreePath = ".git"
		filesResponse, err := files_service.DeleteRepoFiles(git.DefaultContext, repo, doer, opts)
		assert.Nil(t, filesResponse)
		assert.Error(t, err)
		expectedError := "path contains a malformed path component [path: " + opts.Files[0].TreePath + "]"
		assert.EqualError(t, err, expectedError)
	})
}
*/
