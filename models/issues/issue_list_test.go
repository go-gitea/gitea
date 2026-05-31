// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueList_LoadRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	repos, err := issueList.LoadRepositories(t.Context())
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	for _, issue := range issueList {
		assert.Equal(t, issue.RepoID, issue.Repo.ID)
	}
}

func TestIssueList_LoadClosedBy(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	closer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	_, err := issues_model.CloseIssue(t.Context(), issue, closer)
	require.NoError(t, err)

	issues := issues_model.IssueList{issue}
	require.NoError(t, issues.LoadClosedBy(t.Context()))
	require.NotNil(t, issue.ClosedBy)
	assert.Equal(t, closer.ID, issue.ClosedBy.ID)
}

func TestIssueList_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.EnableTimetracking = true
	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	assert.NoError(t, issueList.LoadAttributes(t.Context()))
	for _, issue := range issueList {
		assert.Equal(t, issue.RepoID, issue.Repo.ID)
		for _, label := range issue.Labels {
			assert.Equal(t, issue.RepoID, label.RepoID)
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
		}
		if issue.PosterID > 0 {
			assert.Equal(t, issue.PosterID, issue.Poster.ID)
		}
		if issue.AssigneeID > 0 {
			assert.Equal(t, issue.AssigneeID, issue.Assignee.ID)
		}
		if issue.MilestoneID > 0 {
			assert.Equal(t, issue.MilestoneID, issue.Milestone.ID)
		}
		if issue.IsPull {
			assert.Equal(t, issue.ID, issue.PullRequest.IssueID)
		}
		for _, attachment := range issue.Attachments {
			assert.Equal(t, issue.ID, attachment.IssueID)
		}
		for _, comment := range issue.Comments {
			assert.Equal(t, issue.ID, comment.IssueID)
		}
		if issue.ID == int64(1) {
			assert.Equal(t, int64(400), issue.TotalTrackedTime)
			assert.NotEmpty(t, issue.Projects)
			assert.Equal(t, int64(1), issue.Projects[0].ID)
		} else {
			assert.Empty(t, issue.Projects)
		}
	}
}
