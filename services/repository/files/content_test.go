// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package files

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", "..", ".."))
}

func getExpectedReadmeContentsResponse() *api.ContentsResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	encoding := "base64"
	content := "IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x"
	selfURL := "https://try.gitea.io/api/v1/repos/user2/repo1/contents/" + treePath + "?ref=master"
	htmlURL := "https://try.gitea.io/user2/repo1/src/branch/master/" + treePath
	gitURL := "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := "https://try.gitea.io/user2/repo1/raw/branch/master/" + treePath
	return &api.ContentsResponse{
		Name:        treePath,
		Path:        treePath,
		SHA:         "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		Type:        "file",
		Size:        30,
		Encoding:    &encoding,
		Content:     &content,
		URL:         &selfURL,
		HTMLURL:     &htmlURL,
		GitURL:      &gitURL,
		DownloadURL: &downloadURL,
		Links: &api.FileLinksResponse{
			Self:    &selfURL,
			GitURL:  &gitURL,
			HTMLURL: &htmlURL,
		},
	}
}

func TestGetContents(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	treePath := "README.md"
	ref := ctx.Repo.Repository.DefaultBranch

	expectedContentsResponse := getExpectedReadmeContentsResponse()

	t.Run("Get README.md contents with GetContents(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContents(ctx, ctx.Repo.Repository, treePath, ref, false)
		assert.EqualValues(t, expectedContentsResponse, fileContentResponse)
		assert.NoError(t, err)
	})

	t.Run("Get README.md contents with ref as empty string (should then use the repo's default branch) with GetContents(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContents(ctx, ctx.Repo.Repository, treePath, "", false)
		assert.EqualValues(t, expectedContentsResponse, fileContentResponse)
		assert.NoError(t, err)
	})
}

func TestGetContentsOrListForDir(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	treePath := "" // root dir
	ref := ctx.Repo.Repository.DefaultBranch

	readmeContentsResponse := getExpectedReadmeContentsResponse()
	// because will be in a list, doesn't have encoding and content
	readmeContentsResponse.Encoding = nil
	readmeContentsResponse.Content = nil

	expectedContentsListResponse := []*api.ContentsResponse{
		readmeContentsResponse,
	}

	t.Run("Get root dir contents with GetContentsOrList(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContentsOrList(ctx, ctx.Repo.Repository, treePath, ref)
		assert.EqualValues(t, expectedContentsListResponse, fileContentResponse)
		assert.NoError(t, err)
	})

	t.Run("Get root dir contents with ref as empty string (should then use the repo's default branch) with GetContentsOrList(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContentsOrList(ctx, ctx.Repo.Repository, treePath, "")
		assert.EqualValues(t, expectedContentsListResponse, fileContentResponse)
		assert.NoError(t, err)
	})
}

func TestGetContentsOrListForFile(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	treePath := "README.md"
	ref := ctx.Repo.Repository.DefaultBranch

	expectedContentsResponse := getExpectedReadmeContentsResponse()

	t.Run("Get README.md contents with GetContentsOrList(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContentsOrList(ctx, ctx.Repo.Repository, treePath, ref)
		assert.EqualValues(t, expectedContentsResponse, fileContentResponse)
		assert.NoError(t, err)
	})

	t.Run("Get README.md contents with ref as empty string (should then use the repo's default branch) with GetContentsOrList(ctx, )", func(t *testing.T) {
		fileContentResponse, err := GetContentsOrList(ctx, ctx.Repo.Repository, treePath, "")
		assert.EqualValues(t, expectedContentsResponse, fileContentResponse)
		assert.NoError(t, err)
	})
}

func TestGetContentsErrors(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	treePath := "README.md"
	ref := repo.DefaultBranch

	t.Run("bad treePath", func(t *testing.T) {
		badTreePath := "bad/tree.md"
		fileContentResponse, err := GetContents(ctx, repo, badTreePath, ref, false)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: , rel_path: bad]")
		assert.Nil(t, fileContentResponse)
	})

	t.Run("bad ref", func(t *testing.T) {
		badRef := "bad_ref"
		fileContentResponse, err := GetContents(ctx, repo, treePath, badRef, false)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: "+badRef+", rel_path: ]")
		assert.Nil(t, fileContentResponse)
	})
}

func TestGetContentsOrListErrors(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	treePath := "README.md"
	ref := repo.DefaultBranch

	t.Run("bad treePath", func(t *testing.T) {
		badTreePath := "bad/tree.md"
		fileContentResponse, err := GetContentsOrList(ctx, repo, badTreePath, ref)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: , rel_path: bad]")
		assert.Nil(t, fileContentResponse)
	})

	t.Run("bad ref", func(t *testing.T) {
		badRef := "bad_ref"
		fileContentResponse, err := GetContentsOrList(ctx, repo, treePath, badRef)
		assert.Error(t, err)
		assert.EqualError(t, err, "object does not exist [id: "+badRef+", rel_path: ]")
		assert.Nil(t, fileContentResponse)
	})
}

func TestGetContentsOrListOfEmptyRepos(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo15")
	ctx.SetParams(":id", "15")
	test.LoadRepo(t, ctx, 15)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository

	t.Run("empty repo", func(t *testing.T) {
		contents, err := GetContentsOrList(ctx, repo, "", "")
		assert.NoError(t, err)
		assert.Empty(t, contents)
	})
}

func TestGetBlobBySHA(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	ctx.SetParams(":id", "1")
	ctx.SetParams(":sha", sha)

	gbr, err := GetBlobBySHA(ctx, ctx.Repo.Repository, ctx.Params(":sha"))
	expectedGBR := &api.GitBlobResponse{
		Content:  "dHJlZSAyYTJmMWQ0NjcwNzI4YTJlMTAwNDllMzQ1YmQ3YTI3NjQ2OGJlYWI2CmF1dGhvciB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+IDE0ODk5NTY0NzkgLTA0MDAKY29tbWl0dGVyIEV0aGFuIEtvZW5pZyA8ZXRoYW50a29lbmlnQGdtYWlsLmNvbT4gMTQ4OTk1NjQ3OSAtMDQwMAoKSW5pdGlhbCBjb21taXQK",
		Encoding: "base64",
		URL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/65f1bf27bc3bf70f64657658635e66094edbcb4d",
		SHA:      "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Size:     180,
	}
	assert.NoError(t, err)
	assert.Equal(t, expectedGBR, gbr)
}
