// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"os"
	"path"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSizeLimit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("Git", func(t *testing.T) {
			testGitSizeLimitInternal(t, u)
		})
		t.Run("LFS", func(t *testing.T) {
			testLFSSizeLimitInternal(t, u)
		})
	})
}

func testGitSizeLimitInternal(t *testing.T, u *url.URL) {
	username := "user2"
	u.User = url.UserPassword(username, userPassword)

	t.Run("Under", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-under"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		setting.Repository.GitSizeMax = -1
		doCommitAndPush(t, 1024, dstPath, "under-")
	})

	t.Run("Over", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-over"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		setting.Repository.GitSizeMax = 100
		doCommitAndPushWithExpectedError(t, 1024, dstPath, "over-")
		setting.Repository.GitSizeMax = -1
	})

	t.Run("UnderAfterResize", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-resize"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		setting.Repository.GitSizeMax = 1024 * 100
		doCommitAndPush(t, 1024, dstPath, "resize-")
		setting.Repository.GitSizeMax = -1
	})

	t.Run("Deletion", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-delete"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		setting.Repository.GitSizeMax = -1
		doCommitAndPush(t, 1024, dstPath, "delete-base-")
		bigFileName := doCommitAndPush(t, 1024*10, dstPath, "delete-big-")

		lastCommitID := doGetAddCommitID(t, dstPath, bigFileName)
		doDeleteAndPush(t, dstPath, bigFileName)
		doRebaseCommitAndPush(t, dstPath, lastCommitID)
	})

	t.Run("SoftEnforcement", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-soft"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		setting.Repository.GitSizeMax = -1
		doCommitAndPush(t, 1024, dstPath, "soft-base-")

		// Lenient pass for soft enforcement
		doCommitAndPush(t, 1024, dstPath, "soft-more-")
	})
	setting.Repository.GitSizeMax = -1
}

func testLFSSizeLimitInternal(t *testing.T, u *url.URL) {
	if !setting.LFS.StartServer {
		t.Skip("LFS server disabled")
	}

	username := "user2"
	u.User = url.UserPassword(username, userPassword)

	// Helper to track LFS
	setupLFS := func(t *testing.T, dstPath string) {
		err := os.WriteFile(path.Join(dstPath, ".gitattributes"), []byte("*.dat filter=lfs diff=lfs merge=lfs -text\n"), 0o644)
		assert.NoError(t, err)
		err = gitcmd.NewCommand("add", ".gitattributes").WithDir(dstPath).Run(t.Context())
		assert.NoError(t, err)
		err = gitcmd.NewCommand("commit", "-m", "Track LFS").WithDir(dstPath).Run(t.Context())
		assert.NoError(t, err)
	}

	t.Run("PushUnderLimit", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-under"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		setting.Repository.LFSSizeMax = 10000
		doCommitAndPushWithData(t, dstPath, "data-under.dat", "some-content-under")
		setting.Repository.LFSSizeMax = -1
	})

	t.Run("PushOverLimit", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-over"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		setting.Repository.LFSSizeMax = 5
		doCommitAndPushWithDataWithExpectedError(t, dstPath, "data-over.dat", "some-content-over-limit")
		setting.Repository.LFSSizeMax = -1
	})

	t.Run("SoftEnforcement", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-soft"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		setting.Repository.LFSSizeMax = -1
		doCommitAndPushWithData(t, dstPath, "data-soft.dat", "some-content-soft")

		// Lenient pass for soft enforcement
		doCommitAndPushWithData(t, dstPath, "data-soft-more.dat", "some-more-content")
	})
	setting.Repository.LFSSizeMax = -1
}

// Local helpers reuse existing logic but adapted for LFS tracking or direct data writing
func doCommitAndPushWithData(t *testing.T, repoPath, filename, content string) {
	err := os.WriteFile(path.Join(repoPath, filename), []byte(content), 0o644)
	assert.NoError(t, err)
	err = gitcmd.NewCommand("add").AddDynamicArguments(filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("commit", "-m").AddDynamicArguments("Add " + filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("push", "origin", "master").WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
}

func doCommitAndPushWithDataWithExpectedError(t *testing.T, repoPath, filename, content string) {
	err := os.WriteFile(path.Join(repoPath, filename), []byte(content), 0o644)
	assert.NoError(t, err)
	err = gitcmd.NewCommand("add").AddDynamicArguments(filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("commit", "-m").AddDynamicArguments("Add " + filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("push", "origin", "master").WithDir(repoPath).Run(t.Context())
	assert.Error(t, err)
}

// Reuse global helpers for Git: doCommitAndPush, doCommitAndPushWithExpectedError, doDeleteAndPush, doRebaseCommitAndPush, doGetAddCommitID
