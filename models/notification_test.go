// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueNotifications(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	assert.NoError(t, CreateOrUpdateIssueNotifications(issue.ID, 0, 2, 0))

	// User 9 is inactive, thus notifications for user 1 and 4 are created
	notf := unittest.AssertExistsAndLoadBean(t, &Notification{UserID: 1, IssueID: issue.ID}).(*Notification)
	assert.Equal(t, NotificationStatusUnread, notf.Status)
	unittest.CheckConsistencyFor(t, &Issue{ID: issue.ID})

	notf = unittest.AssertExistsAndLoadBean(t, &Notification{UserID: 4, IssueID: issue.ID}).(*Notification)
	assert.Equal(t, NotificationStatusUnread, notf.Status)
}

func TestNotificationsForUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	statuses := []NotificationStatus{NotificationStatusRead, NotificationStatusUnread}
	notfs, err := NotificationsForUser(db.DefaultContext, user, statuses, 1, 10)
	assert.NoError(t, err)
	if assert.Len(t, notfs, 3) {
		assert.EqualValues(t, 5, notfs[0].ID)
		assert.EqualValues(t, user.ID, notfs[0].UserID)
		assert.EqualValues(t, 4, notfs[1].ID)
		assert.EqualValues(t, user.ID, notfs[1].UserID)
		assert.EqualValues(t, 2, notfs[2].ID)
		assert.EqualValues(t, user.ID, notfs[2].UserID)
	}
}

func TestNotification_GetRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &Notification{RepoID: 1}).(*Notification)
	repo, err := notf.GetRepo()
	assert.NoError(t, err)
	assert.Equal(t, repo, notf.Repository)
	assert.EqualValues(t, notf.RepoID, repo.ID)
}

func TestNotification_GetIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &Notification{RepoID: 1}).(*Notification)
	issue, err := notf.GetIssue()
	assert.NoError(t, err)
	assert.Equal(t, issue, notf.Issue)
	assert.EqualValues(t, notf.IssueID, issue.ID)
}

func TestGetNotificationCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	cnt, err := GetNotificationCount(db.DefaultContext, user, NotificationStatusRead)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, cnt)

	cnt, err = GetNotificationCount(db.DefaultContext, user, NotificationStatusUnread)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

func TestSetNotificationStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	notf := unittest.AssertExistsAndLoadBean(t,
		&Notification{UserID: user.ID, Status: NotificationStatusRead}).(*Notification)
	_, err := SetNotificationStatus(notf.ID, user, NotificationStatusPinned)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t,
		&Notification{ID: notf.ID, Status: NotificationStatusPinned})

	_, err = SetNotificationStatus(1, user, NotificationStatusRead)
	assert.Error(t, err)
	_, err = SetNotificationStatus(unittest.NonexistentID, user, NotificationStatusRead)
	assert.Error(t, err)
}

func TestUpdateNotificationStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	notfUnread := unittest.AssertExistsAndLoadBean(t,
		&Notification{UserID: user.ID, Status: NotificationStatusUnread}).(*Notification)
	notfRead := unittest.AssertExistsAndLoadBean(t,
		&Notification{UserID: user.ID, Status: NotificationStatusRead}).(*Notification)
	notfPinned := unittest.AssertExistsAndLoadBean(t,
		&Notification{UserID: user.ID, Status: NotificationStatusPinned}).(*Notification)
	assert.NoError(t, UpdateNotificationStatuses(user, NotificationStatusUnread, NotificationStatusRead))
	unittest.AssertExistsAndLoadBean(t,
		&Notification{ID: notfUnread.ID, Status: NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&Notification{ID: notfRead.ID, Status: NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&Notification{ID: notfPinned.ID, Status: NotificationStatusPinned})
}
