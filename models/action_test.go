// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestAction_GetRepoPath(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
	action := &Action{RepoID: repo.ID}
	assert.Equal(t, path.Join(owner.Name, repo.Name), action.GetRepoPath())
}

func TestAction_GetRepoLink(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
	action := &Action{RepoID: repo.ID}
	setting.AppSubURL = "/suburl"
	expected := path.Join(setting.AppSubURL, owner.Name, repo.Name)
	assert.Equal(t, expected, action.GetRepoLink())
}

func TestGetFeeds(t *testing.T) {
	// test with an individual user
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	actions, err := GetFeeds(db.DefaultContext, GetFeedsOptions{
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

	actions, err = GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
}

func TestGetFeedsForRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	privRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	pubRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8}).(*repo_model.Repository)

	// private repo & no login
	actions, err := GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)

	// public repo & no login
	actions, err = GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)

	// private repo and login
	actions, err = GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedRepo:  privRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)

	// public repo & login
	actions, err = GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedRepo:  pubRepo,
		IncludePrivate: true,
		Actor:          user,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
}

func TestGetFeeds2(t *testing.T) {
	// test with an organization user
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	actions, err := GetFeeds(db.DefaultContext, GetFeedsOptions{
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

	actions, err = GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedUser:   org,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: false,
		IncludeDeleted:  true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
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
		assert.Equal(t, test.result, activityReadable(test.user, test.doer), test.desc)
	}
}

func TestNotifyWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	action := &Action{
		ActUserID: 8,
		RepoID:    1,
		OpType:    ActionStarRepo,
	}
	assert.NoError(t, NotifyWatchers(action))

	// One watchers are inactive, thus action is only created for user 8, 1, 4, 11
	unittest.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    8,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    1,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    4,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
	unittest.AssertExistsAndLoadBean(t, &Action{
		ActUserID: action.ActUserID,
		UserID:    11,
		RepoID:    action.RepoID,
		OpType:    action.OpType,
	})
}

func TestGetFeedsCorrupted(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	unittest.AssertExistsAndLoadBean(t, &Action{
		ID:     8,
		RepoID: 1700,
	})

	actions, err := GetFeeds(db.DefaultContext, GetFeedsOptions{
		RequestedUser:  user,
		Actor:          user,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
}

func TestConsistencyUpdateAction(t *testing.T) {
	if !setting.Database.UseSQLite3 {
		t.Skip("Test is only for SQLite database.")
	}
	assert.NoError(t, unittest.PrepareTestDatabase())
	id := 8
	unittest.AssertExistsAndLoadBean(t, &Action{
		ID: int64(id),
	})
	_, err := db.GetEngine(db.DefaultContext).Exec(`UPDATE action SET created_unix = "" WHERE id = ?`, id)
	assert.NoError(t, err)
	actions := make([]*Action, 0, 1)
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
	count, err := CountActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = FixActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = CountActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	count, err = FixActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)

	//
	// XORM must be happy now
	//
	assert.NoError(t, db.GetEngine(db.DefaultContext).Where("id = ?", id).Find(&actions))
	unittest.CheckConsistencyFor(t, &Action{})
}
