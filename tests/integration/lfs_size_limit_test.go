// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func base62(n int64) string {
	if n == 0 {
		return string(alphabet[0])
	}
	var buf [11]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%62]
		n /= 62
	}
	return string(buf[i:])
}

// ID5: 5-char ID derived from unix seconds.
func ID5() string {
	sec := time.Now().Unix()
	s := base62(sec)
	if len(s) < 5 {
		return fmt.Sprintf("%05s", s)
	}
	return s[len(s)-5:]
}

// TestLFSSizeLimit exercises the pre-receive size limit logic.
// Runs sequentially (no t.Parallel) because it mutates GLOBAL settings.
//
// Enable/disable via:
//
//	setting.SaveGlobalRepositorySetting(enabled, repoLimit, lfsLimit, lfsSizeInRepoSize)
//
// Scenarios:
//   - repo-only limit: blocks git blobs but not LFS objects (when lfsSizeInRepoSize=false, lfsLimit=0)
//   - LFS-only limit: blocks LFS objects but not git blobs
//   - combined repo+LFS (LFS counted into repo size): blocks LFS when it would exceed repo limit
//   - per-repo LFS override wins over global default (retry same push after increasing per-repo LFS limit)
func TestLFSSizeLimit(t *testing.T) {
	onGiteaRun(t, testLFSSizeLimit)
}

