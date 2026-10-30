// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/private"
	"gitea.dev/services/contexttest"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookPostReceiveSyncBranches(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	createEmptyRepo := func(t *testing.T, defaultBranch string) *repo_model.Repository {
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{Name: "empty-repo", DefaultBranch: "main"}, true)
		require.NoError(t, err)
		t.Cleanup(func() {
			assert.NoError(t, repo_service.DeleteRepositoryDirectly(context.WithoutCancel(t.Context()), repo.ID))
		})
		repo.DefaultBranch = defaultBranch
		require.NoError(t, repo_model.UpdateDefaultBranch(t.Context(), repo))
		return repo
	}
	// write refs directly to the bare repository like a push does, without running the hooks
	pushRefs := func(t *testing.T, repo *repo_model.Repository, refs ...git.RefName) *private.HookOptions {
		treeID, _, err := gitcmd.NewCommand("mktree").WithRepo(repo).RunStdString(t.Context())
		require.NoError(t, err)
		commitID, _, err := gitcmd.NewCommand("commit-tree").AddDynamicArguments(strings.TrimSpace(treeID)).AddOptionValues("-m", "init").WithRepo(repo).RunStdString(t.Context())
		require.NoError(t, err)
		commitID = strings.TrimSpace(commitID)
		opts := &private.HookOptions{UserID: user2.ID}
		for _, ref := range refs {
			require.NoError(t, gitcmd.NewCommand("update-ref").AddDynamicArguments(string(ref), commitID).WithRepo(repo).Run(t.Context()))
			opts.OldCommitIDs = append(opts.OldCommitIDs, git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String())
			opts.NewCommitIDs = append(opts.NewCommitIDs, commitID)
			opts.RefFullNames = append(opts.RefFullNames, ref)
		}
		return opts
	}
	syncBranches := func(t *testing.T, repo *repo_model.Repository, opts *private.HookOptions) {
		ctx, _ := contexttest.MockPrivateContext(t, "/")
		hookPostReceiveSyncDatabaseBranches(ctx, opts, repo, hookPostReceiveCollectPushUpdates(opts, repo))
		hookPostReceiveSyncRepoDefaultBranch(ctx, opts, repo)
	}
	assertRepo := func(t *testing.T, repo *repo_model.Repository, isEmpty bool, defaultBranch string) {
		dbRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, isEmpty, dbRepo.IsEmpty)
		assert.Equal(t, defaultBranch, dbRepo.DefaultBranch)
		assert.Equal(t, isEmpty, repo.IsEmpty)
		assert.Equal(t, defaultBranch, repo.DefaultBranch)
		headBranch, err := git.GetDefaultBranch(t.Context(), repo)
		require.NoError(t, err)
		assert.Equal(t, defaultBranch, headBranch)
	}

	for _, tc := range []struct {
		name          string
		defaultBranch string
		branches      []string
		expected      string
	}{
		{name: "KeepDefaultBranch", defaultBranch: "main", branches: []string{"dev", "main"}, expected: "main"},
		{name: "GuessDefaultBranch", defaultBranch: "main", branches: []string{"branch-0", "master"}, expected: "master"},
		{name: "FirstPushedBranch", defaultBranch: "main", branches: []string{"feature", "other"}, expected: "feature"},
		{name: "NoDefaultBranch", defaultBranch: "", branches: []string{"feature"}, expected: "feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := createEmptyRepo(t, tc.defaultBranch)

			refs := make([]git.RefName, 0, len(tc.branches))
			for _, branch := range tc.branches {
				refs = append(refs, git.RefNameFromBranch(branch))
			}
			syncBranches(t, repo, pushRefs(t, repo, refs...))
			assertRepo(t, repo, false, tc.expected)
		})
	}

	t.Run("NonEmptyRepoGuessDefaultBranch", func(t *testing.T) {
		repo := createEmptyRepo(t, "master")

		// the default branch "master" is gone from git, e.g. a converted mirror whose upstream renamed it
		pushRefs(t, repo, git.RefNameFromBranch("main"))
		require.NoError(t, git.SetDefaultBranch(t.Context(), repo, "master"))
		marked, err := repo_model.MarkRepositoryNotEmpty(t.Context(), repo.ID, "master")
		require.NoError(t, err)
		require.True(t, marked)
		repo.IsEmpty = false

		syncBranches(t, repo, pushRefs(t, repo, git.RefNameFromBranch("feature")))
		assertRepo(t, repo, false, "main")
	})

	t.Run("DefaultBranchNotPushed", func(t *testing.T) {
		repo := createEmptyRepo(t, "main")

		// "master" was pushed before but the repo stayed empty, e.g. its post-receive hook failed
		pushRefs(t, repo, git.RefNameFromBranch("master"))

		syncBranches(t, repo, pushRefs(t, repo, git.RefNameFromBranch("feature")))
		assertRepo(t, repo, false, "master")
	})

	t.Run("TagsOnly", func(t *testing.T) {
		repo := createEmptyRepo(t, "main")

		syncBranches(t, repo, pushRefs(t, repo, git.RefNameFromTag("v1.0")))
		assertRepo(t, repo, true, "main")
	})

	t.Run("MarkedByRepoHome", func(t *testing.T) {
		repo := createEmptyRepo(t, "main")

		opts := pushRefs(t, repo, git.RefNameFromBranch("feature"))
		// the repo home page finds branches in git and only clears is_empty
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), &repo_model.Repository{ID: repo.ID}, "is_empty"))

		syncBranches(t, repo, opts)
		assertRepo(t, repo, false, "feature")
	})

	t.Run("InitializedConcurrently", func(t *testing.T) {
		repo := createEmptyRepo(t, "main")

		// the push of "dev" initializes the repo first
		pushRefs(t, repo, git.RefNameFromBranch("dev"))
		marked, err := repo_model.MarkRepositoryNotEmpty(t.Context(), repo.ID, "dev")
		require.NoError(t, err)
		require.True(t, marked)
		require.NoError(t, git.SetDefaultBranch(t.Context(), repo, "dev"))

		syncBranches(t, repo, pushRefs(t, repo, git.RefNameFromBranch("feature")))
		assertRepo(t, repo, false, "dev")
	})
}
