// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/test"
	"gitea.dev/services/automergequeue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestAutoMergeDisabledOrCanceled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&automergequeue.AddToQueue, func(automergequeue.AutoMergeItem) {})()
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	nonWriter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	notifier := &automergeNotifier{}

	samePull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, samePull.LoadIssue(t.Context()))
	_, err := ScheduleAutoMerge(t.Context(), admin, samePull, repo_model.MergeStyleSquash, "title", false)
	require.NoError(t, err)
	assert.Equal(t, repo_model.MergeStyleSquash, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: samePull.IssueID, Type: issues_model.CommentTypePRScheduledToAutoMerge}).CommentMetaData.MergeStyle)

	notifier.PullRequestPushCommits(t.Context(), nonWriter, samePull, nil)
	notifier.PullRequestChangeTargetBranch(t.Context(), admin, samePull, "")
	unittest.AssertExistsAndLoadBean(t, &pull_model.AutoMerge{PullID: samePull.ID})
	notifier.IssueChangeStatus(t.Context(), admin, "", samePull.Issue, nil, true)
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: samePull.ID})
	assert.Equal(t, "closed", unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: samePull.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge}).CommentMetaData.AutoMergeDisabledReason)

	samePull.Flow = issues_model.PullRequestFlowAGit
	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, samePull.ID, repo_model.MergeStyleMerge, "title", false))
	notifier.PullRequestPushCommits(t.Context(), nonWriter, samePull, nil)
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: samePull.ID})

	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, samePull.ID, repo_model.MergeStyleMerge, "title", false))
	oldTitle := samePull.Issue.Title
	samePull.Issue.Title = "WIP: " + oldTitle
	notifier.IssueChangeTitle(t.Context(), admin, samePull.Issue, oldTitle)
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: samePull.ID})

	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, samePull.ID, repo_model.MergeStyleMerge, "title", false))
	samePull.Issue.PosterID = nonWriter.ID
	require.NoError(t, CancelScheduledAutoMerge(t.Context(), nonWriter, samePull, access_model.Permission{}))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: samePull.ID})
}
