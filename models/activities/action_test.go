// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"fmt"
	"path"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issue_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAction_GetRepoPath(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	action := &activities_model.Action{RepoID: repo.ID}
	assert.Equal(t, path.Join(owner.Name, repo.Name), action.GetRepoPath(t.Context()))
}

func TestAction_GetRepoLink(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	comment := unittest.AssertExistsAndLoadBean(t, &issue_model.Comment{ID: 2})
	action := &activities_model.Action{RepoID: repo.ID, CommentID: comment.ID}
	defer test.MockVariableValue(&setting.AppURL, "https://try.gitea.io/suburl/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/suburl")()
	expected := path.Join(setting.AppSubURL, owner.Name, repo.Name)
	assert.Equal(t, expected, action.GetRepoLink(t.Context()))
	assert.Equal(t, repo.HTMLURL(), action.GetRepoAbsoluteLink(t.Context()))
	assert.Equal(t, comment.HTMLURL(t.Context()), action.GetCommentHTMLURL(t.Context()))
}

func TestActivityReadable(t *testing.T) {
	tt := []struct {
		desc   string
		user   *user_model.User
		doer   *user_model.User
		result bool
	}{{
		desc:   "user should see own activity",
		user:   &user_model.User{ID: 1},
		doer:   &user_model.User{ID: 1},
		result: true,
	}, {
		desc:   "anon should see activity if public",
		user:   &user_model.User{ID: 1},
		result: true,
	}, {
		desc:   "anon should NOT see activity",
		user:   &user_model.User{ID: 1, KeepActivityPrivate: true},
		result: false,
	}, {
		desc:   "user should see own activity if private too",
		user:   &user_model.User{ID: 1, KeepActivityPrivate: true},
		doer:   &user_model.User{ID: 1},
		result: true,
	}, {
		desc:   "other user should NOT see activity",
		user:   &user_model.User{ID: 1, KeepActivityPrivate: true},
		doer:   &user_model.User{ID: 2},
		result: false,
	}, {
		desc:   "admin should see activity",
		user:   &user_model.User{ID: 1, KeepActivityPrivate: true},
		doer:   &user_model.User{ID: 2, IsAdmin: true},
		result: true,
	}}
	for _, test := range tt {
		assert.Equal(t, test.result, activities_model.ActivityReadable(test.user, test.doer), test.desc)
	}
}

func TestConsistencyUpdateAction(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip("Test is only for SQLite database.")
	}
	assert.NoError(t, unittest.PrepareTestDatabase())
	id := 8
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ID: int64(id),
	})
	_, err := db.GetEngine(t.Context()).Exec(`UPDATE action SET created_unix = '' WHERE id = ?`, id)
	assert.NoError(t, err)
	actions := make([]*activities_model.Action, 0, 1)
	//
	// XORM returns an error when created_unix is a string
	//
	err = db.GetEngine(t.Context()).Where("id = ?", id).Find(&actions)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "type string to a int64: invalid syntax")
	}
	//
	// Get rid of incorrectly set created_unix
	//
	for _, wantCount := range []int{1, 0} {
		count, err := activities_model.CountActionCreatedUnixString(t.Context())
		assert.NoError(t, err)
		assert.EqualValues(t, wantCount, count)
		count, err = activities_model.FixActionCreatedUnixString(t.Context())
		assert.NoError(t, err)
		assert.EqualValues(t, wantCount, count)
	}

	//
	// XORM must be happy now
	//
	assert.NoError(t, db.GetEngine(t.Context()).Where("id = ?", id).Find(&actions))
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

func TestDeleteIssueActions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// load an issue
	issue := unittest.AssertExistsAndLoadBean(t, &issue_model.Issue{ID: 4})
	assert.NotEqual(t, issue.ID, issue.Index) // it needs to use different ID/Index to test the DeleteIssueActions to delete some actions by IssueIndex

	// insert a comment
	err := db.Insert(t.Context(), &issue_model.Comment{Type: issue_model.CommentTypeComment, IssueID: issue.ID})
	assert.NoError(t, err)
	comment := unittest.AssertExistsAndLoadBean(t, &issue_model.Comment{Type: issue_model.CommentTypeComment, IssueID: issue.ID})

	// truncate action table and insert some actions
	err = db.TruncateBeans(t.Context(), &activities_model.Action{})
	assert.NoError(t, err)
	err = db.Insert(t.Context(), &activities_model.Action{
		OpType:    activities_model.ActionCommentIssue,
		CommentID: comment.ID,
	})
	assert.NoError(t, err)
	err = db.Insert(t.Context(), &activities_model.Action{
		OpType:  activities_model.ActionCreateIssue,
		RepoID:  issue.RepoID,
		Content: fmt.Sprintf("%d|content...", issue.Index),
	})
	assert.NoError(t, err)

	// assert that the actions exist, then delete them
	unittest.AssertCount(t, &activities_model.Action{}, 2)
	assert.NoError(t, activities_model.DeleteIssueActions(t.Context(), issue.RepoID, issue.ID, issue.Index))
	unittest.AssertCount(t, &activities_model.Action{}, 0)
}

