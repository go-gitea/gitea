// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestGetFileContents(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	treePath := "README.md"
	ref := ctx.Repo.Repository.DefaultBranch

	expectedFileContentResponse := &gitea.FileContentResponse{
		Name:        treePath,
		Path:        treePath,
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
	}

	t.Run("Get README.md contents", func(t *testing.T) {
		fileContentResponse, err := GetFileContents(ctx.Repo.Repository, treePath, ref)
		assert.EqualValues(t, expectedFileContentResponse, fileContentResponse)
		assert.Nil(t, err)
	})

	t.Run("Get REAMDE.md contents with ref as empty string (should then use the repo's default branch)", func(t *testing.T) {
		fileContentResponse, err := GetFileContents(ctx.Repo.Repository, treePath, "")
		assert.EqualValues(t, expectedFileContentResponse, fileContentResponse)
		assert.Nil(t, err)
	})
}

func TestGetFileContentsErrors(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	repo := ctx.Repo.Repository
	treePath := "README.md"
	ref := repo.DefaultBranch

	t.Run("bad treePath", func(t *testing.T) {
		badTreePath := "bad/tree.md"
		fileContentResponse, err := GetFileContents(repo, badTreePath, ref)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: , rel_path: bad]")
		assert.Nil(t, fileContentResponse)
	})

	t.Run("bad ref", func(t *testing.T) {
		badRef := "bad_ref"
		fileContentResponse, err := GetFileContents(repo, treePath, badRef)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: "+badRef+", rel_path: ]")
		assert.Nil(t, fileContentResponse)
	})
}
