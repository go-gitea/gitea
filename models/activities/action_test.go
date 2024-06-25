// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"fmt"
	"path"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestAction_GetRepoPath(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	action := &activities_model.Action{RepoID: repo.ID}
	assert.Equal(t, path.Join(owner.Name, repo.Name), action.GetRepoPath(db.DefaultContext))
}

func TestAction_GetRepoLink(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	comment := unittest.AssertExistsAndLoadBean(t, &issue_model.Comment{ID: 2})
	action := &activities_model.Action{RepoID: repo.ID, CommentID: comment.ID}
	setting.AppSubURL = "/suburl"
	expected := path.Join(setting.AppSubURL, owner.Name, repo.Name)
	assert.Equal(t, expected, action.GetRepoLink(db.DefaultContext))
	assert.Equal(t, repo.HTMLURL(), action.GetRepoAbsoluteLink(db.DefaultContext))
	assert.Equal(t, comment.HTMLURL(db.DefaultContext), action.GetCommentHTMLURL(db.DefaultContext))
}

func TestGetFeeds(t *testing.T) {
	// test with an individual user
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	actions, count, err := activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 1, actions[0].ID)
		assert.EqualValues(t, user.ID, actions[0].UserID)
	}
	assert.Equal(t, int64(1), count)

	actions, count, err = activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
	assert.Equal(t, int64(0), count)
}

func TestGetFeedsForRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	privRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	pubRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})

	// private repo & no login
	actions, count, err := activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
	assert.Equal(t, int64(0), count)

	// public repo & no login
	actions, count, err = activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)

	// private repo and login
	actions, count, err = activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)

	// public repo & login
	actions, count, err = activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, int64(1), count)
}

func TestGetFeeds2(t *testing.T) {
	// test with an organization user
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	actions, count, err := activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   org,
		Actor:           user,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 2, actions[0].ID)
		assert.EqualValues(t, org.ID, actions[0].UserID)
	}
	assert.Equal(t, int64(1), count)

	actions, count, err = activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:   org,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
	assert.Equal(t, int64(0), count)
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

func TestNotifyWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	action := &activities_model.Action{
		ActUserID: 8,
		RepoID:    1,
		OpType:    activities_model.ActionStarRepo,
	}
	assert.NoError(t, activities_model.NotifyWatchers(db.DefaultContext, action))

	// One watchers are inactive, thus action is only created for user 8, 1, 4, 11
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: action.ActUserID,
		UserID:    8,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: action.ActUserID,
		UserID:    1,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: action.ActUserID,
		UserID:    4,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ActUserID: action.ActUserID,
		UserID:    11,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
}

func TestGetFeedsCorrupted(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		ID:     8,
		RepoID: 1700,
	})

	actions, count, err := activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedUser:  user,
		Actor:          user,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
	assert.Equal(t, int64(0), count)
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
	_, err := db.GetEngine(db.DefaultContext).Exec(`UPDATE action SET created_unix = "" WHERE id = ?`, id)
	assert.NoError(t, err)
	actions := make([]*activities_model.Action, 0, 1)
	//
	// XORM returns an error when created_unix is a string
	//
	err = db.GetEngine(db.DefaultContext).Where("id = ?", id).Find(&actions)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "type string to a int64: invalid syntax")
	}
	//
	// Get rid of incorrectly set created_unix
	//
	count, err := activities_model.CountActionCreatedUnixString(db.DefaultContext)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = activities_model.FixActionCreatedUnixString(db.DefaultContext)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = activities_model.CountActionCreatedUnixString(db.DefaultContext)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	count, err = activities_model.FixActionCreatedUnixString(db.DefaultContext)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)

	//
	// XORM must be happy now
	//
	assert.NoError(t, db.GetEngine(db.DefaultContext).Where("id = ?", id).Find(&actions))
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

func TestDeleteIssueActions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// load an issue
	issue := unittest.AssertExistsAndLoadBean(t, &issue_model.Issue{ID: 4})
	assert.NotEqualValues(t, issue.ID, issue.Index) // it needs to use different ID/Index to test the DeleteIssueActions to delete some actions by IssueIndex

	// insert a comment
	err := db.Insert(db.DefaultContext, &issue_model.Comment{Type: issue_model.CommentTypeComment, IssueID: issue.ID})
	assert.NoError(t, err)
	comment := unittest.AssertExistsAndLoadBean(t, &issue_model.Comment{Type: issue_model.CommentTypeComment, IssueID: issue.ID})

	// truncate action table and insert some actions
	err = db.TruncateBeans(db.DefaultContext, &activities_model.Action{})
	assert.NoError(t, err)
	err = db.Insert(db.DefaultContext, &activities_model.Action{
		OpType:    activities_model.ActionCommentIssue,
		CommentID: comment.ID,
	})
	assert.NoError(t, err)
	err = db.Insert(db.DefaultContext, &activities_model.Action{
		OpType:  activities_model.ActionCreateIssue,
		RepoID:  issue.RepoID,
		Content: fmt.Sprintf("%d|content...", issue.Index),
	})
	assert.NoError(t, err)

	// assert that the actions exist, then delete them
	unittest.AssertCount(t, &activities_model.Action{}, 2)
	assert.NoError(t, activities_model.DeleteIssueActions(db.DefaultContext, issue.RepoID, issue.ID, issue.Index))
	unittest.AssertCount(t, &activities_model.Action{}, 0)
}

func TestRepoActions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_ = db.TruncateBeans(db.DefaultContext, &activities_model.Action{})
	for i := 0; i < 3; i++ {
		_ = db.Insert(db.DefaultContext, &activities_model.Action{
			UserID:    2 + int64(i),
			ActUserID: 2,
			RepoID:    repo.ID,
			OpType:    activities_model.ActionCommentIssue,
		})
	}
	count, _ := db.Count[activities_model.Action](db.DefaultContext, &db.ListOptions{})
	assert.EqualValues(t, 3, count)
	actions, _, err := activities_model.GetFeeds(db.DefaultContext, activities_model.GetFeedsOptions{
		RequestedRepo: repo,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
}