func TestGetFeedsConfidential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	hiddenViewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	insertIssue := func(index int64, confidential bool) *issue_model.Issue {
		issue := &issue_model.Issue{RepoID: repo.ID, Repo: repo, Index: index, PosterID: owner.ID, Title: "t", IsConfidential: confidential}
		require.NoError(t, db.Insert(ctx, issue))
		require.NoError(t, db.Insert(ctx, &activities_model.Action{
			UserID: owner.ID, ActUserID: owner.ID, RepoID: repo.ID, OpType: activities_model.ActionCreateIssue,
			Content: fmt.Sprintf("%d|t", index),
		}))
		return issue
	}
	secret := insertIssue(9001, true)
	insertIssue(9002, false)

	feedContents := func(opts activities_model.GetFeedsOptions) []string {
		actions, count, err := activities_model.GetFeeds(ctx, opts)
		require.NoError(t, err)
		contents := make([]string, 0, len(actions))
		for _, action := range actions {
			contents = append(contents, action.Content)
		}
		assert.Equal(t, int64(len(contents)), count)
		return contents
	}
	ownDashboard := func(actor *user_model.User) activities_model.GetFeedsOptions {
		return activities_model.GetFeedsOptions{RequestedUser: actor, Actor: actor, IncludePrivate: true}
	}
	ownerProfile := func(actor *user_model.User) activities_model.GetFeedsOptions {
		return activities_model.GetFeedsOptions{RequestedUser: owner, Actor: actor, IncludePrivate: true}
	}
	repoFeed := func(actor *user_model.User) activities_model.GetFeedsOptions {
		return activities_model.GetFeedsOptions{RequestedRepo: repo, Actor: actor, IncludePrivate: true}
	}

	// the owner is not hidden from the issue, but a hidden viewer viewing own dashboard must still be filtered
	assert.Contains(t, feedContents(ownDashboard(owner)), "9001|t")
	assert.Contains(t, feedContents(ownerProfile(admin)), "9001|t")
	for _, opts := range []activities_model.GetFeedsOptions{ownerProfile(hiddenViewer), repoFeed(hiddenViewer), repoFeed(nil)} {
		contents := feedContents(opts)
		assert.NotContains(t, contents, "9001|t")
		assert.Contains(t, contents, "9002|t")
	}

	// own-dashboard branch: actions of the hidden viewer's own dashboard about the confidential issue
	require.NoError(t, db.Insert(ctx, &activities_model.Action{
		UserID: hiddenViewer.ID, ActUserID: owner.ID, RepoID: repo.ID, OpType: activities_model.ActionCommentIssue, Content: "9001|t",
	}))
	assert.NotContains(t, feedContents(ownDashboard(hiddenViewer)), "9001|t")
	assert.Contains(t, feedContents(repoFeed(admin)), "9001|t")

	// publishing shows the entries again without touching the rows
	_, err := db.GetEngine(ctx).ID(secret.ID).Cols("is_confidential").Update(&issue_model.Issue{IsConfidential: false})
	require.NoError(t, err)
	assert.Contains(t, feedContents(repoFeed(hiddenViewer)), "9001|t")
}

func TestGetFeedsManyHiddenConfidentialIssues(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	hiddenViewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})

	const hiddenCount = 500
	for index := int64(9100); index < 9100+hiddenCount; index++ {
		require.NoError(t, db.Insert(ctx, &issue_model.Issue{RepoID: repo.ID, Repo: repo, Index: index, PosterID: 2, Title: "t", IsConfidential: true}))
	}
	_, _, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{RequestedRepo: repo, Actor: hiddenViewer, IncludePrivate: true})
	require.NoError(t, err)
}
