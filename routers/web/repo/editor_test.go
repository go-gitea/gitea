// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/contexttest"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadName(t *testing.T) {
	unittest.PrepareTestEnv(t)

	kases := map[string]string{
		".git/refs/master":               "",
		"/root/abc":                      "root/abc",
		"./../../abc":                    "abc",
		"a/../.git":                      "",
		"a/../../../abc":                 "abc",
		"../../../acd":                   "acd",
		"../../.git/abc":                 "",
		"..\\..\\.git/abc":               "..\\..\\.git/abc",
		"..\\../.git/abc":                "",
		"..\\../.git":                    "",
		"abc/../def":                     "def",
		".drone.yml":                     ".drone.yml",
		".abc/def/.drone.yml":            ".abc/def/.drone.yml",
		"..drone.yml.":                   "..drone.yml.",
		"..a.dotty...name...":            "..a.dotty...name...",
		"..a.dotty../.folder../.name...": "..a.dotty../.folder../.name...",
	}
	for k, v := range kases {
		assert.EqualValues(t, cleanUploadFileName(k), v)
	}
}

func TestGetUniquePatchBranchName(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	expectedBranchName := "user2-patch-1"
	branchName := GetUniquePatchBranchName(ctx)
	assert.Equal(t, expectedBranchName, branchName)
}

func TestGetClosestParentWithFiles(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetParams(":id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	branch := repo.DefaultBranch
	gitRepo, _ := git.OpenRepository(git.DefaultContext, repo.RepoPath())
	defer gitRepo.Close()
	commit, _ := gitRepo.GetBranchCommit(branch)
	var expectedTreePath string // Should return the root dir, empty string, since there are no subdirs in this repo
	for _, deletedFile := range []string{
		"dir1/dir2/dir3/file.txt",
		"file.txt",
	} {
		treePath := GetClosestParentWithFiles(deletedFile, commit)
		assert.Equal(t, expectedTreePath, treePath)
	}
}
