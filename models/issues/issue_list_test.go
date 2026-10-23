// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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

func TestIssueList_LoadIsRead(t *testing.T) {
	// Regression: In("issue_id") was missing the issueIDs argument, causing
	// xorm to generate "0=1" and never mark any issue as read.
	require.NoError(t, unittest.PrepareTestDatabase())

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	// Fixture: uid=1 has is_read=true on issue 1 only.
	issueList := issues_model.IssueList{issue1, issue2}
	require.NoError(t, issueList.LoadIsRead(t.Context(), 1))

	assert.True(t, issue1.IsRead, "issue 1 should be marked read for user 1")
	assert.False(t, issue2.IsRead, "issue 2 should not be marked read for user 1")
}

func TestIssueList_LoadAttributes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Service.EnableTimetracking, true)()
	fixtureIssues := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}
	issueList := issues_model.IssueList{fixtureIssues[0]}
	for i := range db.DefaultMaxInSize {
		issueList = append(issueList, &issues_model.Issue{ID: int64(1000 + i), RepoID: 1, IsPull: true})
	}
	issueList = append(issueList, fixtureIssues[1:]...)

	require.NoError(t, issueList.LoadAttributes(t.Context()))
	require.NoError(t, issueList.LoadAttachments(t.Context()))
	require.NoError(t, issueList.LoadComments(t.Context()))
	assert.Len(t, fixtureIssues[0].Labels, 1)
	assert.Len(t, fixtureIssues[1].Labels, 2)
	assert.Len(t, fixtureIssues[0].Assignees, 1)
	assert.Empty(t, fixtureIssues[1].Assignees)
	assert.Len(t, fixtureIssues[0].Attachments, 1)
	assert.Len(t, fixtureIssues[1].Attachments, 1)
	assert.Len(t, fixtureIssues[2].Attachments, 2)
	assert.Len(t, fixtureIssues[0].Comments, 3)
	assert.Len(t, fixtureIssues[1].Comments, 4)
	assert.Len(t, fixtureIssues[2].Comments, 1)
	for _, issue := range fixtureIssues {
		assert.Equal(t, issue.RepoID, issue.Repo.ID)
		for _, label := range issue.Labels {
			assert.Equal(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: label.ID}), label)
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
			require.NotNil(t, issue.PullRequest)
			assert.Equal(t, issue.ID, issue.PullRequest.IssueID)
			assert.Same(t, issue, issue.PullRequest.Issue)
		}
		for _, attachment := range issue.Attachments {
			assert.Equal(t, issue.ID, attachment.IssueID)
		}
		for _, comment := range issue.Comments {
			assert.Equal(t, issue.ID, comment.IssueID)
		}
		if issue.ID == int64(1) {
			assert.Equal(t, int64(400), issue.TotalTrackedTime)
		}
		if issue.ID == 1 || issue.ID == 2 {
			require.Len(t, issue.Projects, 1)
			assert.Equal(t, int64(1), issue.Projects[0].ID)
		} else {
			assert.Empty(t, issue.Projects)
		}
	}
	assert.Equal(t, int64(3682), fixtureIssues[1].TotalTrackedTime)

	require.NoError(t, issueList.LoadDiscussComments(t.Context()))
	assert.Len(t, fixtureIssues[0].Comments, 2)
	assert.Empty(t, fixtureIssues[1].Comments)
	assert.Len(t, fixtureIssues[2].Comments, 1)
	for _, issue := range fixtureIssues {
		for _, comment := range issue.Comments {
			assert.Equal(t, issue.ID, comment.IssueID)
			assert.Equal(t, issues_model.CommentTypeComment, comment.Type)
		}
	}
}
