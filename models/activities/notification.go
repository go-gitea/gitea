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
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/svg"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/xorm"
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

	// SubjectID and SubjectRef identify what the notification is about, together with
	// RepoID and Source. SubjectID holds an issue or release id; SubjectRef holds a commit
	// sha. A repository notification needs neither, RepoID alone identifies it.
	SubjectID  int64  `xorm:"NOT NULL DEFAULT 0"`
	SubjectRef string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`

	// Title is snapshotted when the notification is written so the list can be rendered
	// from the row alone, without loading (or failing to load) the subject.
	Title string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`

	// CommentID is a cursor to the comment that last updated an issue notification,
	// it is not part of the notification's identity.
	CommentID int64

	UpdatedBy int64 `xorm:"NOT NULL"`

	Issue      *issues_model.Issue    `xorm:"-"`
	Repository *repo_model.Repository `xorm:"-"`
	Comment    *issues_model.Comment  `xorm:"-"`
	User       *user_model.User       `xorm:"-"`
	Release    *repo_model.Release    `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

// IssueID returns the issue this notification is about, or 0 for other sources.
func (n *Notification) IssueID() int64 {
	if n.Source == NotificationSourceIssue || n.Source == NotificationSourcePullRequest {
		return n.SubjectID
	}
	return 0
}

// ReleaseID returns the release this notification is about, or 0 for other sources.
func (n *Notification) ReleaseID() int64 {
	if n.Source == NotificationSourceRelease {
		return n.SubjectID
	}
	return 0
}

// CommitID returns the commit sha this notification is about, or "" for other sources.
func (n *Notification) CommitID() string {
	if n.Source == NotificationSourceCommit {
		return n.SubjectRef
	}
	return ""
}

// TableIndices implements xorm's TableIndices interface
func (n *Notification) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 7)
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

	// the unique index below leads with user_id, so subject-only lookups and deletions need their own
	subjectIndex := schemas.NewIndex("idx_notification_subject", schemas.IndexType)
	subjectIndex.AddColumn("source", "subject_id")
	indices = append(indices, subjectIndex)

	uniqueNotificationKey := schemas.NewIndex("unique_notification_subject", schemas.UniqueType)
	uniqueNotificationKey.AddColumn("user_id", "repo_id", "source", "subject_id", "subject_ref")
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

// subjectCond matches the row identified by the unique index on
// (user_id, repo_id, source, subject_id, subject_ref).
func subjectCond(ctx context.Context, n *Notification) *xorm.Session {
	return db.GetEngine(ctx).
		Where("user_id = ?", n.UserID).
		And("repo_id = ?", n.RepoID).
		And("source = ?", n.Source).
		And("subject_id = ?", n.SubjectID).
		And("subject_ref = ?", n.SubjectRef)
}

// upsertNotification marks an existing notification unread, or inserts it when the user has
// no notification for that subject yet. It reports whether the user's unread count changed,
// so callers can skip the real-time push when it did not.
//
// Look the row up first and insert only when there is none. Two concurrent callers can both
// find nothing and both try to insert; the unique index rejects the loser, which then falls
// back to the update path — after the constraint fires the row provably exists.
func upsertNotification(ctx context.Context, doerID int64, n *Notification) (bool, error) {
	existing, err := findNotificationBySubject(ctx, n)
	if err != nil {
		return false, err
	}

	if existing == nil {
		n.Status = NotificationStatusUnread
		n.UpdatedBy = doerID
		insertErr := db.Insert(ctx, n)
		if insertErr == nil {
			return true, nil
		}
		// A concurrent insert won the race, so the row exists now: load and update it.
		// Any other insert failure leaves no row, and must be reported rather than dropped.
		n.ID = 0
		if existing, err = findNotificationBySubject(ctx, n); err != nil {
			return false, err
		}
		if existing == nil {
			return false, insertErr
		}
	}

	// Only a read notification moves the unread count, and a pinned one keeps its pin,
	// matching updateIssueNotification and UpdateNotificationStatuses.
	countChanged := existing.Status == NotificationStatusRead
	cols := []string{"updated_by"}
	if countChanged {
		existing.Status = NotificationStatusUnread
		cols = append(cols, "status")
	}
	existing.UpdatedBy = doerID
	if _, err := db.GetEngine(ctx).ID(existing.ID).Cols(cols...).Update(existing); err != nil {
		return false, err
	}
	return countChanged, nil
}

func findNotificationBySubject(ctx context.Context, subject *Notification) (*Notification, error) {
	notification := new(Notification)
	ok, err := subjectCond(ctx, subject).Get(notification)
	if err != nil || !ok {
		return nil, err
	}
	return notification, nil
}

// CreateRepoTransferNotification creates a notification for the user a repository was transferred to
func CreateRepoTransferNotification(ctx context.Context, doerID, repoID, receiverID int64, title string) (bool, error) {
	return upsertNotification(ctx, doerID, &Notification{
		Source: NotificationSourceRepository,
		UserID: receiverID,
		RepoID: repoID, // a repository notification needs no further subject
		Title:  title,
	})
}

// CreateCommitNotification notifies receiverID about a commit mentioning them.
func CreateCommitNotification(ctx context.Context, doerID, repoID int64, commitID string, receiverID int64, title string) (bool, error) {
	return upsertNotification(ctx, doerID, &Notification{
		Source:     NotificationSourceCommit,
		UserID:     receiverID,
		RepoID:     repoID,
		SubjectRef: commitID,
		Title:      title,
	})
}

// CreateReleaseNotification notifies receiverID about a published release.
func CreateReleaseNotification(ctx context.Context, doerID, repoID, releaseID, receiverID int64, title string) (bool, error) {
	return upsertNotification(ctx, doerID, &Notification{
		Source:    NotificationSourceRelease,
		UserID:    receiverID,
		RepoID:    repoID,
		SubjectID: releaseID,
		Title:     title,
	})
}

func createIssueNotification(ctx context.Context, userID int64, issue *issues_model.Issue, commentID, updatedByID int64) error {
	notification := &Notification{
		Source:    NotificationSourceForIssue(issue),
		UserID:    userID,
		RepoID:    issue.RepoID,
		SubjectID: issue.ID,
		Title:     issue.Title,
		Status:    NotificationStatusUnread,
		CommentID: commentID,
		UpdatedBy: updatedByID,
	}
	insertErr := db.Insert(ctx, notification)
	if insertErr == nil {
		return nil
	}
	// Two queue workers can both find no row and both insert; the unique index rejects the
	// loser, whose transaction would otherwise roll back every recipient of the same event.
	notification.ID = 0
	existing, err := findNotificationBySubject(ctx, notification)
	if err != nil || existing == nil {
		return insertErr
	}
	return updateIssueNotification(ctx, existing, commentID, updatedByID)
}

func updateIssueNotification(ctx context.Context, notification *Notification, commentID, updatedByID int64) error {
	// NOTICE: Only update comment id when the before notification on this issue is read, otherwise you may miss some old comments.
	// But we need update update_by so that the notification will be reorder
	notification.UpdatedBy = updatedByID
	cols := []string{"updated_by"}
	if notification.Status == NotificationStatusRead {
		notification.Status = NotificationStatusUnread
		notification.CommentID = commentID
		cols = append(cols, "status", "comment_id")
	}

	_, err := db.GetEngine(ctx).ID(notification.ID).Cols(cols...).Update(notification)
	return err
}

// GetIssueNotification return the notification about an issue
func GetIssueNotification(ctx context.Context, userID, issueID int64) (*Notification, error) {
	notification := new(Notification)
	ok, err := db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("source IN (?, ?)", NotificationSourceIssue, NotificationSourcePullRequest).
		And("subject_id = ?", issueID).
		Get(notification)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, db.ErrNotExist{Resource: "notification", ID: issueID}
	}
	return notification, nil
}

// LoadAttributes loads the repo, user and comment of a notification, then makes a
// best-effort attempt at the subject. A missing or unreadable subject is not an error:
// the notification still renders from its stored Title.
func (n *Notification) LoadAttributes(ctx context.Context) error {
	if err := n.loadRepo(ctx); err != nil {
		return err
	}
	if err := n.loadUser(ctx); err != nil {
		return err
	}
	if err := n.loadComment(ctx); err != nil {
		return err
	}
	n.LoadSubject(ctx)
	return nil
}

// LoadSubject enriches the notification with its live subject when it is still available.
// Errors are deliberately swallowed — a deleted issue or release must not stop the
// notification list from rendering.
func (n *Notification) LoadSubject(ctx context.Context) {
	if err := n.loadIssue(ctx); err != nil {
		log.Debug("Notification[%d]: subject issue %d unavailable: %v", n.ID, n.SubjectID, err)
	}
	if err := n.loadRelease(ctx); err != nil {
		log.Debug("Notification[%d]: subject release %d unavailable: %v", n.ID, n.SubjectID, err)
	}
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
	if n.Issue == nil && n.IssueID() != 0 {
		n.Issue, err = issues_model.GetIssueByID(ctx, n.IssueID())
		if err != nil {
			return fmt.Errorf("getIssueByID [%d]: %w", n.IssueID(), err)
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
					IssueID: n.IssueID(),
				}
			}
			return err
		}
	}
	return nil
}

func (n *Notification) loadRelease(ctx context.Context) (err error) {
	if n.Release == nil && n.ReleaseID() != 0 {
		n.Release, err = repo_model.GetReleaseByID(ctx, n.ReleaseID())
		if err != nil {
			return fmt.Errorf("GetReleaseByID [%d]: %w", n.ReleaseID(), err)
		}
		if err = n.loadRepo(ctx); err != nil {
			return err
		}
		n.Release.Repo = n.Repository // GetReleaseByID leaves Repo nil, but Release.Link() dereferences it
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

// HTMLURL formats a URL-string to the notification. The subject may be unloaded or gone,
// in which case it falls back to the repository, which is always loaded.
func (n *Notification) HTMLURL(ctx context.Context) string {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Comment != nil {
			return n.Comment.HTMLURL(ctx)
		}
		if n.Issue != nil {
			return n.Issue.HTMLURL(ctx)
		}
	case NotificationSourceCommit:
		return n.Repository.HTMLURL(ctx) + "/commit/" + url.PathEscape(n.SubjectRef)
	case NotificationSourceRelease:
		if n.Release != nil {
			return n.Release.HTMLURL()
		}
	}
	return n.Repository.HTMLURL(ctx)
}

// Link formats a relative URL-string to the notification, with the same fallback as HTMLURL.
func (n *Notification) Link(ctx context.Context) string {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Comment != nil {
			return n.Comment.Link(ctx)
		}
		if n.Issue != nil {
			return n.Issue.Link()
		}
	case NotificationSourceCommit:
		return n.Repository.Link() + "/commit/" + url.PathEscape(n.SubjectRef)
	case NotificationSourceRelease:
		if n.Release != nil {
			return n.Release.Link()
		}
	}
	return n.Repository.Link()
}

func (n *Notification) IconHTML(ctx context.Context) template.HTML {
	switch n.Source {
	case NotificationSourceIssue, NotificationSourcePullRequest:
		if n.Issue != nil {
			return n.Issue.IconHTML(ctx)
		}
		// the issue is gone or was not loaded, fall back to a state-less icon
		return svg.RenderHTML(util.Iif(n.Source == NotificationSourcePullRequest, "octicon-git-pull-request", "octicon-issue-opened"), 16, "tw-text-text-light")
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

// DisplayTitle returns the text to show for the notification. It prefers the live subject
// when one was loaded, so a renamed issue shows its current title, and otherwise falls
// back to the title snapshotted when the notification was written.
func (n *Notification) DisplayTitle() string {
	switch {
	case n.Issue != nil:
		return n.Issue.Title
	case n.Release != nil:
		return n.Release.Title
	case n.Title != "":
		return n.Title
	case n.Repository != nil:
		return n.Repository.FullName()
	}
	return ""
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
	if err != nil {
		if db.IsErrNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return markReadIfUnread(ctx, notification)
}

// SetRepoReadBy sets repo to be visited by given user.
func SetRepoReadBy(ctx context.Context, userID, repoID int64) (bool, error) {
	return setSubjectReadIfUnread(ctx, &Notification{
		UserID: userID,
		RepoID: repoID,
		Source: NotificationSourceRepository,
	})
}

// SetReleaseReadBy sets release notification to be read by given user.
func SetReleaseReadBy(ctx context.Context, releaseID, userID int64) (bool, error) {
	// The caller does not know the repository, but a release id already identifies one
	// release, so match on the subject alone here.
	notification := new(Notification)
	ok, err := db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("source = ?", NotificationSourceRelease).
		And("subject_id = ?", releaseID).
		Get(notification)
	if err != nil || !ok {
		return false, err
	}
	return markReadIfUnread(ctx, notification)
}

// SetCommitReadBy sets commit notification to be read by given user.
func SetCommitReadBy(ctx context.Context, repoID, userID int64, commitID string) (bool, error) {
	return setSubjectReadIfUnread(ctx, &Notification{
		UserID:     userID,
		RepoID:     repoID,
		Source:     NotificationSourceCommit,
		SubjectRef: commitID,
	})
}

func setSubjectReadIfUnread(ctx context.Context, subject *Notification) (bool, error) {
	notification, err := findNotificationBySubject(ctx, subject)
	if err != nil || notification == nil {
		return false, err
	}
	return markReadIfUnread(ctx, notification)
}

// markReadIfUnread reports whether the unread count actually decreased, so callers can
// skip the real-time push on a no-op.
func markReadIfUnread(ctx context.Context, notification *Notification) (bool, error) {
	if notification.Status != NotificationStatusUnread {
		return false, nil
	}

	notification.Status = NotificationStatusRead
	if _, err := db.GetEngine(ctx).ID(notification.ID).Cols("status").Update(notification); err != nil {
		return false, err
	}
	return true, nil
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

// DeleteIssueNotifications removes every notification pointing at a deleted issue.
// It cannot be expressed as a bean literal because a bare subject id would also match
// a release with the same id, so the source has to be part of the condition.
func DeleteIssueNotifications(ctx context.Context, issueID int64) error {
	_, err := db.GetEngine(ctx).
		Where("source IN (?, ?)", NotificationSourceIssue, NotificationSourcePullRequest).
		And("subject_id = ?", issueID).
		Delete(new(Notification))
	return err
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
