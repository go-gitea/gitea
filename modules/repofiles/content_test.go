// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func getExpectedReadmeFileContentsResponse() *api.FileContentsResponse {
	return &api.FileContentsResponse{
		Name:            "README.md",
		Path:            "README.md",
		SHA:             "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Type:            "file",
		Size:            30,
		Encoding:        "base64",
		Content:         "IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x",
		URL:             "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md?ref=master",
		HTMLURL:         "https://try.gitea.io/user2/repo1/src/branch/master/README.md",
		GitURL:          "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		DownloadURL:     "https://try.gitea.io/user2/repo1/raw/branch/master/README.md",
		SubmoduleGitURL: "",
		Links: &api.FileLinksResponse{
			Self:    "https://try.gitea.io/api/v1/repos/user2/repo1/contents/README.md?ref=master",
			GitURL:  "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			HTMLURL: "https://try.gitea.io/user2/repo1/src/branch/master/README.md",
		},
	}
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

	expectedFileContentsResponse := getExpectedReadmeFileContentsResponse()

	t.Run("Get README.md contents with GetFileContents()", func(t *testing.T) {
		fileContentResponse, err := GetFileContents(ctx.Repo.Repository, treePath, ref, false)
		assert.EqualValues(t, expectedFileContentsResponse, fileContentResponse)
		assert.Nil(t, err)
	})

	t.Run("Get REAMDE.md contents with ref as empty string (should then use the repo's default branch) with GetFileContents()", func(t *testing.T) {
		fileContentResponse, err := GetFileContents(ctx.Repo.Repository, treePath, "", false)
		assert.EqualValues(t, expectedFileContentsResponse, fileContentResponse)
		assert.Nil(t, err)
	})
}

func TestGetFileContentsOrListForDir(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	treePath := "" // root dir
	ref := ctx.Repo.Repository.DefaultBranch

	readmeFileContentsResponse := getExpectedReadmeFileContentsResponse()
	// because will be in a list, doesn't have encoding and content
	readmeFileContentsResponse.Encoding = ""
	readmeFileContentsResponse.Content = ""

	expectedFileContentsListResponse := []*api.FileContentsResponse{
		readmeFileContentsResponse,
	}

	t.Run("Get root dir contents with GetFileContentsOrList()", func(t *testing.T) {
		fileContentResponse, err := GetFileContentsOrList(ctx.Repo.Repository, treePath, ref)
		assert.EqualValues(t, expectedFileContentsListResponse, fileContentResponse)
		assert.Nil(t, err)
	})

	t.Run("Get root dir contents with ref as empty string (should then use the repo's default branch) with GetFileContentsOrList()", func(t *testing.T) {
		fileContentResponse, err := GetFileContentsOrList(ctx.Repo.Repository, treePath, "")
		assert.EqualValues(t, expectedFileContentsListResponse, fileContentResponse)
		assert.Nil(t, err)
	})
}

func TestGetFileContentsOrListForFile(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	treePath := "README.md"
	ref := ctx.Repo.Repository.DefaultBranch

	expectedFileContentsResponse := getExpectedReadmeFileContentsResponse()

	t.Run("Get README.md contents with GetFileContentsOrList()", func(t *testing.T) {
		fileContentResponse, err := GetFileContentsOrList(ctx.Repo.Repository, treePath, ref)
		assert.EqualValues(t, expectedFileContentsResponse, fileContentResponse)
		assert.Nil(t, err)
	})

	t.Run("Get REAMDE.md contents with ref as empty string (should then use the repo's default branch) with GetFileContentsOrList()", func(t *testing.T) {
		fileContentResponse, err := GetFileContentsOrList(ctx.Repo.Repository, treePath, "")
		assert.EqualValues(t, expectedFileContentsResponse, fileContentResponse)
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
		fileContentResponse, err := GetFileContents(repo, badTreePath, ref, false)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: , rel_path: bad]")
		assert.Nil(t, fileContentResponse)
	})

	t.Run("bad ref", func(t *testing.T) {
		badRef := "bad_ref"
		fileContentResponse, err := GetFileContents(repo, treePath, badRef, false)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: "+badRef+", rel_path: ]")
		assert.Nil(t, fileContentResponse)
	})
}

func TestGetFileContentsOrListErrors(t *testing.T) {
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
		fileContentResponse, err := GetFileContentsOrList(repo, badTreePath, ref)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: , rel_path: bad]")
		assert.Nil(t, fileContentResponse)
	})

	t.Run("bad ref", func(t *testing.T) {
		badRef := "bad_ref"
		fileContentResponse, err := GetFileContentsOrList(repo, treePath, badRef)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: "+badRef+", rel_path: ]")
		assert.Nil(t, fileContentResponse)
	})
}
