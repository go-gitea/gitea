// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
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
	// NotificationSourceRepository is a notification for a repository
	NotificationSourceRepository
)

// Notification represents a notification
type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`
	RepoID int64 `xorm:"INDEX NOT NULL"`

	Status NotificationStatus `xorm:"SMALLINT INDEX NOT NULL"`
	Source NotificationSource `xorm:"SMALLINT INDEX NOT NULL"`

	IssueID   int64  `xorm:"INDEX NOT NULL"`
	CommitID  string `xorm:"INDEX"`
	CommentID int64

	UpdatedBy int64 `xorm:"INDEX NOT NULL"`

	Issue      *issues_model.Issue    `xorm:"-"`
	Repository *repo_model.Repository `xorm:"-"`
	Comment    *issues_model.Comment  `xorm:"-"`
	User       *user_model.User       `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated INDEX NOT NULL"`
}

func init() {
	db.RegisterModel(new(Notification))
}

// CreateRepoTransferNotification creates  notification for the user a repository was transferred to
func CreateRepoTransferNotification(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		var notify []*Notification

		if newOwner.IsOrganization() {
			users, err := organization.GetUsersWhoCanCreateOrgRepo(ctx, newOwner.ID)
			if err != nil || len(users) == 0 {
				return err
			}
			for i := range users {
				notify = append(notify, &Notification{
					UserID:    i,
					RepoID:    repo.ID,
					Status:    NotificationStatusUnread,
					UpdatedBy: doer.ID,
					Source:    NotificationSourceRepository,
				})
			}
		} else {
			notify = []*Notification{{
				UserID:    newOwner.ID,
				RepoID:    repo.ID,
				Status:    NotificationStatusUnread,
				UpdatedBy: doer.ID,
				Source:    NotificationSourceRepository,
			}}
		}

		return db.Insert(ctx, notify)
	})
}

// CreateOrUpdateIssueNotifications creates an issue notification
// for each watcher, or updates it if already exists
// receiverID > 0 just send to receiver, else send to all watcher
func CreateOrUpdateIssueNotifications(ctx context.Context, issueID, commentID, notificationAuthorID, receiverID int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := createOrUpdateIssueNotifications(ctx, issueID, commentID, notificationAuthorID, receiverID); err != nil {
		return err
	}

	return committer.Commit()
}

func createOrUpdateIssueNotifications(ctx context.Context, issueID, commentID, notificationAuthorID, receiverID int64) error {
	// init
	var toNotify container.Set[int64]
	notifications, err := getNotificationsByIssueID(ctx, issueID)
	if err != nil {
		return err
	}

	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return err
	}

	if receiverID > 0 {
		toNotify = make(container.Set[int64], 1)
		toNotify.Add(receiverID)
	} else {
		toNotify = make(container.Set[int64], 32)
		issueWatches, err := issues_model.GetIssueWatchersIDs(ctx, issueID, true)
		if err != nil {
			return err
		}
		toNotify.AddMultiple(issueWatches...)
		if !(issue.IsPull && issues_model.HasWorkInProgressPrefix(issue.Title)) {
			repoWatches, err := repo_model.GetRepoWatchersIDs(ctx, issue.RepoID)
			if err != nil {
				return err
			}
			toNotify.AddMultiple(repoWatches...)
		}
		issueParticipants, err := issue.GetParticipantIDsByIssue(ctx)
		if err != nil {
			return err
		}
		toNotify.AddMultiple(issueParticipants...)

		// dont notify user who cause notification
		delete(toNotify, notificationAuthorID)
		// explicit unwatch on issue
		issueUnWatches, err := issues_model.GetIssueWatchersIDs(ctx, issueID, false)
		if err != nil {
			return err
		}
		for _, id := range issueUnWatches {
			toNotify.Remove(id)
		}
	}

	err = issue.LoadRepo(ctx)
	if err != nil {
		return err
	}

	// notify
	for userID := range toNotify {
		issue.Repo.Units = nil
		user, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				continue
			}

			return err
		}
		if issue.IsPull && !access_model.CheckRepoUnitUser(ctx, issue.Repo, user, unit.TypePullRequests) {
			continue
		}
		if !issue.IsPull && !access_model.CheckRepoUnitUser(ctx, issue.Repo, user, unit.TypeIssues) {
			continue
		}

		if notificationExists(notifications, issue.ID, userID) {
			if err = updateIssueNotification(ctx, userID, issue.ID, commentID, notificationAuthorID); err != nil {
				return err
			}
			continue
		}
		if err = createIssueNotification(ctx, userID, issue, commentID, notificationAuthorID); err != nil {
			return err
		}
	}
	return nil
}

