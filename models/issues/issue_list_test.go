// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestIssueList_LoadRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	repos, err := issueList.LoadRepositories(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	for _, issue := range issueList {
		assert.EqualValues(t, issue.RepoID, issue.Repo.ID)
	}
}

func TestIssueList_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.EnableTimetracking = true
	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	assert.NoError(t, issueList.LoadAttributes())
	for _, issue := range issueList {
		assert.EqualValues(t, issue.RepoID, issue.Repo.ID)
		for _, label := range issue.Labels {
			assert.EqualValues(t, issue.RepoID, label.RepoID)
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
		}
		if issue.PosterID > 0 {
			assert.EqualValues(t, issue.PosterID, issue.Poster.ID)
		}
		if issue.AssigneeID > 0 {
			assert.EqualValues(t, issue.AssigneeID, issue.Assignee.ID)
		}
		if issue.MilestoneID > 0 {
			assert.EqualValues(t, issue.MilestoneID, issue.Milestone.ID)
		}
		if issue.IsPull {
			assert.EqualValues(t, issue.ID, issue.PullRequest.IssueID)
		}
		for _, attachment := range issue.Attachments {
			assert.EqualValues(t, issue.ID, attachment.IssueID)
		}
		for _, comment := range issue.Comments {
			assert.EqualValues(t, issue.ID, comment.IssueID)
		}
		if issue.ID == int64(1) {
			assert.Equal(t, int64(400), issue.TotalTrackedTime)
			assert.NotNil(t, issue.Project)
			assert.Equal(t, int64(1), issue.Project.ID)
		} else {
			assert.Nil(t, issue.Project)
		}
	}
}
