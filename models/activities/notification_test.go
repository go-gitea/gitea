// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"context"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrUpdateIssueNotifications(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	assert.NoError(t, activities_model.CreateOrUpdateIssueNotifications(t.Context(), issue.ID, 0, 2, 0))

	// User 9 is inactive, thus notifications for user 1 and 4 are created
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 1, IssueID: issue.ID})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
	unittest.CheckConsistencyFor(t, &issues_model.Issue{ID: issue.ID})

	notf = unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 4, IssueID: issue.ID})
	assert.Equal(t, activities_model.NotificationStatusUnread, notf.Status)
}

func TestNotificationsForUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notfs, err := db.Find[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusRead,
			activities_model.NotificationStatusUnread,
		},
	})
	assert.NoError(t, err)
	if assert.Len(t, notfs, 3) {
		assert.EqualValues(t, 5, notfs[0].ID)
		assert.Equal(t, user.ID, notfs[0].UserID)
		assert.EqualValues(t, 4, notfs[1].ID)
		assert.Equal(t, user.ID, notfs[1].UserID)
		assert.EqualValues(t, 2, notfs[2].ID)
		assert.Equal(t, user.ID, notfs[2].UserID)
	}
}

func TestNotification_GetRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{RepoID: 1})
	repo, err := notf.GetRepo(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, repo, notf.Repository)
	assert.Equal(t, notf.RepoID, repo.ID)
}

func TestNotification_GetIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	notf := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{RepoID: 1})
	issue, err := notf.GetIssue(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, issue, notf.Issue)
	assert.Equal(t, notf.IssueID, issue.ID)
}

func TestGetNotificationCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	cnt, err := db.Count[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusRead,
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, cnt)

	cnt, err = db.Count[activities_model.Notification](t.Context(), activities_model.FindNotificationOptions{
		UserID: user.ID,
		Status: []activities_model.NotificationStatus{
			activities_model.NotificationStatusUnread,
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

func TestSetNotificationStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notf := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusRead})
	_, err := activities_model.SetNotificationStatus(t.Context(), notf.ID, user, activities_model.NotificationStatusPinned)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notf.ID, Status: activities_model.NotificationStatusPinned})

	_, err = activities_model.SetNotificationStatus(t.Context(), 1, user, activities_model.NotificationStatusRead)
	assert.Error(t, err)
	_, err = activities_model.SetNotificationStatus(t.Context(), unittest.NonexistentID, user, activities_model.NotificationStatusRead)
	assert.Error(t, err)
}

func TestUpdateNotificationStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	notfUnread := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusUnread})
	notfRead := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusRead})
	notfPinned := unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{UserID: user.ID, Status: activities_model.NotificationStatusPinned})
	assert.NoError(t, activities_model.UpdateNotificationStatuses(t.Context(), user, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead))
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfUnread.ID, Status: activities_model.NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfRead.ID, Status: activities_model.NotificationStatusRead})
	unittest.AssertExistsAndLoadBean(t,
		&activities_model.Notification{ID: notfPinned.ID, Status: activities_model.NotificationStatusPinned})
}

func TestSetIssueReadBy(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		return activities_model.SetIssueReadBy(ctx, issue.ID, user.ID)
	}))

	nt, err := activities_model.GetIssueNotification(t.Context(), user.ID, issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, activities_model.NotificationStatusRead, nt.Status)
}
