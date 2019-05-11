// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func getExpectedFileResponse() *api.FileResponse {
	return &api.FileResponse{
		Content: &api.FileContentResponse{
			Name:        "README.md",
			Path:        "README.md",
			SHA:         "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			Size:        30,
			URL:         "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
			HTMLURL:     "https://try.gitea.io/user2/repo1/blob/master/README.md",
			GitURL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			DownloadURL: "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
			Type:        "blob",
			Links: &api.FileLinksResponse{
				Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md",
				GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
				HTMLURL: "https://try.gitea.io/user2/repo1/blob/master/README.md",
			},
		},
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

func TestGetFileResponseFromCommit(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	branch := repo.DefaultBranch
	treePath := "README.md"
	gitRepo, _ := git.OpenRepository(repo.RepoPath())
	commit, _ := gitRepo.GetBranchCommit(branch)
	expectedFileResponse := getExpectedFileResponse()

	fileResponse, err := GetFileResponseFromCommit(repo, commit, branch, treePath)
	assert.Nil(t, err)
	assert.EqualValues(t, expectedFileResponse, fileResponse)
}
