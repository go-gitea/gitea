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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm"
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

// FindNotificationOptions represent the filters for notifications. If an ID is 0 it will be ignored.
type FindNotificationOptions struct {
	db.ListOptions
	UserID            int64
	RepoID            int64
	IssueID           int64
	Status            []NotificationStatus
	Source            []NotificationSource
	UpdatedAfterUnix  int64
	UpdatedBeforeUnix int64
}

// ToCond will convert each condition into a xorm-Cond
func (opts *FindNotificationOptions) ToCond() builder.Cond {
	cond := builder.NewCond()
	if opts.UserID != 0 {
		cond = cond.And(builder.Eq{"notification.user_id": opts.UserID})
	}
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"notification.repo_id": opts.RepoID})
	}
	if opts.IssueID != 0 {
		cond = cond.And(builder.Eq{"notification.issue_id": opts.IssueID})
	}
	if len(opts.Status) > 0 {
		cond = cond.And(builder.In("notification.status", opts.Status))
	}
	if len(opts.Source) > 0 {
		cond = cond.And(builder.In("notification.source", opts.Source))
	}
	if opts.UpdatedAfterUnix != 0 {
		cond = cond.And(builder.Gte{"notification.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		cond = cond.And(builder.Lte{"notification.updated_unix": opts.UpdatedBeforeUnix})
	}
	return cond
}

// ToSession will convert the given options to a xorm Session by using the conditions from ToCond and joining with issue table if required
func (opts *FindNotificationOptions) ToSession(ctx context.Context) *xorm.Session {
	sess := db.GetEngine(ctx).Where(opts.ToCond())
	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)
	}
	return sess
}

// GetNotifications returns all notifications that fit to the given options.
func GetNotifications(ctx context.Context, options *FindNotificationOptions) (nl NotificationList, err error) {
	err = options.ToSession(ctx).OrderBy("notification.updated_unix DESC").Find(&nl)
	return nl, err
}

// CountNotifications count all notifications that fit to the given options and ignore pagination.
func CountNotifications(ctx context.Context, opts *FindNotificationOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.ToCond()).Count(&Notification{})
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

func getNotificationsByIssueID(ctx context.Context, issueID int64) (notifications []*Notification, err error) {
	err = db.GetEngine(ctx).
		Where("issue_id = ?", issueID).
		Find(&notifications)
	return notifications, err
}

func notificationExists(notifications []*Notification, issueID, userID int64) bool {
	for _, notification := range notifications {
		if notification.IssueID == issueID && notification.UserID == userID {
			return true
		}
	}

	return false
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

// NotificationsForUser returns notifications for a given user and status
func NotificationsForUser(ctx context.Context, user *user_model.User, statuses []NotificationStatus, page, perPage int) (notifications NotificationList, err error) {
	if len(statuses) == 0 {
		return nil, nil
	}

	sess := db.GetEngine(ctx).
		Where("user_id = ?", user.ID).
		In("status", statuses).
		OrderBy("updated_unix DESC")

	if page > 0 && perPage > 0 {
		sess.Limit(perPage, (page-1)*perPage)
	}

	err = sess.Find(&notifications)
	return notifications, err
}

// CountUnread count unread notifications for a user
func CountUnread(ctx context.Context, userID int64) int64 {
	exist, err := db.GetEngine(ctx).Where("user_id = ?", userID).And("status = ?", NotificationStatusUnread).Count(new(Notification))
	if err != nil {
		log.Error("countUnread", err)
		return 0
	}
	return exist
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

// NotificationList contains a list of notifications
type NotificationList []*Notification

// LoadAttributes load Repo Issue User and Comment if not loaded
func (nl NotificationList) LoadAttributes(ctx context.Context) error {
	var err error
	for i := 0; i < len(nl); i++ {
		err = nl[i].LoadAttributes(ctx)
		if err != nil && !issues_model.IsErrCommentNotExist(err) {
			return err
		}
	}
	return nil
}

func (nl NotificationList) getPendingRepoIDs() []int64 {
	ids := make(container.Set[int64], len(nl))
	for _, notification := range nl {
		if notification.Repository != nil {
			continue
		}
		ids.Add(notification.RepoID)
	}
	return ids.Values()
}

// LoadRepos loads repositories from database
func (nl NotificationList) LoadRepos(ctx context.Context) (repo_model.RepositoryList, []int, error) {
	if len(nl) == 0 {
		return repo_model.RepositoryList{}, []int{}, nil
	}

	repoIDs := nl.getPendingRepoIDs()
	repos := make(map[int64]*repo_model.Repository, len(repoIDs))
	left := len(repoIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", repoIDs[:limit]).
			Rows(new(repo_model.Repository))
		if err != nil {
			return nil, nil, err
		}

		for rows.Next() {
			var repo repo_model.Repository
			err = rows.Scan(&repo)
			if err != nil {
				rows.Close()
				return nil, nil, err
			}

			repos[repo.ID] = &repo
		}
		_ = rows.Close()

		left -= limit
		repoIDs = repoIDs[limit:]
	}

	failed := []int{}

	reposList := make(repo_model.RepositoryList, 0, len(repoIDs))
	for i, notification := range nl {
		if notification.Repository == nil {
			notification.Repository = repos[notification.RepoID]
		}
		if notification.Repository == nil {
			log.Error("Notification[%d]: RepoID: %d not found", notification.ID, notification.RepoID)
			failed = append(failed, i)
			continue
		}
		var found bool
		for _, r := range reposList {
			if r.ID == notification.RepoID {
				found = true
				break
			}
		}
		if !found {
			reposList = append(reposList, notification.Repository)
		}
	}
	return reposList, failed, nil
}

func (nl NotificationList) getPendingIssueIDs() []int64 {
	ids := make(container.Set[int64], len(nl))
	for _, notification := range nl {
		if notification.Issue != nil {
			continue
		}
		ids.Add(notification.IssueID)
	}
	return ids.Values()
}

// LoadIssues loads issues from database
func (nl NotificationList) LoadIssues(ctx context.Context) ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	issueIDs := nl.getPendingIssueIDs()
	issues := make(map[int64]*issues_model.Issue, len(issueIDs))
	left := len(issueIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", issueIDs[:limit]).
			Rows(new(issues_model.Issue))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var issue issues_model.Issue
			err = rows.Scan(&issue)
			if err != nil {
				rows.Close()
				return nil, err
			}

			issues[issue.ID] = &issue
		}
		_ = rows.Close()

		left -= limit
		issueIDs = issueIDs[limit:]
	}

	failures := []int{}

	for i, notification := range nl {
		if notification.Issue == nil {
			notification.Issue = issues[notification.IssueID]
			if notification.Issue == nil {
				if notification.IssueID != 0 {
					log.Error("Notification[%d]: IssueID: %d Not Found", notification.ID, notification.IssueID)
					failures = append(failures, i)
				}
				continue
			}
			notification.Issue.Repo = notification.Repository
		}
	}
	return failures, nil
}

