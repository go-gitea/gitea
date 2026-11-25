// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeQueueRequiredContexts(t *testing.T) {
	assert.Equal(t,
		[]string{"CI / test (merge_group)", "CI / lint (merge_group)", "CI / build (merge_group)", "CI (pull_request) / *"},
		MergeQueueRequiredContexts([]string{"CI / test (pull_request)", "CI / lint (pull_request_target)", "CI / build (push)", "CI (pull_request) / *"}),
	)
}

func TestQueuedPullHeadAndBaseChanges(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadBaseRepo(ctx))
	entry := &MergeQueueEntry{MergeQueueEntry: &pull_model.MergeQueueEntry{PullID: pr.ID, HeadSHA: pr.MergeBase}, PullRequest: pr, Enqueuer: pr.BaseRepo.MustOwner(ctx)}
	entries := []*MergeQueueEntry{entry}
	require.ErrorAs(t, MergeQueuePullRequests(ctx, entries), new(ErrSHADoesNotMatch))

	pr.BaseBranch = "MASTER"
	require.ErrorContains(t, MergeQueuePullRequests(ctx, entries), "no longer targets MASTER")

	pr.BaseBranch, entry.GroupRef = "queue-lease", pr.GetGitHeadRefName()
	require.NoError(t, pr.UpdateCols(ctx, "base_branch"))
	head, err := git.GetFullCommitID(ctx, pr.BaseRepo, entry.GroupRef)
	require.NoError(t, err)
	parent, err := git.GetFullCommitID(ctx, pr.BaseRepo, head+"^")
	require.NoError(t, err)
	entry.HeadSHA, entry.BaseSHA, entry.GroupHeadSHA = head, parent, head
	require.NoError(t, git.CreateBranch(ctx, pr.BaseRepo, pr.BaseBranch, parent+"^"))
	rewound, err := git.GetBranchCommitID(ctx, pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	require.ErrorAs(t, MergeQueuePullRequests(ctx, entries), new(*git.ErrPushOutOfDate))
	current, err := git.GetBranchCommitID(ctx, pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	assert.Equal(t, rewound, current)
	assert.Empty(t, unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID}).MergedCommitID)

	require.NoError(t, git.RemoveRef(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch))
	require.ErrorAs(t, MergeQueuePullRequests(ctx, entries), new(*git.ErrPushOutOfDate))
	assert.False(t, git.IsBranchExist(ctx, pr.BaseRepo, pr.BaseBranch))

	require.NoError(t, db.Insert(ctx, &pull_model.MergeQueueEntry{RepoID: pr.BaseRepoID, BaseBranch: pr.BaseBranch, PullID: pr.ID}))
	defer test.MockVariableValue(&setting.Repository.PullRequest.RetargetChildrenOnMerge, true)()
	require.NoError(t, AdjustPullsCausedByBranchDeleted(ctx, entry.Enqueuer, pr.BaseRepo, pr.BaseBranch))
	unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID, BaseBranch: pr.BaseRepo.DefaultBranch})
}
