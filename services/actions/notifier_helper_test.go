// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"testing"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIfNeedApproval(t *testing.T) {
	alwaysWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
		return true, nil
	}
	neverWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
		return false, nil
	}
	hasMerged := func(_ context.Context, _, _ int64) (bool, error) { return true, nil }
	noMerged := func(_ context.Context, _, _ int64) (bool, error) { return false, nil }
	errPerm := errors.New("perm error")
	errMerge := errors.New("merge error")

	forkRun := &actions_model.ActionRun{IsForkPullRequest: true, TriggerEvent: actions_module.GithubEventPullRequest}
	nonForkRun := &actions_model.ActionRun{IsForkPullRequest: false, TriggerEvent: actions_module.GithubEventPullRequest}
	prTargetRun := &actions_model.ActionRun{IsForkPullRequest: true, TriggerEvent: actions_module.GithubEventPullRequestTarget}

	repo := &repo_model.Repository{ID: 1}
	normalUser := &user_model.User{ID: 10}
	restrictedUser := &user_model.User{ID: 11, IsRestricted: true}

	t.Run("not a fork PR never needs approval", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), nonForkRun, repo, normalUser, alwaysWrite, hasMerged)
		require.NoError(t, err)
		assert.False(t, need)
	})

	t.Run("pull_request_target never needs approval even when fork", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), prTargetRun, repo, normalUser, alwaysWrite, hasMerged)
		require.NoError(t, err)
		assert.False(t, need)
	})

	t.Run("restricted user always needs approval", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, restrictedUser, alwaysWrite, hasMerged)
		require.NoError(t, err)
		assert.True(t, need)
	})

	t.Run("fork PR with write permission does not need approval", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, alwaysWrite, noMerged)
		require.NoError(t, err)
		assert.False(t, need)
	})

	t.Run("fork PR with merged PR but no write permission does not need approval", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, neverWrite, hasMerged)
		require.NoError(t, err)
		assert.False(t, need)
	})

	t.Run("fork PR with no write and no merged PR needs approval", func(t *testing.T) {
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, neverWrite, noMerged)
		require.NoError(t, err)
		assert.True(t, need)
	})

	t.Run("canWriteActions error is propagated", func(t *testing.T) {
		failWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
			return false, errPerm
		}
		_, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, failWrite, noMerged)
		require.ErrorIs(t, err, errPerm)
	})

	t.Run("hasMergedPR error is propagated", func(t *testing.T) {
		failMerge := func(_ context.Context, _, _ int64) (bool, error) { return false, errMerge }
		_, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, neverWrite, failMerge)
		require.ErrorIs(t, err, errMerge)
	})

	t.Run("restricted user skips permission check entirely", func(t *testing.T) {
		// The perm and merge functions must not be called for a restricted user.
		called := false
		trackWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
			called = true
			return true, nil
		}
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, restrictedUser, trackWrite, noMerged)
		require.NoError(t, err)
		assert.True(t, need)
		assert.False(t, called, "permission check must not run for restricted user")
	})
}