// Without returns the notification list without the failures
func (nl NotificationList) Without(failures []int) NotificationList {
	if len(failures) == 0 {
		return nl
	}
	remaining := make([]*Notification, 0, len(nl))
	last := -1
	var i int
	for _, i = range failures {
		remaining = append(remaining, nl[last+1:i]...)
		last = i
	}
	if len(nl) > i {
		remaining = append(remaining, nl[i+1:]...)
	}
	return remaining
}

func (nl NotificationList) getPendingCommentIDs() []int64 {
	ids := make(container.Set[int64], len(nl))
	for _, notification := range nl {
		if notification.CommentID == 0 || notification.Comment != nil {
			continue
		}
		ids.Add(notification.CommentID)
	}
	return ids.Values()
}

// LoadComments loads comments from database
func (nl NotificationList) LoadComments(ctx context.Context) ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	commentIDs := nl.getPendingCommentIDs()
	comments := make(map[int64]*issues_model.Comment, len(commentIDs))
	left := len(commentIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", commentIDs[:limit]).
			Rows(new(issues_model.Comment))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var comment issues_model.Comment
			err = rows.Scan(&comment)
			if err != nil {
				rows.Close()
				return nil, err
			}

			comments[comment.ID] = &comment
		}
		_ = rows.Close()

		left -= limit
		commentIDs = commentIDs[limit:]
	}

	failures := []int{}
	for i, notification := range nl {
		if notification.CommentID > 0 && notification.Comment == nil && comments[notification.CommentID] != nil {
			notification.Comment = comments[notification.CommentID]
			if notification.Comment == nil {
				log.Error("Notification[%d]: CommentID[%d] failed to load", notification.ID, notification.CommentID)
				failures = append(failures, i)
				continue
			}
			notification.Comment.Issue = notification.Issue
		}
	}
	return failures, nil
}

// GetNotificationCount returns the notification count for user
func GetNotificationCount(ctx context.Context, user *user_model.User, status NotificationStatus) (count int64, err error) {
	count, err = db.GetEngine(ctx).
		Where("user_id = ?", user.ID).
		And("status = ?", status).
		Count(&Notification{})
	return count, err
}

// UserIDCount is a simple coalition of UserID and Count
type UserIDCount struct {
	UserID int64
	Count  int64
}

// GetUIDsAndNotificationCounts between the two provided times
func GetUIDsAndNotificationCounts(ctx context.Context, since, until timeutil.TimeStamp) ([]UserIDCount, error) {
	sql := `SELECT user_id, count(*) AS count FROM notification ` +
		`WHERE user_id IN (SELECT user_id FROM notification WHERE updated_unix >= ? AND ` +
		`updated_unix < ?) AND status = ? GROUP BY user_id`
	var res []UserIDCount
	return res, db.GetEngine(ctx).SQL(sql, since, until, NotificationStatusUnread).Find(&res)
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
