// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeQueueRequiredContexts(t *testing.T) {
	assert.Equal(t,
		[]string{"CI / test (merge_group)", "CI / lint (merge_group)", "CI (pull_request) / *"},
		MergeQueueRequiredContexts([]string{"CI / test (pull_request)", "CI / lint (pull_request_target)", "CI (pull_request) / *"}),
	)
}

func TestMergeQueuePullRequestsRejectsChangedBaseOrHead(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadBaseRepo(ctx))
	items := []MergeQueueItem{{PR: pr, HeadSHA: pr.MergeBase}}
	require.ErrorAs(t, MergeQueuePullRequests(ctx, pr.BaseRepo, pr.BaseBranch, items), new(ErrSHADoesNotMatch))

	pr.BaseBranch = "MASTER"
	require.ErrorContains(t, MergeQueuePullRequests(ctx, pr.BaseRepo, pr.BaseBranch, items), "no longer targets MASTER")
}
