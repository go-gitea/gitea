// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/rand"
	"io"
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

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		// Phase 1: Push with no limit
		setting.Repository.GitSizeMax = -1
		setting.Repository.LFSSizeMax = -1
		doCommitAndPush(t, 1024, dstPath, "under-phase1-")

		// Phase 2: Push with limit enabled but not exceeded
		setting.Repository.GitSizeMax = 50 * 1024 // 50 KiB
		doCommitAndPush(t, 1024, dstPath, "under-phase2-")
	})

	t.Run("Over", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-over"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		// Set restrictive limit and attempt push
		setting.Repository.GitSizeMax = 100
		setting.Repository.LFSSizeMax = -1
		doCommitAndPushWithExpectedError(t, 1024, dstPath, "over-")
	})

	t.Run("UnderAfterResize", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-resize"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		// Attempt push with restrictive limit - should fail
		setting.Repository.GitSizeMax = 100
		setting.Repository.LFSSizeMax = -1
		doCommitAndPushWithExpectedError(t, 1024, dstPath, "resize-")

		// Increase limit and retry same push - should succeed
		setting.Repository.GitSizeMax = 30 * 1024 // 30 KiB
		_, _, err := gitcmd.NewCommand("push", "origin", "master").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err, "Push should succeed after limit increase")
	})

	t.Run("DeletionAndSoftEnforcement", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-git-soft"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)

		// Step 1: Push 1KB file with no limit
		setting.Repository.GitSizeMax = -1
		setting.Repository.LFSSizeMax = -1
		doCommitAndPush(t, 1024, dstPath, "soft-base-")

		// Step 2: Push 10KB file
		doCommitAndPush(t, 10*1024, dstPath, "soft-big-")

		// Step 3: Delete big file using reset
		_, _, err := gitcmd.NewCommand("reset", "--hard", "HEAD~1").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err, "Reset should succeed")

		// Step 4: Set very restrictive limit
		setting.Repository.GitSizeMax = 10 // 10 bytes

		// Step 5: Force push - should succeed (soft enforcement)
		_, _, err = gitcmd.NewCommand("push", "--force-with-lease", "origin", "master").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err, "Force push should succeed with soft enforcement")

		// Step 6: Try to push another 1KB file - should fail
		doCommitAndPushWithExpectedError(t, 1024, dstPath, "soft-new-")
	})
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
		err = gitcmd.NewCommand("push", "origin", "master").WithDir(dstPath).Run(t.Context())
		assert.NoError(t, err)
	}

	t.Run("PushUnderLimit", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-under"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		// Push with limit enabled but not exceeded
		setting.Repository.GitSizeMax = -1
		setting.Repository.LFSSizeMax = 10000
		doCommitAndPushWithData(t, dstPath, "data-under.dat", "some-content-under")
	})

	t.Run("PushOverLimit", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-over"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		// Push with restrictive limit - should fail
		setting.Repository.GitSizeMax = -1
		setting.Repository.LFSSizeMax = 5
		doCommitAndPushWithDataWithExpectedError(t, dstPath, "data-over.dat", "some-content-over-limit")
	})

	t.Run("SoftEnforcement", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		repoName := "repo-lfs-soft-enforce"
		ctx := NewAPITestContext(t, username, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Cleanup: reset limits and delete repository
		defer func() {
			setting.Repository.GitSizeMax = -1
			setting.Repository.LFSSizeMax = -1
		}()
		defer doAPIDeleteRepository(ctx)

		doAPICreateRepository(ctx, false)(t)
		dstPath := t.TempDir()
		u.Path = ctx.GitPath()
		doGitClone(dstPath, u)(t)
		setupLFS(t, dstPath)

		// Step 1 & 2: Init LFS and push 1024B file with random content (Commit 1)
		setting.Repository.GitSizeMax = -1
		setting.Repository.LFSSizeMax = -1
		doCommitAndPushLFSWithRandomData(t, dstPath, "data-soft-1.dat", 1024)

		// Step 3: Push 10240B LFS file with random content (Commit 2)
		doCommitAndPushLFSWithRandomData(t, dstPath, "data-soft-2.dat", 10240)

		// Step 4: Set limit to 10 KiB (1 KiB below current ~11 KiB)
		setting.Repository.LFSSizeMax = 10 * 1024

		// Step 5: Try to push 1KB LFS file - should fail (Commit 3, local only)
		err := os.WriteFile(path.Join(dstPath, "data-soft-3.dat"), generateRandomData(1024), 0o644)
		assert.NoError(t, err)
		err = gitcmd.NewCommand("add", "data-soft-3.dat").WithDir(dstPath).Run(t.Context())
		assert.NoError(t, err)
		err = gitcmd.NewCommand("commit", "-m", "Add data-soft-3.dat").WithDir(dstPath).Run(t.Context())
		assert.NoError(t, err)
		err = gitcmd.NewCommand("push", "origin", "master").WithDir(dstPath).Run(t.Context())
		assert.Error(t, err, "Push should fail when exceeding LFS limit")

		// Step 6: Reset to Commit 1 (removes Commits 2 & 3)
		_, _, err = gitcmd.NewCommand("reset", "--hard", "HEAD~2").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err, "Reset should succeed")
		_, _, err = gitcmd.NewCommand("push", "--force-with-lease", "origin", "master").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err, "Force push should succeed with soft enforcement")

		// Step 7: Try to push 1024B LFS file - should still fail
		doCommitAndPushLFSWithRandomDataWithExpectedError(t, dstPath, "data-soft-new.dat", 1024)
	})
}

// Helper functions

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

func generateRandomData(size int) []byte {
	data := make([]byte, size)
	_, _ = io.ReadFull(rand.Reader, data)
	return data
}

func doCommitAndPushLFSWithRandomData(t *testing.T, repoPath, filename string, size int) {
	err := os.WriteFile(path.Join(repoPath, filename), generateRandomData(size), 0o644)
	assert.NoError(t, err)
	err = gitcmd.NewCommand("add").AddDynamicArguments(filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)

	// Verify file is tracked by LFS
	stdout, _, err := gitcmd.NewCommand("lfs", "ls-files").WithDir(repoPath).RunStdString(t.Context())
	assert.NoError(t, err, "git lfs ls-files should succeed")
	assert.Contains(t, stdout, filename, "File %s should be tracked by LFS", filename)

	err = gitcmd.NewCommand("commit", "-m").AddDynamicArguments("Add " + filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("push", "origin", "master").WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
}

func doCommitAndPushLFSWithRandomDataWithExpectedError(t *testing.T, repoPath, filename string, size int) {
	err := os.WriteFile(path.Join(repoPath, filename), generateRandomData(size), 0o644)
	assert.NoError(t, err)
	err = gitcmd.NewCommand("add").AddDynamicArguments(filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)

	// Verify file is tracked by LFS
	stdout, _, err := gitcmd.NewCommand("lfs", "ls-files").WithDir(repoPath).RunStdString(t.Context())
	assert.NoError(t, err, "git lfs ls-files should succeed")
	assert.Contains(t, stdout, filename, "File %s should be tracked by LFS", filename)

	err = gitcmd.NewCommand("commit", "-m").AddDynamicArguments("Add " + filename).WithDir(repoPath).Run(t.Context())
	assert.NoError(t, err)
	err = gitcmd.NewCommand("push", "origin", "master").WithDir(repoPath).Run(t.Context())
	assert.Error(t, err)
}

// Reuse global helpers for Git: doCommitAndPush, doCommitAndPushWithExpectedError
