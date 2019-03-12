// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/sdk/gitea"
)

func getExpectedFileResponse() *gitea.FileResponse {
	return &gitea.FileResponse{
		Content: &gitea.FileContentResponse{
			Name:        "README.md",
			Path:        "README.md",
			SHA:         "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			Size:        30,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/README.md",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
			Type:        "blob",
			Links: &gitea.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/README.md",
			},
		},
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

func TestGetFileResponseFromCommit(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	branch := "master"
	treePath := "README.md"
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commit, _ := gitRepo.GetBranchCommit(branch)
	expectedFileResponse := getExpectedFileResponse()

	fileResponse, err := GetFileResponseFromCommit(repo, commit, branch, treePath)
	assert.Nil(t, err)
	assert.EqualValues(t, expectedFileResponse, fileResponse)
}

// Test errors thrown by GetFileResponseFromCommit
func TestGetFileResponseFromCommitErrors(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	branch := "master"
	treePath := "README.md"

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	commit, err := gitRepo.GetBranchCommit(branch)

	// nil repo
	fileResponse, err := GetFileResponseFromCommit(nil, commit, branch, treePath)
	assert.Nil(t, fileResponse)
	assert.EqualError(t, err, "repo cannot be nil")

	// nil commit
	fileResponse, err = GetFileResponseFromCommit(repo, nil, branch, treePath)
	assert.Nil(t, fileResponse)
	assert.EqualError(t, err, "commit cannot be nil")
}
