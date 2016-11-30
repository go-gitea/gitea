package models

import (
	"time"
)

const (
	NotificationStatusUnread = "U"
	NotificationStatusRead   = "R"

	NotificationSourceIssue       = "I"
	NotificationSourcePullRequest = "P"
	NotificationSourceCommit      = "C"
)

type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`
	RepoID int64 `xorm:"INDEX NOT NULL"`

	Status string `xorm:"VARCHAR(1) INDEX NOT NULL"`
	Source string `xorm:"VARCHAR(1) INDEX NOT NULL"`

	IssueID  int64  `xorm:"INDEX NOT NULL"`
	PullID   int64  `xorm:"INDEX"`
	CommitID string `xorm:"INDEX"`

	Issue       *Issue       `xorm:"-"`
	PullRequest *PullRequest `xorm:"-"`

	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX NOT NULL"`
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"INDEX NOT NULL"`
}

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
func (n *Notification) BeforeUpdate() {
	var (
		now     = time.Now()
		nowUnix = now.Unix()
	)
	n.Updated = now
	n.UpdatedUnix = nowUnix
}

func CreateOrUpdateIssueNotifications(issue *Issue) error {
	watches, err := getWatchers(x, issue.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	defer sess.Close()

	for _, watch := range watches {
		exists, err := issueNotificationExists(sess, watch.UserID, watch.RepoID)
		if err != nil {
			return err
		}

		if exists {
			err = updateIssueNotification(sess, watch.UserID, issue.ID)
		} else {
			err = createIssueNotification(sess, watch.UserID, issue)
		}

		if err != nil {
			return err
		}
	}

	return sess.Commit()
}

func issueNotificationExists(e Engine, userID, issueID int64) (bool, error) {
	count, err := e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Count(Notification{})
	return count > 0, err
}

func createIssueNotification(e Engine, userID int64, issue *Issue) error {
	notification := &Notification{
		UserID:  userID,
		RepoID:  issue.RepoID,
		Status:  NotificationStatusUnread,
		IssueID: issue.ID,
	}

	if issue.IsPull {
		notification.Source = NotificationSourcePullRequest
	} else {
		notification.Source = NotificationSourceIssue
	}

	_, err := e.Insert(notification)
	return err
}

func updateIssueNotification(e Engine, userID, issueID int64) error {
	notification, err := getIssueNotification(e, userID, issueID)
	if err != nil {
		return err
	}

	notification.Status = NotificationStatusUnread

	_, err = e.Id(notification.ID).Update(notification)
	return err
}

func getIssueNotification(e Engine, userID, issueID int64) (*Notification, error) {
	notification := new(Notification)
	_, err := e.
		Where("user_id = ?").
		And("issue_id = ?", issueID).
		Get(notification)
	return notification, err
}
