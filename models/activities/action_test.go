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
	"code.gitea.io/gitea/modules/test"

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
	defer test.MockVariableValue(&setting.AppURL, "https://try.gitea.io/suburl/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/suburl")()
	expected := path.Join(setting.AppSubURL, owner.Name, repo.Name)
	assert.Equal(t, expected, action.GetRepoLink(db.DefaultContext))
	assert.Equal(t, repo.HTMLURL(), action.GetRepoAbsoluteLink(db.DefaultContext))
	assert.Equal(t, comment.HTMLURL(db.DefaultContext), action.GetCommentHTMLURL(db.DefaultContext))
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
	_, err := db.GetEngine(db.DefaultContext).Exec(`UPDATE action SET created_unix = '' WHERE id = ?`, id)
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
