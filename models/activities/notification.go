// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/svg"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

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
	// NotificationSourceRelease is a notification for a release
	NotificationSourceRelease
)

// Notification represents a notification
type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status NotificationStatus `xorm:"SMALLINT NOT NULL"`
	Source NotificationSource `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey string `xorm:"VARCHAR(255) NOT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	Issue      *issues_model.Issue    `xorm:"-"`
	Repository *repo_model.Repository `xorm:"-"`
	Comment    *issues_model.Comment  `xorm:"-"`
	User       *user_model.User       `xorm:"-"`
	Release    *repo_model.Release    `xorm:"-"`
	Commit     *git.Commit            `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

// TableIndices implements xorm's TableIndices interface
func (n *Notification) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 6)
	usuuIndex := schemas.NewIndex("u_s_uu", schemas.IndexType)
	usuuIndex.AddColumn("user_id", "status", "updated_unix")
	indices = append(indices, usuuIndex)

	userIDIndex := schemas.NewIndex("idx_notification_user_id", schemas.IndexType)
	userIDIndex.AddColumn("user_id")
	indices = append(indices, userIDIndex)

	repoIDIndex := schemas.NewIndex("idx_notification_repo_id", schemas.IndexType)
	repoIDIndex.AddColumn("repo_id")
	indices = append(indices, repoIDIndex)

	statusIndex := schemas.NewIndex("idx_notification_status", schemas.IndexType)
	statusIndex.AddColumn("status")
	indices = append(indices, statusIndex)

	updatedByIndex := schemas.NewIndex("idx_notification_updated_by", schemas.IndexType)
	updatedByIndex.AddColumn("updated_by")
	indices = append(indices, updatedByIndex)

	uniqueNotificationKey := schemas.NewIndex("unique_notification_key", schemas.UniqueType)
	uniqueNotificationKey.AddColumn("user_id", "unique_key")
	indices = append(indices, uniqueNotificationKey)

	return indices
}

func init() {
	db.RegisterModel(new(Notification))
}

// NotificationSourceForIssue returns the notification source matching whether
// the issue is a pull request or a regular issue.
func NotificationSourceForIssue(issue *issues_model.Issue) NotificationSource {
	return util.Iif(issue.IsPull, NotificationSourcePullRequest, NotificationSourceIssue)
}

func uniqueKeyForIssueNotification(issueID int64, isPull bool) string {
	return fmt.Sprintf("%s-%d", util.Iif(isPull, "pull", "issue"), issueID)
}

func uniqueKeyForCommitNotification(repoID int64, commitID string) string {
	return fmt.Sprintf("commit-%d-%s", repoID, commitID)
}

func uniqueKeyForRepositoryNotification(repoID int64) string {
	return fmt.Sprintf("repo-%d", repoID)
}

// UniqueKeyForReleaseNotification returns the unique_key value for a release notification.
func UniqueKeyForReleaseNotification(releaseID int64) string {
	return fmt.Sprintf("release-%d", releaseID)
}

// upsertNotificationByUniqueKey marks an existing notification unread (updating the doer)
// or inserts newNotification if none exists for the user/unique_key pair.
//
// The unique index on (user_id, unique_key) means two concurrent callers can race: both
// see no existing row and both attempt to insert, but the DB rejects the second insert.
// We retry the read-update path a small number of times to absorb that race.
func upsertNotificationByUniqueKey(ctx context.Context, doerID int64, newNotification *Notification) error {
	const maxAttempts = 3
	var lastInsertErr error
	for range maxAttempts {
		existing := new(Notification)
		ok, err := db.GetEngine(ctx).
			Where("user_id = ?", newNotification.UserID).
			And("unique_key = ?", newNotification.UniqueKey).
			Get(existing)
		if err != nil {
			return err
		}
		if ok {
			existing.Status = NotificationStatusUnread
			existing.UpdatedBy = doerID
			_, err := db.GetEngine(ctx).ID(existing.ID).Cols("status", "updated_by").Update(existing)
			return err
		}
		insertErr := db.Insert(ctx, newNotification)
		if insertErr == nil {
			return nil
		}
		// Insert failed — likely a concurrent insert won the race. Loop and try the update path.
		lastInsertErr = insertErr
		newNotification.ID = 0
	}
	return lastInsertErr
}

// CreateRepoTransferNotification creates a notification for the user a repository was transferred to
func CreateRepoTransferNotification(ctx context.Context, doerID, repoID, receiverID int64) error {
	return upsertNotificationByUniqueKey(ctx, doerID, &Notification{
		UserID:    receiverID,
		RepoID:    repoID,
		Status:    NotificationStatusUnread,
		UpdatedBy: doerID,
		Source:    NotificationSourceRepository,
		UniqueKey: uniqueKeyForRepositoryNotification(repoID),
	})
}

func CreateCommitNotifications(ctx context.Context, doerID, repoID int64, commitID string, receiverID int64) error {
	return upsertNotificationByUniqueKey(ctx, doerID, &Notification{
		Source:    NotificationSourceCommit,
		UserID:    receiverID,
		RepoID:    repoID,
		CommitID:  commitID,
		UniqueKey: uniqueKeyForCommitNotification(repoID, commitID),
		Status:    NotificationStatusUnread,
		UpdatedBy: doerID,
	})
}

func CreateOrUpdateReleaseNotifications(ctx context.Context, doerID, repoID, releaseID, receiverID int64) error {
	return upsertNotificationByUniqueKey(ctx, doerID, &Notification{
		Source:    NotificationSourceRelease,
		RepoID:    repoID,
		UserID:    receiverID,
		Status:    NotificationStatusUnread,
		ReleaseID: releaseID,
		UniqueKey: UniqueKeyForReleaseNotification(releaseID),
		UpdatedBy: doerID,
	})
}

func createIssueNotification(ctx context.Context, userID int64, issue *issues_model.Issue, commentID, updatedByID int64) error {
	uniqueKey := uniqueKeyForIssueNotification(issue.ID, issue.IsPull)
	notification := &Notification{
		UserID:    userID,
		RepoID:    issue.RepoID,
		Status:    NotificationStatusUnread,
		IssueID:   issue.ID,
		CommentID: commentID,
		UniqueKey: uniqueKey,
		UpdatedBy: updatedByID,
	}

	if issue.IsPull {
		notification.Source = NotificationSourcePullRequest
	} else {
		notification.Source = NotificationSourceIssue
	}

	return db.Insert(ctx, notification)
}

func updateIssueNotification(ctx context.Context, notification *Notification, commentID, updatedByID int64) error {
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

	_, err := db.GetEngine(ctx).ID(notification.ID).Cols(cols...).Update(notification)
	return err
}

// GetIssueNotification return the notification about an issue
func GetIssueNotification(ctx context.Context, userID, issueID int64) (*Notification, error) {
	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	return getIssueNotificationByUniqueKey(ctx, userID, uniqueKeyForIssueNotification(issueID, issue.IsPull), issueID)
}

func getIssueNotificationByUniqueKey(ctx context.Context, userID int64, uniqueKey string, issueID int64) (*Notification, error) {
	notification := new(Notification)
	ok, err := db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("unique_key = ?", uniqueKey).
		Get(notification)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, db.ErrNotExist{Resource: "notification", ID: issueID}
	}
	return notification, nil
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
	if err = n.loadCommit(ctx); err != nil {
		return err
	}
	if err = n.loadRelease(ctx); err != nil {
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

func (n *Notification) loadCommit(ctx context.Context) (err error) {
	if n.Source != NotificationSourceCommit || n.CommitID == "" || n.Commit != nil {
		return nil
	}

	if n.Repository == nil {
		if err := n.loadRepo(ctx); err != nil {
			return err
		}
	}

	repo, err := gitrepo.OpenRepository(ctx, n.Repository)
	if err != nil {
		return fmt.Errorf("OpenRepository [%d]: %w", n.Repository.ID, err)
	}
	defer repo.Close()

	n.Commit, err = repo.GetCommit(n.CommitID)
	if err != nil {
		return fmt.Errorf("Notification[%d]: Failed to get repo for commit %s: %v", n.ID, n.CommitID, err)
	}
	return nil
}

func (n *Notification) loadRelease(ctx context.Context) (err error) {
	if n.Release == nil && n.ReleaseID != 0 {
		n.Release, err = repo_model.GetReleaseByID(ctx, n.ReleaseID)
		if err != nil {
			return fmt.Errorf("GetReleaseByID [%d]: %w", n.ReleaseID, err)
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
		return n.Issue.HTMLURL(ctx)
	case NotificationSourceCommit:
		return n.Repository.HTMLURL(ctx) + "/commit/" + url.PathEscape(n.CommitID)
	case NotificationSourceRepository:
		return n.Repository.HTMLURL(ctx)
	case NotificationSourceRelease:
		if n.Release == nil {
			return ""
		}
		return n.Release.HTMLURL()
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
	case NotificationSourceRelease:
		if n.Release == nil {
			return ""
		}
		return n.Release.Link()
	}
	return ""
}

func (n *Notification) IconHTML(ctx context.Context) template.HTML {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		// n.Issue should be loaded before calling this method
		return n.Issue.IconHTML(ctx)
	case NotificationSourceCommit:
		return svg.RenderHTML("octicon-git-commit", 16, "tw-text-text-light")
	case NotificationSourceRepository:
		return svg.RenderHTML("octicon-repo", 16, "tw-text-text-light")
	case NotificationSourceRelease:
		return svg.RenderHTML("octicon-tag", 16, "tw-text-text-light")
	default:
		return ""
	}
}

// APIURL formats a URL-string to the notification
func (n *Notification) APIURL() string {
	return setting.AppURL + "api/v1/notifications/threads/" + strconv.FormatInt(n.ID, 10)
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

// SetIssueReadBy sets issue to be read by given user.
func SetIssueReadBy(ctx context.Context, issueID, userID int64) error {
	if err := issues_model.UpdateIssueUserByRead(ctx, userID, issueID); err != nil {
		return err
	}

	return setIssueNotificationStatusReadIfUnread(ctx, userID, issueID)
}

func setIssueNotificationStatusReadIfUnread(ctx context.Context, userID, issueID int64) error {
	notification, err := GetIssueNotification(ctx, userID, issueID)
	if err != nil {
		if db.IsErrNotExist(err) {
			return nil
		}
		return err
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
	return setNotificationStatusReadIfUnreadByUniqueKey(ctx, userID, uniqueKeyForRepositoryNotification(repoID))
}

// SetReleaseReadBy sets release notification to be read by given user.
func SetReleaseReadBy(ctx context.Context, releaseID, userID int64) error {
	return setNotificationStatusReadIfUnreadByUniqueKey(ctx, userID, UniqueKeyForReleaseNotification(releaseID))
}

// SetCommitReadBy sets commit notification to be read by given user.
func SetCommitReadBy(ctx context.Context, repoID, userID int64, commitID string) error {
	return setNotificationStatusReadIfUnreadByUniqueKey(ctx, userID, uniqueKeyForCommitNotification(repoID, commitID))
}

func setNotificationStatusReadIfUnreadByUniqueKey(ctx context.Context, userID int64, uniqueKey string) error {
	notification := new(Notification)
	ok, err := db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("unique_key = ?", uniqueKey).
		Get(notification)
	if err != nil || !ok {
		return err
	}
	if notification.Status != NotificationStatusUnread {
		return nil
	}

	notification.Status = NotificationStatusRead
	_, err = db.GetEngine(ctx).ID(notification.ID).Cols("status").Update(notification)
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

// UpdateNotificationStatuses updates the statuses of all of a user's notifications that are of the currentStatus type to the desiredStatus
func UpdateNotificationStatuses(ctx context.Context, user *user_model.User, currentStatus, desiredStatus NotificationStatus) error {
	n := &Notification{Status: desiredStatus, UpdatedBy: user.ID}
	_, err := db.GetEngine(ctx).
		Where("user_id = ? AND status = ?", user.ID, currentStatus).
		Cols("status", "updated_by", "updated_unix").
		Update(n)
	return err
}
