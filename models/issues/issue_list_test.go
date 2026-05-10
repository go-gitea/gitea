// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

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

func TestIssueListLoadProjectsWithColumns(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Fixture facts (from models/fixtures/project_issue.yml + project_board.yml):
	// - Issue 1 is in project 1, column 1 ("To Do")
	// - Issue 5 is in project 1, column 3 ("Done")
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue5 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5})

	list := issues_model.IssueList{issue1, issue5}
	require.NoError(t, list.LoadProjects(t.Context()))

	require.Len(t, issue1.LoadedProjects, 1)
	assert.Equal(t, int64(1), issue1.LoadedProjects[0].Project.ID)
	assert.Equal(t, int64(1), issue1.LoadedProjects[0].ColumnID)
	assert.Equal(t, "To Do", issue1.LoadedProjects[0].ColumnTitle)

	require.Len(t, issue5.LoadedProjects, 1)
	assert.Equal(t, int64(1), issue5.LoadedProjects[0].Project.ID)
	assert.Equal(t, int64(3), issue5.LoadedProjects[0].ColumnID)
	assert.Equal(t, "Done", issue5.LoadedProjects[0].ColumnTitle)

	// Issue.Projects should also be populated (web-UI fallback).
	require.Len(t, issue1.Projects, 1)
	assert.Equal(t, int64(1), issue1.Projects[0].ID)
}
