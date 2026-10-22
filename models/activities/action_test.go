// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"fmt"
	"path"
	"strings"
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
	count, err := activities_model.CountActionCreatedUnixString(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = activities_model.FixActionCreatedUnixString(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = activities_model.CountActionCreatedUnixString(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	count, err = activities_model.FixActionCreatedUnixString(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)

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

func TestGetCommentPreview(t *testing.T) {
	for _, entry := range []struct {
		name     string
		opType   activities_model.ActionType
		content  string
		comment  *issue_model.Comment
		expected string
	}{
		{"live comment preferred over snapshot", activities_model.ActionCommentIssue, "1|stale snapshot", &issue_model.Comment{Content: "fresh content"}, "fresh content"},
		{"snapshot fallback when comment not loaded", activities_model.ActionCommentIssue, "1|snapshot body", nil, "snapshot body"},
		{"dismiss review reads field 2 as fallback", activities_model.ActionPullReviewDismissed, "5|reviewer|dismiss reason", nil, "dismiss reason"},
		{"dismiss review prefers live comment", activities_model.ActionPullReviewDismissed, "5|reviewer|old reason", &issue_model.Comment{Content: "updated reason"}, "updated reason"},
		{"live comment uses meaningful prose excerpt", activities_model.ActionCommentPull, "1|stale snapshot", &issue_model.Comment{Content: "```suggestion\nreplacement\n```\n\n" + strings.Repeat("a", 300)}, strings.Repeat("a", 197) + "…"},
		{"empty live comment preferred over snapshot", activities_model.ActionCommentIssue, "1|old content", &issue_model.Comment{}, ""},
	} {
		t.Run(entry.name, func(t *testing.T) {
			action := &activities_model.Action{
				OpType:         entry.opType,
				Content:        entry.content,
				Comment:        entry.comment,
				CanReadComment: true,
			}
			assert.Equal(t, entry.expected, action.GetCommentPreview(t.Context()))
		})
	}
}
