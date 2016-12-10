package models

import (
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
	PullID   int64  `xorm:"INDEX"`
	CommitID string `xorm:"INDEX"`

	Issue       *Issue       `xorm:"-"`
	PullRequest *PullRequest `xorm:"-"`

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

// BeforeUpdate runs while updateing a record
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
		exists, err := issueNotificationExists(sess, watch.UserID, issue.ID)
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
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(notification)
	return notification, err
}

// NotificationsForUser returns notifications for a given user and status
func NotificationsForUser(user *User, status NotificationStatus) ([]*Notification, error) {
	return notificationsForUser(x, user, status)
}
func notificationsForUser(e Engine, user *User, status NotificationStatus) (notifications []*Notification, err error) {
	err = e.
		Where("user_id = ?", user.ID).
		And("status = ?", status).
		OrderBy("updated_unix DESC").
		Find(&notifications)
	return
}

// GetRepo returns the repo of the notification
func (n *Notification) GetRepo() (repo *Repository, err error) {
	repo = new(Repository)
	_, err = x.
		Where("id = ?", n.RepoID).
		Get(repo)
	return
}

// GetIssue returns the issue of the notification
func (n *Notification) GetIssue() (issue *Issue, err error) {
	issue = new(Issue)
	_, err = x.
		Where("id = ?", n.IssueID).
		Get(issue)
	return
}
