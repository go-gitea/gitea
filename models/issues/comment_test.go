// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	now := time.Now().Unix()
	comment, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeComment,
		Doer:    doer,
		Repo:    repo,
		Issue:   issue,
		Content: "Hello",
	})
	assert.NoError(t, err)
	then := time.Now().Unix()

	assert.Equal(t, issues_model.CommentTypeComment, comment.Type)
	assert.Equal(t, "Hello", comment.Content)
	assert.Equal(t, issue.ID, comment.IssueID)
	assert.Equal(t, doer.ID, comment.PosterID)
	unittest.AssertInt64InRange(t, now, then, int64(comment.CreatedUnix))
	unittest.AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	updatedIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
	unittest.AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestLoadAssigneeUserAndTeam_DeletedTeamBecomesGhostTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 15})
	comment := &issues_model.Comment{
		Type:           issues_model.CommentTypeAssignees,
		IssueID:        issue.ID,
		AssigneeTeamID: 999999, // non-existing team ID
	}
	assert.NoError(t, comment.LoadAssigneeUserAndTeam(t.Context()))
	assert.NotNil(t, comment.AssigneeTeam)
	assert.EqualValues(t, -1, comment.AssigneeTeam.ID)
}

func TestCommentListLoadAttributesMixedAssignees(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	first := &issues_model.Comment{
		ID: 1, PosterID: 1, IssueID: 1, LabelID: 1, MilestoneID: 1, OldMilestoneID: 1, AssigneeID: 1, DependentIssueID: 2,
	}
	last := &issues_model.Comment{
		ID: 2, PosterID: 2, IssueID: 4, LabelID: 2, MilestoneID: 3, OldMilestoneID: 3, AssigneeID: 2, DependentIssueID: 1,
	}
	comments := issues_model.CommentList{
		{AssigneeTeamID: 8},
		{AssigneeID: 999999},
		first,
	}
	for i := range db.DefaultMaxInSize {
		id := unittest.NonexistentID - int64(i)
		comments = append(comments, &issues_model.Comment{
			ID: id, PosterID: id, IssueID: id, LabelID: id, MilestoneID: id, OldMilestoneID: id, AssigneeID: id, DependentIssueID: id,
		})
	}
	comments = append(comments, last)

	require.NoError(t, comments.LoadAttributes(t.Context()))
	assert.Nil(t, comments[0].Assignee)
	assert.Zero(t, comments[0].AssigneeID)
	assert.Equal(t, user_model.GhostUserID, comments[1].AssigneeID)
	assert.Equal(t, user_model.GhostUserID, comments[1].Assignee.ID)
	for _, comment := range []*issues_model.Comment{first, last} {
		require.NotNil(t, comment.Poster)
		assert.Equal(t, comment.PosterID, comment.Poster.ID)
		require.NotNil(t, comment.Issue)
		assert.Equal(t, comment.IssueID, comment.Issue.ID)
		require.NotNil(t, comment.Label)
		assert.Equal(t, comment.LabelID, comment.Label.ID)
		require.NotNil(t, comment.Milestone)
		assert.Equal(t, comment.MilestoneID, comment.Milestone.ID)
		require.NotNil(t, comment.OldMilestone)
		assert.Equal(t, comment.OldMilestoneID, comment.OldMilestone.ID)
		require.NotNil(t, comment.Assignee)
		assert.Equal(t, comment.AssigneeID, comment.Assignee.ID)
		require.NotNil(t, comment.DependentIssue)
		assert.Equal(t, comment.DependentIssueID, comment.DependentIssue.ID)
		require.NotNil(t, comment.DependentIssue.Repo)
		assert.Equal(t, comment.DependentIssue.RepoID, comment.DependentIssue.Repo.ID)
		require.Len(t, comment.Attachments, 2)
		for _, attachment := range comment.Attachments {
			assert.Equal(t, comment.ID, attachment.CommentID)
		}
	}
	missing := comments[3]
	assert.Equal(t, user_model.GhostUserID, missing.Poster.ID)
	assert.Equal(t, user_model.GhostUserID, missing.Assignee.ID)
	assert.Nil(t, missing.Issue)
	assert.Nil(t, missing.Label)
	assert.Nil(t, missing.Milestone)
	assert.Nil(t, missing.OldMilestone)
	assert.Nil(t, missing.DependentIssue)
	assert.Empty(t, missing.Attachments)
}

func Test_UpdateCommentAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	attachment := repo_model.Attachment{
		RepoID: issue.RepoID, // must match the comment's repo, else the cross-repo guard rejects it
		Name:   "test.txt",
	}
	assert.NoError(t, db.Insert(t.Context(), &attachment))

	err := issues_model.UpdateCommentAttachments(t.Context(), comment, []string{attachment.UUID})
	assert.NoError(t, err)

	attachment2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: attachment.ID})
	assert.Equal(t, attachment.Name, attachment2.Name)
	assert.Equal(t, comment.ID, attachment2.CommentID)
	assert.Equal(t, comment.IssueID, attachment2.IssueID)
}

func TestFetchCodeComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	res, err := issues_model.FetchCodeComments(t.Context(), issue, user, false)
	assert.NoError(t, err)
	assert.Contains(t, res, "README.md")
	assert.Contains(t, res["README.md"], int64(4))
	assert.Len(t, res["README.md"][4], 1)
	assert.Equal(t, int64(4), res["README.md"][4][0].ID)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	res, err = issues_model.FetchCodeComments(t.Context(), issue, user2, false)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestAsCommentType(t *testing.T) {
	assert.Equal(t, issues_model.CommentTypeComment, issues_model.CommentType(0))
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType(""))
	assert.Equal(t, issues_model.CommentTypeUndefined, issues_model.AsCommentType("nonsense"))
	assert.Equal(t, issues_model.CommentTypeComment, issues_model.AsCommentType("comment"))
	assert.Equal(t, issues_model.CommentTypePRUnScheduledToAutoMerge, issues_model.AsCommentType("pull_cancel_scheduled_merge"))
}

func TestMigrate_InsertIssueComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	_ = issue.LoadRepo(t.Context())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})
	reaction := &issues_model.Reaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	comment := &issues_model.Comment{
		PosterID:  owner.ID,
		Poster:    owner,
		IssueID:   issue.ID,
		Issue:     issue,
		Reactions: []*issues_model.Reaction{reaction},
	}

	err := issues_model.InsertIssueComments(t.Context(), []*issues_model.Comment{comment})
	assert.NoError(t, err)

	issueModified := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.Equal(t, issue.NumComments+1, issueModified.NumComments)

	unittest.CheckConsistencyFor(t, &issues_model.Issue{})
}

func Test_UpdateIssueNumComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	assert.NoError(t, issues_model.UpdateIssueNumComments(t.Context(), issue2.ID))
	issue2 = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	assert.Equal(t, 1, issue2.NumComments)
}
