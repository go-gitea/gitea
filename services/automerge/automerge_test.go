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
	"gitea.dev/modules/git"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/test"
	"gitea.dev/services/automergequeue"
	issue_service "gitea.dev/services/issue"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	notify_service.RegisterNotifier(NewNotifier())
	unittest.MainTest(m)
}

func TestAutoMergeDisabledOrCanceled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&automergequeue.AddToQueue, func(_item automergequeue.AutoMergeItem) {})()
	var checkedPullID int64
	defer test.MockVariableValue(&pull_service.AddPullRequestToCheckQueue, func(pullID int64) { checkedPullID = pullID })()
	writer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	nonWriter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	pr.Status = issues_model.PullRequestStatusChecking
	require.NoError(t, pr.UpdateCols(t.Context(), "status"))
	require.NoError(t, ScheduleAutoMerge(t.Context(), writer, pr, repo_model.MergeStyleSquash, "title", false))
	assert.Equal(t, pr.ID, checkedPullID)
	assert.Equal(t, "squash", unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRScheduledToAutoMerge}).Content)

	require.NoError(t, pr.Issue.LoadRepo(t.Context()))
	require.NoError(t, pull_service.ChangeTargetBranch(t.Context(), pr, writer, "feature/1"))
	assert.Equal(t, "feature/1", unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID}).BaseBranch)
	unittest.AssertExistsAndLoadBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	require.NoError(t, pull_service.ChangeTargetBranch(t.Context(), pr, nonWriter, "master"))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, Content: "base_changed"})

	require.NoError(t, ScheduleAutoMerge(t.Context(), writer, pr, repo_model.MergeStyleMerge, "title", false))
	require.NoError(t, issue_service.CloseIssue(t.Context(), pr.Issue, writer, ""))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, Content: "closed"})

	require.NoError(t, issue_service.ReopenIssue(t.Context(), pr.Issue, writer, ""))
	require.NoError(t, ScheduleAutoMerge(t.Context(), writer, pr, repo_model.MergeStyleMerge, "title", false))
	require.NoError(t, issue_service.ChangeTitle(t.Context(), pr.Issue, writer, "WIP: "+pr.Issue.Title))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, Content: "work_in_progress"})

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	require.NoError(t, pr.LoadHeadRepo(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	writer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: pr.BaseRepo.OwnerID})
	pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
	require.NoError(t, ScheduleAutoMerge(t.Context(), writer, pr, repo_model.MergeStyleMerge, "title", false))
	pull_service.UpdatePullsRefs(t.Context(), pusher, pr.HeadRepo, &repo_module.PushUpdateOptions{RefFullName: git.RefNameFromBranch(pr.HeadBranch)})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRUnScheduledToAutoMerge, PosterID: pusher.ID, Content: "pushed_by_non_writer"})

	require.NoError(t, ScheduleAutoMerge(t.Context(), writer, pr, repo_model.MergeStyleMerge, "title", false))
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: pr.Issue.PosterID})
	require.NoError(t, CancelScheduledAutoMerge(t.Context(), poster, pr, access_model.Permission{}))
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
}