func testLFSSizeLimit(t *testing.T, baseURL *url.URL) {
	// Always disable at the end so we don't leak to other integration tests.
	t.Cleanup(func() {
		setting.SaveGlobalRepositorySetting(false, 0, 0, false)
	})

	if !setting.LFS.StartServer {
		t.Skip("LFS server disabled")
	}
	// ---- sizes ----
	// Repo-size checks are not precise (compression), so use very different sizes vs limit.
	const (
		repoLimitBytes = int64(64 * 1024)     // 64KiB
		gitUnderBytes  = int(8 * 1024)        // 8KiB
		gitOverBytes   = int(4 * 1024 * 1024) // 4MiB (far above limit)
		lfsBigBytes    = int(1 * 1024 * 1024) // 1MiB (should still pass repo-only when not counted)
	)

	// LFS-only checks are precise.
	const (
		lfsLimitBytes = int64(64 * 1024) // 64KiB
		lfsUnderBytes = int(32 * 1024)   // 32KiB
		lfsOverBytes  = int(128 * 1024)  // 128KiB
	)

	// Combined repo+LFS (LFS counted into repo size).
	const (
		combinedRepoLimitBytes = int64(128 * 1024) // 128KiB
		combinedLFSUnderBytes  = int(64 * 1024)    // 64KiB
		combinedLFSOverBytes   = int(512 * 1024)   // 512KiB
	)

	// ---- helpers ----

	newLimitRepo := func(t *testing.T, suffix string) APITestContext {
		t.Helper()

		// Make a unique repo name without relying on tests.GetTestUID / CreateRepo.
		repoName := fmt.Sprintf(
			"lfsl-%s-%s",
			suffix,
			ID5(),
		)

		ctx := NewAPITestContext(
			t,
			"user2",
			repoName,
			auth_model.AccessTokenScopeWriteRepository,
			auth_model.AccessTokenScopeWriteUser,
		)

		// Create via API (same style as git_general_test.go)
		doAPICreateRepository(ctx, false)(t)
		return ctx
	}

	cloneHTTP := func(t *testing.T, ctx APITestContext) string {
		t.Helper()

		u := *baseURL // copy
		u.Path = ctx.GitPath()
		u.User = url.UserPassword(ctx.Username, userPassword)

		dst := t.TempDir()
		doGitClone(dst, &u)(t)
		return dst
	}

	commitSignature := func() *git.Signature {
		return &git.Signature{
			Email: "user2@example.com",
			Name:  "User Two",
			When:  time.Now(),
		}
	}

	configureLFSForPrefix := func(t *testing.T, repoPath, prefix string) {
		t.Helper()

		// git lfs install
		err := gitcmd.NewCommand("lfs").AddArguments("install").WithDir(repoPath).Run(t.Context())
		require.NoError(t, err)

		// track prefix*
		_, _, err = gitcmd.NewCommand("lfs").
			AddArguments("track").
			AddDynamicArguments(prefix + "*").
			WithDir(repoPath).
			RunStdString(t.Context())
		require.NoError(t, err)

		// commit .gitattributes
		err = git.AddChanges(t.Context(), repoPath, false, ".gitattributes")
		require.NoError(t, err)

		sig := commitSignature()
		err = git.CommitChanges(t.Context(), repoPath, git.CommitChangesOptions{
			Committer: sig,
			Author:    sig,
			Message:   "configure LFS tracking",
		})
		require.NoError(t, err)
	}

	pushCurrentBranch := func(t *testing.T, repoPath string) error {
		t.Helper()
		_, _, err := gitcmd.NewCommand("push", "origin", "master").WithDir(repoPath).RunStdString(t.Context())
		return err
	}

	// Push a git blob by creating a new random file (random-ish content reduces compression effects).
	pushGitBlob := func(t *testing.T, ctx APITestContext, size int) error {
		t.Helper()

		repoPath := cloneHTTP(t, ctx)

		_, err := generateCommitWithNewData(
			t.Context(),
			size,
			repoPath,
			"user2@example.com",
			"User Two",
			"git-data-",
		)
		require.NoError(t, err)

		return pushCurrentBranch(t, repoPath)
	}

	// Push an LFS object by tracking prefix* and then committing a file created by generateCommitWithNewData.
	pushLFSObjectOnce := func(t *testing.T, ctx APITestContext, size int) error {
		t.Helper()

		repoPath := cloneHTTP(t, ctx)

		const prefix = "lfs-data-"
		configureLFSForPrefix(t, repoPath, prefix)

		_, err := generateCommitWithNewData(
			t.Context(),
			size,
			repoPath,
			"user2@example.com",
			"User Two",
			prefix,
		)
		require.NoError(t, err)

		return pushCurrentBranch(t, repoPath)
	}

	// Prepare a single LFS commit in a clone and return the clone path, so we can:
	//  - push (expect fail)
	//  - change limits
	//  - push SAME COMMIT again (expect pass)
	prepareLFSPushForRetry := func(t *testing.T, ctx APITestContext, size int) string {
		t.Helper()

		repoPath := cloneHTTP(t, ctx)

		const prefix = "lfs-retry-"
		configureLFSForPrefix(t, repoPath, prefix)

		_, err := generateCommitWithNewData(
			t.Context(),
			size,
			repoPath,
			"user2@example.com",
			"User Two",
			prefix,
		)
		require.NoError(t, err)

		// Ensure the repoPath is used (avoid accidental cleanup / unused).
		_, err = os.Stat(filepath.Join(repoPath, ".git"))
		require.NoError(t, err)

		return repoPath
	}

	runGlobalThenPerRepo := func(
		t *testing.T,
		name string,
		global func(t *testing.T),
		perRepo func(t *testing.T),
	) {
		t.Helper()
		t.Run(name+"/Global", func(t *testing.T) { global(t) })
		t.Run(name+"/PerRepo", func(t *testing.T) { perRepo(t) })
	}

	// ---- tests ----

	runGlobalThenPerRepo(t,
		"GlobalAndRepoOnlyLimit_BlocksGitButNotLFS",
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Global: repo limit ON, LFS limit OFF, LFS not counted into repo size.
			setting.SaveGlobalRepositorySetting(true, repoLimitBytes, 0, false)

			ctx := newLimitRepo(t, "ggit")

			// push over on git (NOK)
			err := pushGitBlob(t, ctx, gitOverBytes)
			assert.Error(t, err, "git file well over repo limit should be rejected")

			// push LFS (OK)
			err = pushLFSObjectOnce(t, ctx, lfsBigBytes)
			assert.NoError(t, err, "LFS push should be allowed when repo-only limit is enabled and LFSSizeInRepoSize=false")

			// push under on git (OK)
			err = pushGitBlob(t, ctx, gitUnderBytes)
			assert.NoError(t, err, "small git file should be accepted if under repo limit")
		},
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Per-repo: enable checking globally but do not set global limits.
			setting.SaveGlobalRepositorySetting(true, 0, 0, false)

			ctx := newLimitRepo(t, "rgit")

			// set per-repo repo limit
			t.Run("APISetRepoSizeLimit", doAPISetRepoSizeLimit(ctx, ctx.Username, ctx.Reponame, repoLimitBytes))

			// push over on git (NOK)
			err := pushGitBlob(t, ctx, gitOverBytes)
			assert.Error(t, err, "git file well over per-repo repo limit should be rejected")

			// push LFS (OK)
			err = pushLFSObjectOnce(t, ctx, lfsBigBytes)
			assert.NoError(t, err, "LFS push should be allowed when only per-repo repo limit is set and LFSSizeInRepoSize=false")

			// push under on git (OK)
			err = pushGitBlob(t, ctx, gitUnderBytes)
			assert.NoError(t, err, "small git file should be accepted under per-repo repo limit")
		},
	)

	runGlobalThenPerRepo(t,
		"GlobalAndLFSOnlyLimit_BlocksLFSButNotGit",
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Global: LFS limit ON, repo limit OFF, LFS not counted into repo size.
			setting.SaveGlobalRepositorySetting(true, 0, lfsLimitBytes, false)

			ctx := newLimitRepo(t, "glfs")

			// push on git (OK)
			err := pushGitBlob(t, ctx, gitOverBytes)
			assert.NoError(t, err, "git push should be allowed when repo limit is 0")

			// push over on LFS (NOK)
			err = pushLFSObjectOnce(t, ctx, lfsOverBytes)
			assert.Error(t, err, "LFS push above global LFS limit must be rejected")

			// push under on LFS (OK) (under is last)
			err = pushLFSObjectOnce(t, ctx, lfsUnderBytes)
			assert.NoError(t, err, "LFS push under global LFS limit must be accepted")
		},
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Per-repo: enable checking globally but do not set global limits.
			setting.SaveGlobalRepositorySetting(true, 0, 0, false)

			ctx := newLimitRepo(t, "rlfs")

			// set per-repo LFS limit
			t.Run("APISetRepoLFSSizeLimit", doAPISetRepoLFSSizeLimit(ctx, ctx.Username, ctx.Reponame, lfsLimitBytes))

			// push on git (OK)
			err := pushGitBlob(t, ctx, gitOverBytes)
			assert.NoError(t, err, "git push should be allowed when repo limit is 0")

			// push over on LFS (NOK)
			err = pushLFSObjectOnce(t, ctx, lfsOverBytes)
			assert.Error(t, err, "LFS push above per-repo LFS limit must be rejected")
		},
	)

	runGlobalThenPerRepo(t,
		"GlobalOrCombinedRepoAndLFSLimits_BlocksLFSandGit",
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Global: repo limit ON, no LFS-specific limit, but LFS counted into repo size.
			setting.SaveGlobalRepositorySetting(true, combinedRepoLimitBytes, 0, true)

			ctx := newLimitRepo(t, "gc")

			// push over via LFS (NOK)
			err := pushLFSObjectOnce(t, ctx, combinedLFSOverBytes)
			assert.Error(t, err, "LFS push must be rejected when it would exceed repo limit and LFSSizeInRepoSize=true")

			// push under via LFS (OK)
			err = pushLFSObjectOnce(t, ctx, combinedLFSUnderBytes)
			assert.NoError(t, err, "LFS push under repo limit must be accepted when LFSSizeInRepoSize=true")
		},
		func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Per-repo: enable checking globally with LFSSizeInRepoSize=true but no global limits.
			setting.SaveGlobalRepositorySetting(true, 0, 0, true)

			ctx := newLimitRepo(t, "rc")

			// set per-repo repo limit
			t.Run("APISetRepoSizeLimit", doAPISetRepoSizeLimit(ctx, ctx.Username, ctx.Reponame, combinedRepoLimitBytes))

			// push over via LFS (NOK)
			err := pushLFSObjectOnce(t, ctx, combinedLFSOverBytes)
			assert.Error(t, err, "LFS push must be rejected when it would exceed per-repo repo limit and LFSSizeInRepoSize=true")

			// push under via LFS (OK)
			err = pushLFSObjectOnce(t, ctx, combinedLFSUnderBytes)
			assert.NoError(t, err, "LFS push under per-repo repo limit must be accepted when LFSSizeInRepoSize=true")
		},
	)

	t.Run("PerRepoLFSOverride_WinsOverGlobalDefault", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Global: strict LFS limit, no repo limit.
		setting.SaveGlobalRepositorySetting(true, 0, lfsLimitBytes, false)

		ctx := newLimitRepo(t, "rwing")

		// Prepare one LFS commit and keep the clone so we can retry pushing SAME commit.
		repoPath := prepareLFSPushForRetry(t, ctx, lfsOverBytes)

		// First push must fail.
		err := pushCurrentBranch(t, repoPath)
		assert.Error(t, err, "push must fail under strict global LFS limit")

		// Increase per-repo LFS limit to allow the previously rejected object.
		// Use a clearly larger limit than the file size.
		newPerRepoLimit := int64(lfsOverBytes) * 4
		t.Run("APISetRepoLFSSizeLimit", doAPISetRepoLFSSizeLimit(ctx, ctx.Username, ctx.Reponame, newPerRepoLimit))

		// Retry pushing SAME commit must succeed now.
		err = pushCurrentBranch(t, repoPath)
		assert.NoError(t, err, "per-repo LFS limit must override global default and allow previously rejected push")
	})
}
