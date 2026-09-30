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

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadIssue(t.Context()))
	_, err := ScheduleAutoMerge(t.Context(), admin, pr, repo_model.MergeStyleSquash, "title", false)
	require.NoError(t, err)
	assert.Equal(t, "squash", unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRScheduledToAutoMerge}).Content)

	notifier.PullRequestSynchronized(t.Context(), nonWriter, pr, "", "")
	notifier.PullRequestChangeTargetBranch(t.Context(), admin, pr, "")
	unittest.AssertExistsAndLoadBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	notifier.IssueChangeStatus(t.Context(), admin, "", pr.Issue, nil, true)
	assert.Equal(t, "closed", unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge}).Content)

	pr.Flow = issues_model.PullRequestFlowAGit
	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, pr.ID, repo_model.MergeStyleMerge, "title", false))
	notifier.PullRequestSynchronized(t.Context(), nonWriter, pr, "", "")
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})

	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, pr.ID, repo_model.MergeStyleMerge, "title", false))
	oldTitle := pr.Issue.Title
	pr.Issue.Title = "WIP: " + oldTitle
	notifier.IssueChangeTitle(t.Context(), admin, pr.Issue, oldTitle)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, Content: "work_in_progress"})

	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, pr.ID, repo_model.MergeStyleMerge, "title", false))
	notifier.PullRequestChangeTargetBranch(t.Context(), nonWriter, pr, "")
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, Content: "base_changed"})

	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), admin, pr.ID, repo_model.MergeStyleMerge, "title", false))
	pr.Issue.PosterID = nonWriter.ID
	require.NoError(t, CancelScheduledAutoMerge(t.Context(), nonWriter, pr, access_model.Permission{}))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
}