func createIssueNotification(ctx context.Context, userID int64, issue *issues_model.Issue, commentID, updatedByID int64) error {
	notification := &Notification{
		UserID:    userID,
		RepoID:    issue.RepoID,
		Status:    NotificationStatusUnread,
		IssueID:   issue.ID,
		CommentID: commentID,
		UpdatedBy: updatedByID,
	}

	if issue.IsPull {
		notification.Source = NotificationSourcePullRequest
	} else {
		notification.Source = NotificationSourceIssue
	}

	return db.Insert(ctx, notification)
}

func updateIssueNotification(ctx context.Context, userID, issueID, commentID, updatedByID int64) error {
	notification, err := GetIssueNotification(ctx, userID, issueID)
	if err != nil {
		return err
	}

	// NOTICE: Only update comment id when the before notification on this issue is read, otherwise you may miss some old comments.
	// But we need update update_by so that the notification will be reorder
	var cols []string
	if notification.Status == NotificationStatusRead {
		notification.Status = NotificationStatusUnread
		notification.CommentID = commentID
		cols = []string{"status", "update_by", "comment_id"}
	} else {
		notification.UpdatedBy = updatedByID
		cols = []string{"update_by"}
	}

	_, err = db.GetEngine(ctx).ID(notification.ID).Cols(cols...).Update(notification)
	return err
}

// GetIssueNotification return the notification about an issue
func GetIssueNotification(ctx context.Context, userID, issueID int64) (*Notification, error) {
	notification := new(Notification)
	_, err := db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(notification)
	return notification, err
}

// LoadAttributes load Repo Issue User and Comment if not loaded
func (n *Notification) LoadAttributes(ctx context.Context) (err error) {
	if err = n.loadRepo(ctx); err != nil {
		return err
	}
	if err = n.loadIssue(ctx); err != nil {
		return err
	}
	if err = n.loadUser(ctx); err != nil {
		return err
	}
	if err = n.loadComment(ctx); err != nil {
		return err
	}
	return err
}

func (n *Notification) loadRepo(ctx context.Context) (err error) {
	if n.Repository == nil {
		n.Repository, err = repo_model.GetRepositoryByID(ctx, n.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", n.RepoID, err)
		}
	}
	return nil
}

func (n *Notification) loadIssue(ctx context.Context) (err error) {
	if n.Issue == nil && n.IssueID != 0 {
		n.Issue, err = issues_model.GetIssueByID(ctx, n.IssueID)
		if err != nil {
			return fmt.Errorf("getIssueByID [%d]: %w", n.IssueID, err)
		}
		return n.Issue.LoadAttributes(ctx)
	}
	return nil
}

func (n *Notification) loadComment(ctx context.Context) (err error) {
	if n.Comment == nil && n.CommentID != 0 {
		n.Comment, err = issues_model.GetCommentByID(ctx, n.CommentID)
		if err != nil {
			if issues_model.IsErrCommentNotExist(err) {
				return issues_model.ErrCommentNotExist{
					ID:      n.CommentID,
					IssueID: n.IssueID,
				}
			}
			return err
		}
	}
	return nil
}

func (n *Notification) loadUser(ctx context.Context) (err error) {
	if n.User == nil {
		n.User, err = user_model.GetUserByID(ctx, n.UserID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %w", n.UserID, err)
		}
	}
	return nil
}

