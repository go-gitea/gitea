// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"
)

type (
	// NotificationStatus is the status of the notification (read or unread)
	NotificationStatus uint8
	// NotificationSource is the source of the notification (issue, PR, commit, etc)
	NotificationSource uint8
)

const (
	// NotificationStatusUnread represents an unread notification
	NotificationStatusUnread NotificationStatus = iota + 1
	// NotificationStatusRead represents a read notification
	NotificationStatusRead
	// NotificationStatusPinned represents a pinned notification
	NotificationStatusPinned
)

const (
	// NotificationSourceIssue is a notification of an issue
	NotificationSourceIssue NotificationSource = iota + 1
	// NotificationSourcePullRequest is a notification of a pull request
	NotificationSourcePullRequest
	// NotificationSourceCommit is a notification of a commit
	NotificationSourceCommit
)

// Notification represents a notification
type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`
	RepoID int64 `xorm:"INDEX NOT NULL"`

	Status NotificationStatus `xorm:"SMALLINT INDEX NOT NULL"`
	Source NotificationSource `xorm:"SMALLINT INDEX NOT NULL"`

	IssueID  int64  `xorm:"INDEX NOT NULL"`
	CommitID string `xorm:"INDEX"`

	UpdatedBy int64 `xorm:"INDEX NOT NULL"`

	Issue      *Issue      `xorm:"-"`
	Repository *Repository `xorm:"-"`

	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX NOT NULL"`
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"INDEX NOT NULL"`
}

// BeforeInsert runs while inserting a record
func (n *Notification) BeforeInsert() {
	var (
		now     = time.Now()
		nowUnix = now.Unix()
	)
	n.Created = now
	n.CreatedUnix = nowUnix
	n.Updated = now
	n.UpdatedUnix = nowUnix
}

// BeforeUpdate runs while updating a record
func (n *Notification) BeforeUpdate() {
	var (
		now     = time.Now()
		nowUnix = now.Unix()
	)
	n.Updated = now
	n.UpdatedUnix = nowUnix
}

// CreateOrUpdateIssueNotifications creates an issue notification
// for each watcher, or updates it if already exists
func CreateOrUpdateIssueNotifications(issue *Issue, notificationAuthorID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := createOrUpdateIssueNotifications(sess, issue, notificationAuthorID); err != nil {
		return err
	}

	return sess.Commit()
}

func createOrUpdateIssueNotifications(e Engine, issue *Issue, notificationAuthorID int64) error {
	issueWatches, err := getIssueWatchers(e, issue.ID)
	if err != nil {
		return err
	}

	watches, err := getWatchers(e, issue.RepoID)
	if err != nil {
		return err
	}

	notifications, err := getNotificationsByIssueID(e, issue.ID)
	if err != nil {
		return err
	}

	alreadyNotified := make(map[int64]struct{}, len(issueWatches)+len(watches))

	notifyUser := func(userID int64) error {
		// do not send notification for the own issuer/commenter
		if userID == notificationAuthorID {
			return nil
		}

		if _, ok := alreadyNotified[userID]; ok {
			return nil
		}
		alreadyNotified[userID] = struct{}{}

		if notificationExists(notifications, issue.ID, userID) {
			return updateIssueNotification(e, userID, issue.ID, notificationAuthorID)
		}
		return createIssueNotification(e, userID, issue, notificationAuthorID)
	}

	for _, issueWatch := range issueWatches {
		// ignore if user unwatched the issue
		if !issueWatch.IsWatching {
			alreadyNotified[issueWatch.UserID] = struct{}{}
			continue
		}

		if err := notifyUser(issueWatch.UserID); err != nil {
			return err
		}
	}

	for _, watch := range watches {
		if err := notifyUser(watch.UserID); err != nil {
			return err
		}
	}
	return nil
}

func getNotificationsByIssueID(e Engine, issueID int64) (notifications []*Notification, err error) {
	err = e.
		Where("issue_id = ?", issueID).
		Find(&notifications)
	return
}

func notificationExists(notifications []*Notification, issueID, userID int64) bool {
	for _, notification := range notifications {
		if notification.IssueID == issueID && notification.UserID == userID {
			return true
		}
	}

	return false
}

func createIssueNotification(e Engine, userID int64, issue *Issue, updatedByID int64) error {
	notification := &Notification{
		UserID:    userID,
		RepoID:    issue.RepoID,
		Status:    NotificationStatusUnread,
		IssueID:   issue.ID,
		UpdatedBy: updatedByID,
	}

	if issue.IsPull {
		notification.Source = NotificationSourcePullRequest
	} else {
		notification.Source = NotificationSourceIssue
	}

	_, err := e.Insert(notification)
	return err
}

func updateIssueNotification(e Engine, userID, issueID, updatedByID int64) error {
	notification, err := getIssueNotification(e, userID, issueID)
	if err != nil {
		return err
	}

	notification.Status = NotificationStatusUnread
	notification.UpdatedBy = updatedByID

	_, err = e.Id(notification.ID).Update(notification)
	return err
}

func getIssueNotification(e Engine, userID, issueID int64) (*Notification, error) {
	notification := new(Notification)
	_, err := e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(notification)
	return notification, err
}

// NotificationsForUser returns notifications for a given user and status
func NotificationsForUser(user *User, statuses []NotificationStatus, page, perPage int) ([]*Notification, error) {
	return notificationsForUser(x, user, statuses, page, perPage)
}
func notificationsForUser(e Engine, user *User, statuses []NotificationStatus, page, perPage int) (notifications []*Notification, err error) {
	if len(statuses) == 0 {
		return
	}

	sess := e.
		Where("user_id = ?", user.ID).
		In("status", statuses).
		OrderBy("updated_unix DESC")

	if page > 0 && perPage > 0 {
		sess.Limit(perPage, (page-1)*perPage)
	}

	err = sess.Find(&notifications)
	return
}

// GetRepo returns the repo of the notification
func (n *Notification) GetRepo() (*Repository, error) {
	n.Repository = new(Repository)
	_, err := x.
		Where("id = ?", n.RepoID).
		Get(n.Repository)
	return n.Repository, err
}

// GetIssue returns the issue of the notification
func (n *Notification) GetIssue() (*Issue, error) {
	n.Issue = new(Issue)
	_, err := x.
		Where("id = ?", n.IssueID).
		Get(n.Issue)
	return n.Issue, err
}

// GetNotificationCount returns the notification count for user
func GetNotificationCount(user *User, status NotificationStatus) (int64, error) {
	return getNotificationCount(x, user, status)
}

func getNotificationCount(e Engine, user *User, status NotificationStatus) (count int64, err error) {
	count, err = e.
		Where("user_id = ?", user.ID).
		And("status = ?", status).
		Count(&Notification{})
	return
}

func setNotificationStatusReadIfUnread(e Engine, userID, issueID int64) error {
	notification, err := getIssueNotification(e, userID, issueID)
	// ignore if not exists
	if err != nil {
		return nil
	}

	if notification.Status != NotificationStatusUnread {
		return nil
	}

	notification.Status = NotificationStatusRead

	_, err = e.Id(notification.ID).Update(notification)
	return err
}

// SetNotificationStatus change the notification status
func SetNotificationStatus(notificationID int64, user *User, status NotificationStatus) error {
	notification, err := getNotificationByID(notificationID)
	if err != nil {
		return err
	}

	if notification.UserID != user.ID {
		return fmt.Errorf("Can't change notification of another user: %d, %d", notification.UserID, user.ID)
	}

	notification.Status = status

	_, err = x.Id(notificationID).Update(notification)
	return err
}

func getNotificationByID(notificationID int64) (*Notification, error) {
	notification := new(Notification)
	ok, err := x.
		Where("id = ?", notificationID).
		Get(notification)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("Notification %d does not exists", notificationID)
	}

	return notification, nil
}
