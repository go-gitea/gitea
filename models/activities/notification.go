// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
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
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status NotificationStatus `xorm:"SMALLINT NOT NULL"`
	Source NotificationSource `xorm:"SMALLINT NOT NULL"`

	IssueID         int64 `xorm:"NOT NULL"`
	CommitID        string
	CommentID       int64
	CommitCommentID int64

	UpdatedBy int64 `xorm:"NOT NULL"`

	Issue      *issues_model.Issue    `xorm:"-"`
	Repository *repo_model.Repository `xorm:"-"`
	Comment    *issues_model.Comment  `xorm:"-"`
	User       *user_model.User       `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

// TableIndices implements xorm's TableIndices interface
func (n *Notification) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 8)
	usuuIndex := schemas.NewIndex("u_s_uu", schemas.IndexType)
	usuuIndex.AddColumn("user_id", "status", "updated_unix")
	indices = append(indices, usuuIndex)

	// Add the individual indices that were previously defined in struct tags
	userIDIndex := schemas.NewIndex("idx_notification_user_id", schemas.IndexType)
	userIDIndex.AddColumn("user_id")
	indices = append(indices, userIDIndex)

	repoIDIndex := schemas.NewIndex("idx_notification_repo_id", schemas.IndexType)
	repoIDIndex.AddColumn("repo_id")
	indices = append(indices, repoIDIndex)

	statusIndex := schemas.NewIndex("idx_notification_status", schemas.IndexType)
	statusIndex.AddColumn("status")
	indices = append(indices, statusIndex)

	sourceIndex := schemas.NewIndex("idx_notification_source", schemas.IndexType)
	sourceIndex.AddColumn("source")
	indices = append(indices, sourceIndex)

	issueIDIndex := schemas.NewIndex("idx_notification_issue_id", schemas.IndexType)
	issueIDIndex.AddColumn("issue_id")
	indices = append(indices, issueIDIndex)

	commitIDIndex := schemas.NewIndex("idx_notification_commit_id", schemas.IndexType)
	commitIDIndex.AddColumn("commit_id")
	indices = append(indices, commitIDIndex)

	updatedByIndex := schemas.NewIndex("idx_notification_updated_by", schemas.IndexType)
	updatedByIndex.AddColumn("updated_by")
	indices = append(indices, updatedByIndex)

	return indices
}

func init() {
	db.RegisterModel(new(Notification))
}

// CreateCommitCommentNotification creates notifications for a commit comment.
// It notifies the commit author (if they're a Gitea user), everyone already
// participating in the commit's conversation, and any @mentioned users.
// Recipients are filtered to those with code read access on the repository so
// private repos do not leak.
func CreateCommitCommentNotification(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commitSHA string, commitCommentID int64, commitAuthorEmail string, mentionedUsernames []string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		receiverIDs := make(container.Set[int64])

		// Notify everyone who already commented on this commit, otherwise a
		// reply never reaches the person it replies to.
		participantIDs, err := repo_model.GetCommitCommentPosterIDs(ctx, repo.ID, commitSHA)
		if err != nil {
			return fmt.Errorf("GetCommitCommentPosterIDs: %w", err)
		}
		receiverIDs.AddMultiple(participantIDs...)

		// Notify the commit author if they map to a Gitea user. A missing
		// user is expected (commit author may not have a Gitea account); any
		// other error is a real DB failure and should surface.
		if commitAuthorEmail != "" {
			author, err := user_model.GetUserByEmail(ctx, commitAuthorEmail)
			switch {
			case err == nil:
				receiverIDs.Add(author.ID)
			case user_model.IsErrUserNotExist(err):
				// commit author isn't a gitea user, skip silently
			default:
				return fmt.Errorf("GetUserByEmail: %w", err)
			}
		}

		// Notify @mentioned users. GetUserIDsByNames with non-existent users
		// in the slice would normally fail; we already pass true (ignoreNonExisting)
		// so any returned error here is a real DB failure.
		if len(mentionedUsernames) > 0 {
			mentionedIDs, err := user_model.GetUserIDsByNames(ctx, mentionedUsernames, true)
			if err != nil {
				return fmt.Errorf("GetUserIDsByNames: %w", err)
			}
			receiverIDs.AddMultiple(mentionedIDs...)
		}

		receiverIDs.Remove(doer.ID)
		if len(receiverIDs) == 0 {
			return nil
		}

		userIDs := receiverIDs.Values()
		// users deleted between mention-resolution and now drop out of the map here
		recipients, err := user_model.GetUsersMapByIDs(ctx, userIDs)
		if err != nil {
			return fmt.Errorf("GetUsersMapByIDs: %w", err)
		}
		existing, err := getCommitNotifications(ctx, repo.ID, commitSHA, userIDs)
		if err != nil {
			return err
		}

		for uid, recipient := range recipients {
			// without the unit check, commit-author or @mention notifications
			// would reach users with no visibility into a private repo
			if !access.CheckRepoUnitUser(ctx, repo, recipient, unit.TypeCode) {
				continue
			}
			if err := createOrUpdateCommitNotification(ctx, existing[uid], uid, repo.ID, commitSHA, commitCommentID, doer.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

// getCommitNotifications returns the existing notification of each given user
// for one commit, keyed by user id.
func getCommitNotifications(ctx context.Context, repoID int64, commitSHA string, userIDs []int64) (map[int64]*Notification, error) {
	var notifications []*Notification
	if err := db.GetEngine(ctx).
		Where("repo_id = ? AND source = ? AND commit_id = ?", repoID, NotificationSourceCommit, commitSHA).
		In("user_id", userIDs).
		Find(&notifications); err != nil {
		return nil, fmt.Errorf("get commit notifications [%s]: %w", commitSHA, err)
	}
	byUser := make(map[int64]*Notification, len(notifications))
	for _, n := range notifications {
		byUser[n.UserID] = n
	}
	return byUser, nil
}

// createOrUpdateCommitNotification keeps a single notification row per user and
// commit, so a long conversation doesn't flood the list with one entry per
// comment.
func createOrUpdateCommitNotification(ctx context.Context, existing *Notification, userID, repoID int64, commitSHA string, commitCommentID, updatedByID int64) error {
	if existing == nil {
		return db.Insert(ctx, &Notification{
			UserID:          userID,
			RepoID:          repoID,
			Status:          NotificationStatusUnread,
			Source:          NotificationSourceCommit,
			CommitID:        commitSHA,
			CommitCommentID: commitCommentID,
			UpdatedBy:       updatedByID,
		})
	}

	// Only re-point an already-read notification at the newest comment; moving
	// an unread one forward would skip the comments not seen yet.
	existing.UpdatedBy = updatedByID
	cols := []string{"updated_by", "updated_unix"}
	if existing.Status == NotificationStatusRead {
		existing.Status = NotificationStatusUnread
		existing.CommitCommentID = commitCommentID
		cols = append(cols, "status", "commit_comment_id")
	}

	_, err := db.GetEngine(ctx).ID(existing.ID).Cols(cols...).Update(existing)
	return err
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
	if err = (NotificationList{n}).LoadCommitComments(ctx); err != nil {
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

// commitCommentAnchor returns the fragment pointing at the commit comment that
// triggered the notification, or "" when it has been deleted since.
func (n *Notification) commitCommentAnchor() string {
	if n.CommitCommentID == 0 {
		return ""
	}
	return "#" + repo_model.CommitCommentHashTag(n.CommitCommentID)
}

// HTMLURL formats a URL-string to the notification
func (n *Notification) HTMLURL(ctx context.Context) string {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Comment != nil {
			return n.Comment.HTMLURL(ctx)
		}
		return n.Issue.HTMLURL(ctx)
	case NotificationSourceCommit:
		return n.Repository.HTMLURL(ctx) + "/commit/" + url.PathEscape(n.CommitID) + n.commitCommentAnchor()
	case NotificationSourceRepository:
		return n.Repository.HTMLURL(ctx)
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
		return n.Repository.Link() + "/commit/" + url.PathEscape(n.CommitID) + n.commitCommentAnchor()
	case NotificationSourceRepository:
		return n.Repository.Link()
	}
	return ""
}

// APIURL formats a URL-string to the notification
func (n *Notification) APIURL() string {
	return setting.AppURL + "api/v1/notifications/threads/" + strconv.FormatInt(n.ID, 10)
}

func notificationExists(notifications []*Notification, issueID, userID int64) bool {
	for _, notification := range notifications {
		if notification.IssueID == issueID && notification.UserID == userID {
			return true
		}
	}

	return false
}

// UserIDCount is a simple coalition of UserID and Count
type UserIDCount struct {
	UserID int64
	Count  int64
}

// GetUIDsAndNotificationCounts returns the unread counts for every user between the two provided times.
// It must return all user IDs which appear during the period, including count=0 for users who have read all.
func GetUIDsAndNotificationCounts(ctx context.Context, since, until timeutil.TimeStamp) ([]UserIDCount, error) {
	sql := `SELECT user_id, sum(case when status= ? then 1 else 0 end) AS count FROM notification ` +
		`WHERE user_id IN (SELECT user_id FROM notification WHERE updated_unix >= ? AND ` +
		`updated_unix < ?) GROUP BY user_id`
	var res []UserIDCount
	return res, db.GetEngine(ctx).SQL(sql, NotificationStatusUnread, since, until).Find(&res)
}

// SetIssueReadBy sets issue to be read by given user. The bool result is true
// when the unread count actually decreased, so callers can skip a push on no-op.
func SetIssueReadBy(ctx context.Context, issueID, userID int64) (bool, error) {
	if err := issues_model.UpdateIssueUserByRead(ctx, userID, issueID); err != nil {
		return false, err
	}

	return setIssueNotificationStatusReadIfUnread(ctx, userID, issueID)
}

func setIssueNotificationStatusReadIfUnread(ctx context.Context, userID, issueID int64) (bool, error) {
	notification, err := GetIssueNotification(ctx, userID, issueID)
	// ignore if not exists
	if err != nil {
		return false, nil
	}

	if notification.Status != NotificationStatusUnread {
		return false, nil
	}

	notification.Status = NotificationStatusRead

	if _, err := db.GetEngine(ctx).ID(notification.ID).Cols("status").Update(notification); err != nil {
		return false, err
	}
	return true, nil
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

	_, err = db.GetEngine(ctx).ID(notificationID).Cols("status").Update(notification)
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

// UpdateNotificationStatuses updates the statuses of all of a user's notifications
// that are of the currentStatus type to the desiredStatus. Returns the number of
// rows actually changed so callers can skip downstream work on a no-op.
func UpdateNotificationStatuses(ctx context.Context, user *user_model.User, currentStatus, desiredStatus NotificationStatus) (int64, error) {
	n := &Notification{Status: desiredStatus, UpdatedBy: user.ID}
	return db.GetEngine(ctx).
		Where("user_id = ? AND status = ?", user.ID, currentStatus).
		Cols("status", "updated_by", "updated_unix").
		Update(n)
}
