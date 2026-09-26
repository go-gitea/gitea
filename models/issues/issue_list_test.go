// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
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

func TestIssueList_GetApprovalCounts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	issue3 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	counts, err := issues_model.IssueList{issue3}.GetApprovalCounts(t.Context())
	require.NoError(t, err)

	countByType := func(typ issues_model.ReviewType) int64 {
		for _, c := range counts[issue3.ID] {
			if c.Type == typ {
				return c.Count
			}
		}
		return 0
	}

	// The comment count mirrors the reviewer sidebar: a reviewer is counted only when
	// their latest non-dismissed review is a comment.
	// Fixture issue 3: user1's latest review is a comment (counted); user5 commented
	// but was later re-requested (not counted).
	assert.EqualValues(t, 1, countByType(issues_model.ReviewTypeComment))

	// A later verdict or re-request supersedes an earlier comment, matching the
	// sidebar (example: comment -> change request shows as change request, not
	// commented). user996 comments then requests changes, user997 comments then is
	// re-requested: neither stays commented, while a bare comment (user998) does.
	require.NoError(t, db.Insert(t.Context(),
		&issues_model.Review{IssueID: issue3.ID, ReviewerID: 996, Type: issues_model.ReviewTypeComment, UpdatedUnix: 946684900},
		&issues_model.Review{IssueID: issue3.ID, ReviewerID: 996, Type: issues_model.ReviewTypeReject, UpdatedUnix: 946684901},
		&issues_model.Review{IssueID: issue3.ID, ReviewerID: 997, Type: issues_model.ReviewTypeComment, UpdatedUnix: 946684902},
		&issues_model.Review{IssueID: issue3.ID, ReviewerID: 997, Type: issues_model.ReviewTypeRequest, UpdatedUnix: 946684903},
		&issues_model.Review{IssueID: issue3.ID, ReviewerID: 998, Type: issues_model.ReviewTypeComment, UpdatedUnix: 946684904},
	))
	counts, err = issues_model.IssueList{issue3}.GetApprovalCounts(t.Context())
	require.NoError(t, err)
	assert.EqualValues(t, 2, countByType(issues_model.ReviewTypeComment)) // user1 + user998
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