// GetRepo returns the repo of the notification
func (n *Notification) GetRepo(ctx context.Context) (*repo_model.Repository, error) {
	return n.Repository, n.loadRepo(ctx)
}

// GetIssue returns the issue of the notification
func (n *Notification) GetIssue(ctx context.Context) (*issues_model.Issue, error) {
	return n.Issue, n.loadIssue(ctx)
}

// HTMLURL formats a URL-string to the notification
func (n *Notification) HTMLURL(ctx context.Context) string {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Comment != nil {
			return n.Comment.HTMLURL(ctx)
		}
		return n.Issue.HTMLURL()
	case NotificationSourceCommit:
		return n.Repository.HTMLURL() + "/commit/" + url.PathEscape(n.CommitID)
	case NotificationSourceRepository:
		return n.Repository.HTMLURL()
	}
	return ""
}

// Link formats a relative URL-string to the notification
func (n *Notification) Link(ctx context.Context) string {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Comment != nil {
			return n.Comment.Link(ctx)
		}
		return n.Issue.Link()
	case NotificationSourceCommit:
		return n.Repository.Link() + "/commit/" + url.PathEscape(n.CommitID)
	case NotificationSourceRepository:
		return n.Repository.Link()
	}
	return ""
}

// APIURL formats a URL-string to the notification
func (n *Notification) APIURL() string {
	return setting.AppURL + "api/v1/notifications/threads/" + strconv.FormatInt(n.ID, 10)
}

// SetIssueReadBy sets issue to be read by given user.
func SetIssueReadBy(ctx context.Context, issueID, userID int64) error {
	if err := issues_model.UpdateIssueUserByRead(ctx, userID, issueID); err != nil {
		return err
	}

	return setIssueNotificationStatusReadIfUnread(ctx, userID, issueID)
}

func setIssueNotificationStatusReadIfUnread(ctx context.Context, userID, issueID int64) error {
	notification, err := GetIssueNotification(ctx, userID, issueID)
	// ignore if not exists
	if err != nil {
		return nil
	}

	if notification.Status != NotificationStatusUnread {
		return nil
	}

	notification.Status = NotificationStatusRead

	_, err = db.GetEngine(ctx).ID(notification.ID).Cols("status").Update(notification)
	return err
}

// SetRepoReadBy sets repo to be visited by given user.
func SetRepoReadBy(ctx context.Context, userID, repoID int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{
		"user_id": userID,
		"status":  NotificationStatusUnread,
		"source":  NotificationSourceRepository,
		"repo_id": repoID,
	}).Cols("status").Update(&Notification{Status: NotificationStatusRead})
	return err
}

// SetNotificationStatus change the notification status
func SetNotificationStatus(ctx context.Context, notificationID int64, user *user_model.User, status NotificationStatus) (*Notification, error) {
	notification, err := GetNotificationByID(ctx, notificationID)
	if err != nil {
		return notification, err
	}

	if notification.UserID != user.ID {
		return nil, fmt.Errorf("Can't change notification of another user: %d, %d", notification.UserID, user.ID)
	}

	notification.Status = status

	_, err = db.GetEngine(ctx).ID(notificationID).Update(notification)
	return notification, err
}

// GetNotificationByID return notification by ID
func GetNotificationByID(ctx context.Context, notificationID int64) (*Notification, error) {
	notification := new(Notification)
	ok, err := db.GetEngine(ctx).
		Where("id = ?", notificationID).
		Get(notification)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, db.ErrNotExist{Resource: "notification", ID: notificationID}
	}

	return notification, nil
}

// UpdateNotificationStatuses updates the statuses of all of a user's notifications that are of the currentStatus type to the desiredStatus
func UpdateNotificationStatuses(ctx context.Context, user *user_model.User, currentStatus, desiredStatus NotificationStatus) error {
	n := &Notification{Status: desiredStatus, UpdatedBy: user.ID}
	_, err := db.GetEngine(ctx).
		Where("user_id = ? AND status = ?", user.ID, currentStatus).
		Cols("status", "updated_by", "updated_unix").
		Update(n)
	return err
}
