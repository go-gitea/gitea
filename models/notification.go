// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"path"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// NotificationStatus is the status of the notification (read or unread)
type NotificationStatus uint8

const (
	// NotificationStatusUnread represents an unread notification
	NotificationStatusUnread NotificationStatus = iota + 1
	// NotificationStatusRead represents a read notification
	NotificationStatusRead
	// NotificationStatusPinned represents a pinned notification
	NotificationStatusPinned
)

// NotificationSource is the source of the notification (issue, PR, commit, etc)
type NotificationSource uint8

const (
	// NotificationSourceIssue is a notification of an issue
	NotificationSourceIssue NotificationSource = iota + 1
	// NotificationSourcePullRequest is a notification of a pull request
	NotificationSourcePullRequest
	// NotificationSourceCommit is a notification of a commit
	NotificationSourceCommit
	// NotificationSourceRelease is a notifiation of a release
	NotificationSourceRelease
)

// NotificationBy is the reason why user is notified.
type NotificationBy uint8

const (
	// NotificationByWatchRepo reprensents notification according to
	NotificationByWatchRepo NotificationBy = iota + 1
	// NotificationBySubcribeIssue represents notification because subscribe to issue
	NotificationBySubcribeIssue
	// NotificationByMentioned represents notification because being mentioned on issue or comment
	NotificationByMentioned
	// NotificationByParticipated represents notification because participated in an issue
	NotificationByParticipated
	// NotificationByAssigne represents notification because being assignee
	NotificationByAssigne
)

// Notification represents a notification
type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`
	RepoID int64 `xorm:"INDEX NOT NULL"`

	Status NotificationStatus `xorm:"SMALLINT INDEX NOT NULL"`
	Source NotificationSource `xorm:"SMALLINT INDEX NOT NULL"`
	By     NotificationBy     `xorm:"SMALLINT INDEX NOT NULL"`

	IssueID   int64  `xorm:"INDEX NOT NULL"`
	CommitID  string `xorm:"INDEX"`
	CommentID int64
	ReleaseID int64

	UpdatedBy int64 `xorm:"INDEX NOT NULL"`

	Issue      *Issue      `xorm:"-"`
	Repository *Repository `xorm:"-"`
	Comment    *Comment    `xorm:"-"`
	User       *User       `xorm:"-"`
	Release    *Release    `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated INDEX NOT NULL"`
}

// FindNotificationOptions represent the filters for notifications. If an ID is 0 it will be ignored.
type FindNotificationOptions struct {
	ListOptions
	UserID            int64
	RepoID            int64
	IssueID           int64
	ReleaseID         int64
	Status            []NotificationStatus
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
	if opts.UpdatedAfterUnix != 0 {
		cond = cond.And(builder.Gte{"notification.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		cond = cond.And(builder.Lte{"notification.updated_unix": opts.UpdatedBeforeUnix})
	}
	return cond
}

// ToSession will convert the given options to a xorm Session by using the conditions from ToCond and joining with issue table if required
func (opts *FindNotificationOptions) ToSession(e Engine) *xorm.Session {
	sess := e.Where(opts.ToCond())
	if opts.Page != 0 {
		sess = opts.setSessionPagination(sess)
	}
	return sess
}

func getNotifications(e Engine, options FindNotificationOptions) (nl NotificationList, err error) {
	err = options.ToSession(e).OrderBy("notification.updated_unix DESC").Find(&nl)
	return
}

// GetNotifications returns all notifications that fit to the given options.
func GetNotifications(opts FindNotificationOptions) (NotificationList, error) {
	return getNotifications(x, opts)
}

// NotificationOpts represents notification options
type NotificationOpts struct {
	IssueID              int64
	CommentID            int64
	ReleaseID            int64
	NotificationAuthorID int64
	ReceiverID           int64 // 0 -- ALL Watcher
}

// CreateOrUpdateNotifications creates an issue notification
// for each watcher, or updates it if already exists
// receiverID > 0 just send to reciver, else send to all watcher
func CreateOrUpdateNotifications(opts *NotificationOpts) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := createOrUpdateNotifications(sess, opts); err != nil {
		return err
	}

	return sess.Commit()
}

func createOrUpdateNotifications(e Engine, opts *NotificationOpts) error {
	if opts.IssueID > 0 {
		return createOrUpdateIssueNotifications(e, opts)
	} else if opts.ReleaseID > 0 {
		return createOrUpdateReleaseNotifications(e, opts)
	}
	return errors.New("Unknow notification type")
}

func createOrUpdateIssueNotifications(e Engine, opts *NotificationOpts) error {
	// init
	var toNotify map[int64]NotificationBy
	notifications, err := getNotificationsByIssueID(e, opts.IssueID)
	if err != nil {
		return err
	}

	issue, err := getIssueByID(e, opts.IssueID)
	if err != nil {
		return err
	}

	if opts.ReceiverID > 0 {
		toNotify = make(map[int64]NotificationBy, 1)
		toNotify[opts.ReceiverID] = NotificationByAssigne
	} else {
		toNotify = make(map[int64]NotificationBy, 32)
		issueWatches, err := getIssueWatchersIDs(e, opts.IssueID, true)
		if err != nil {
			return err
		}
		for _, id := range issueWatches {
			toNotify[id] = NotificationBySubcribeIssue
		}

		repoWatches, err := getRepoWatchersIDs(e, issue.RepoID)
		if err != nil {
			return err
		}
		for _, id := range repoWatches {
			toNotify[id] = NotificationByWatchRepo
		}
		issueParticipants, err := issue.getParticipantIDsByIssue(e)
		if err != nil {
			return err
		}
		for _, id := range issueParticipants {
			toNotify[id] = NotificationByParticipated
		}

		// dont notify user who cause notification
		delete(toNotify, opts.NotificationAuthorID)
		// explicit unwatch on issue
		issueUnWatches, err := getIssueWatchersIDs(e, opts.IssueID, false)
		if err != nil {
			return err
		}
		for _, id := range issueUnWatches {
			delete(toNotify, id)
		}
	}

	err = issue.loadRepo(e)
	if err != nil {
		return err
	}

	// notify
	for userID, by := range toNotify {
		issue.Repo.Units = nil
		user, err := getUserByID(e, userID)
		if err != nil {
			if IsErrUserNotExist(err) {
				continue
			}

			return err
		}
		if issue.IsPull && !issue.Repo.checkUnitUser(e, user, UnitTypePullRequests) {
			continue
		}
		if !issue.IsPull && !issue.Repo.checkUnitUser(e, user, UnitTypeIssues) {
			continue
		}

		if issueNotificationExists(notifications, issue.ID, userID) {
			if err = updateIssueNotification(e, userID, opts, by); err != nil {
				return err
			}
			continue
		}
		if err = createNotification(e, userID, issue.Repo.ID, opts, issue.IsPull, by); err != nil {
			return err
		}
	}
	return nil
}

func createOrUpdateReleaseNotifications(e Engine, opts *NotificationOpts) error {
	// init
	var toNotify = make(map[int64]NotificationBy, 32)
	notifications, err := getNotificationsByReleaseID(e, opts.ReleaseID)
	if err != nil {
		return err
	}

	release, err := getReleaseByID(e, opts.ReleaseID)
	if err != nil {
		return err
	}

	repoWatches, err := getRepoWatchersIDs(e, release.RepoID)
	if err != nil {
		return err
	}
	for _, id := range repoWatches {
		toNotify[id] = NotificationByWatchRepo
	}

	err = release.loadRepo(e)
	if err != nil {
		return err
	}

	// notify
	for userID, by := range toNotify {
		release.Repo.Units = nil
		user, err := getUserByID(e, userID)
		if err != nil {
			if IsErrUserNotExist(err) {
				continue
			}

			return err
		}
		if !release.Repo.checkUnitUser(e, user, UnitTypeReleases) {
			continue
		}

		if releaseNotificationExists(notifications, release.ID, userID) {
			if err = updateReleaseNotification(e, userID, opts, by); err != nil {
				return err
			}
			continue
		}
		if err = createNotification(e, userID, release.Repo.ID, opts, false, by); err != nil {
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

func getNotificationsByReleaseID(e Engine, releaseID int64) (notifications []*Notification, err error) {
	err = e.
		Where("release_id = ?", releaseID).
		Find(&notifications)
	return
}

func issueNotificationExists(notifications []*Notification, issueID, userID int64) bool {
	for _, notification := range notifications {
		if notification.IssueID == issueID && notification.UserID == userID {
			return true
		}
	}

	return false
}

func releaseNotificationExists(notifications []*Notification, releaseID, userID int64) bool {
	for _, notification := range notifications {
		if notification.ReleaseID == releaseID && notification.UserID == userID {
			return true
		}
	}

	return false
}

func createNotification(e Engine, userID, repoID int64, opts *NotificationOpts, isPull bool, by NotificationBy) error {
	notification := &Notification{
		UserID:    userID,
		RepoID:    repoID,
		Status:    NotificationStatusUnread,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		ReleaseID: opts.ReleaseID,
		By:        by,
		UpdatedBy: opts.NotificationAuthorID,
	}

	if opts.IssueID > 0 {
		if isPull {
			notification.Source = NotificationSourcePullRequest
		} else {
			notification.Source = NotificationSourceIssue
		}
	} else if opts.ReleaseID > 0 {
		notification.Source = NotificationSourceRelease
	}

	_, err := e.Insert(notification)
	return err
}

func updateIssueNotification(e Engine, userID int64, opts *NotificationOpts, by NotificationBy) error {
	notification, err := getNotificationByOpts(e, userID, opts)
	if err != nil {
		return err
	}

	// NOTICE: Only update comment id when the before notification on this issue is read, otherwise you may miss some old comments.
	// But we need update update_by so that the notification will be reorder
	var cols []string
	if notification.Status == NotificationStatusRead {
		notification.Status = NotificationStatusUnread
		notification.CommentID = opts.CommentID
		cols = []string{"status", "update_by", "comment_id"}
	} else {
		notification.UpdatedBy = opts.NotificationAuthorID
		cols = []string{"update_by"}
	}
	notification.By = by
	cols = append(cols, "by")

	_, err = e.ID(notification.ID).Cols(cols...).Update(notification)
	return err
}

func updateReleaseNotification(e Engine, userID int64, opts *NotificationOpts, by NotificationBy) error {
	notification, err := getNotificationByOpts(e, userID, opts)
	if err != nil {
		return err
	}

	// NOTICE: Only update comment id when the before notification on this issue is read, otherwise you may miss some old comments.
	// But we need update update_by so that the notification will be reorder
	var cols []string
	if notification.Status == NotificationStatusRead {
		notification.Status = NotificationStatusUnread
		notification.CommentID = opts.CommentID
		cols = []string{"status", "update_by", "comment_id"}
	} else {
		notification.UpdatedBy = opts.NotificationAuthorID
		cols = []string{"update_by"}
	}
	notification.By = by
	cols = append(cols, "by")

	_, err = e.ID(notification.ID).Cols(cols...).Update(notification)
	return err
}

func getNotificationByOpts(e Engine, userID int64, opts *NotificationOpts) (*Notification, error) {
	notification := new(Notification)
	sess := e.Where("user_id = ?", userID)
	if opts.IssueID > 0 {
		sess.And("issue_id = ?", opts.IssueID)
	} else if opts.ReleaseID > 0 {
		sess.And("release_id = ?", opts.ReleaseID)
	}
	_, err := sess.Get(notification)
	return notification, err
}

// NotificationsForUser returns notifications for a given user and status
func NotificationsForUser(user *User, statuses []NotificationStatus, page, perPage int) (NotificationList, error) {
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

// CountUnread count unread notifications for a user
func CountUnread(user *User) int64 {
	return countUnread(x, user.ID)
}

func countUnread(e Engine, userID int64) int64 {
	exist, err := e.Where("user_id = ?", userID).And("status = ?", NotificationStatusUnread).Count(new(Notification))
	if err != nil {
		log.Error("countUnread", err)
		return 0
	}
	return exist
}

// LoadAttributes load Repo Issue User and Comment if not loaded
func (n *Notification) LoadAttributes() (err error) {
	return n.loadAttributes(x)
}

func (n *Notification) loadAttributes(e Engine) (err error) {
	if err = n.loadRepo(e); err != nil {
		return
	}
	if err = n.loadIssue(e); err != nil {
		return
	}
	if err = n.loadUser(e); err != nil {
		return
	}
	if err = n.loadComment(e); err != nil {
		return
	}
	if err = n.loadRelease(e); err != nil {
		return
	}
	return
}

func (n *Notification) loadRepo(e Engine) (err error) {
	if n.Repository == nil {
		n.Repository, err = getRepositoryByID(e, n.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %v", n.RepoID, err)
		}
	}
	return nil
}

func (n *Notification) loadIssue(e Engine) (err error) {
	if n.Issue == nil && n.IssueID > 0 {
		n.Issue, err = getIssueByID(e, n.IssueID)
		if err != nil {
			return fmt.Errorf("getIssueByID [%d]: %v", n.IssueID, err)
		}
		return n.Issue.loadAttributes(e)
	}
	return nil
}

func (n *Notification) loadComment(e Engine) (err error) {
	if n.Comment == nil && n.CommentID > 0 {
		n.Comment, err = getCommentByID(e, n.CommentID)
		if err != nil {
			return fmt.Errorf("GetCommentByID [%d] for issue ID [%d]: %v", n.CommentID, n.IssueID, err)
		}
	}
	return nil
}

func (n *Notification) loadUser(e Engine) (err error) {
	if n.User == nil {
		n.User, err = getUserByID(e, n.UserID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %v", n.UserID, err)
		}
	}
	return nil
}

func (n *Notification) loadRelease(e Engine) (err error) {
	if n.Release == nil && n.ReleaseID > 0 {
		n.Release, err = getReleaseByID(e, n.ReleaseID)
		if err != nil {
			return fmt.Errorf("getReleaseByID [%d]: %v", n.ReleaseID, err)
		}
		return n.Release.loadAttributes(e)
	}
	return nil
}

// GetRepo returns the repo of the notification
func (n *Notification) GetRepo() (*Repository, error) {
	return n.Repository, n.loadRepo(x)
}

// GetIssue returns the issue of the notification
func (n *Notification) GetIssue() (*Issue, error) {
	return n.Issue, n.loadIssue(x)
}

// HTMLURL formats a URL-string to the notification
func (n *Notification) HTMLURL() string {
	if n.Comment != nil {
		return n.Comment.HTMLURL()
	} else if n.Issue != nil {
		return n.Issue.HTMLURL()
	} else if n.Release != nil {
		return n.Release.HTMLURL()
	}
	return ""
}

// APIURL formats a URL-string to the notification
func (n *Notification) APIURL() string {
	return setting.AppURL + path.Join("api/v1/notifications/threads", fmt.Sprintf("%d", n.ID))
}

// NotificationList contains a list of notifications
type NotificationList []*Notification

// LoadAttributes load Repo Issue User and Comment if not loaded
func (nl NotificationList) LoadAttributes() (err error) {
	for i := 0; i < len(nl); i++ {
		err = nl[i].LoadAttributes()
		if err != nil {
			return
		}
	}
	return
}

func (nl NotificationList) getPendingRepoIDs() []int64 {
	var ids = make(map[int64]struct{}, len(nl))
	for _, notification := range nl {
		if notification.Repository != nil {
			continue
		}
		if _, ok := ids[notification.RepoID]; !ok {
			ids[notification.RepoID] = struct{}{}
		}
	}
	return keysInt64(ids)
}

// LoadRepos loads repositories from database
func (nl NotificationList) LoadRepos() (RepositoryList, []int, error) {
	if len(nl) == 0 {
		return RepositoryList{}, []int{}, nil
	}

	var repoIDs = nl.getPendingRepoIDs()
	var repos = make(map[int64]*Repository, len(repoIDs))
	var left = len(repoIDs)
	for left > 0 {
		var limit = defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := x.
			In("id", repoIDs[:limit]).
			Rows(new(Repository))
		if err != nil {
			return nil, nil, err
		}

		for rows.Next() {
			var repo Repository
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

	var reposList = make(RepositoryList, 0, len(repoIDs))
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
	var ids = make(map[int64]struct{}, len(nl))
	for _, notification := range nl {
		if notification.Issue != nil || notification.IssueID == 0 {
			continue
		}
		if _, ok := ids[notification.IssueID]; !ok {
			ids[notification.IssueID] = struct{}{}
		}
	}
	return keysInt64(ids)
}

func (nl NotificationList) getPendingReleaseIDs() []int64 {
	var ids = make(map[int64]struct{}, len(nl))
	for _, notification := range nl {
		if notification.Release != nil || notification.ReleaseID == 0 {
			continue
		}
		if _, ok := ids[notification.ReleaseID]; !ok {
			ids[notification.ReleaseID] = struct{}{}
		}
	}
	return keysInt64(ids)
}

// LoadIssues loads issues from database
func (nl NotificationList) LoadIssues() ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	var (
		issueIDs = nl.getPendingIssueIDs()
		issues   = make(map[int64]*Issue, len(issueIDs))
		left     = len(issueIDs)
	)
	for left > 0 {
		var limit = defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := x.
			In("id", issueIDs[:limit]).
			Rows(new(Issue))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var issue Issue
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

	var (
		releaseIDs   = nl.getPendingReleaseIDs()
		releases     = make(map[int64]*Release, len(releaseIDs))
		releasesLeft = len(releaseIDs)
	)
	for releasesLeft > 0 {
		var limit = defaultMaxInSize
		if releasesLeft < limit {
			limit = releasesLeft
		}
		rows, err := x.
			In("id", releaseIDs[:limit]).
			Rows(new(Release))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var release Release
			err = rows.Scan(&release)
			if err != nil {
				rows.Close()
				return nil, err
			}

			releases[release.ID] = &release
		}
		_ = rows.Close()

		releasesLeft -= limit
		releaseIDs = releaseIDs[limit:]
	}

	failures := []int{}

	for i, notification := range nl {
		if notification.Issue == nil && notification.IssueID > 0 {
			notification.Issue = issues[notification.IssueID]
			if notification.Issue == nil {
				log.Error("Notification[%d]: IssueID: %d Not Found", notification.ID, notification.IssueID)
				failures = append(failures, i)
				continue
			}
			notification.Issue.Repo = notification.Repository
		}
		if notification.Release == nil && notification.ReleaseID > 0 {
			notification.Release = releases[notification.ReleaseID]
			if notification.Release == nil {
				log.Error("Notification[%d]: ReleaseID: %d Not Found", notification.ID, notification.ReleaseID)
				failures = append(failures, i)
				continue
			}
			notification.Release.Repo = notification.Repository
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
	var ids = make(map[int64]struct{}, len(nl))
	for _, notification := range nl {
		if notification.CommentID == 0 || notification.Comment != nil {
			continue
		}
		if _, ok := ids[notification.CommentID]; !ok {
			ids[notification.CommentID] = struct{}{}
		}
	}
	return keysInt64(ids)
}

// LoadComments loads comments from database
func (nl NotificationList) LoadComments() ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	var commentIDs = nl.getPendingCommentIDs()
	var comments = make(map[int64]*Comment, len(commentIDs))
	var left = len(commentIDs)
	for left > 0 {
		var limit = defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := x.
			In("id", commentIDs[:limit]).
			Rows(new(Comment))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var comment Comment
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

// UserIDCount is a simple coalition of UserID and Count
type UserIDCount struct {
	UserID int64
	Count  int64
}

// GetUIDsAndNotificationCounts between the two provided times
func GetUIDsAndNotificationCounts(since, until timeutil.TimeStamp) ([]UserIDCount, error) {
	sql := `SELECT user_id, count(*) AS count FROM notification ` +
		`WHERE user_id IN (SELECT user_id FROM notification WHERE updated_unix >= ? AND ` +
		`updated_unix < ?) AND status = ? GROUP BY user_id`
	var res []UserIDCount
	return res, x.SQL(sql, since, until, NotificationStatusUnread).Find(&res)
}

func setNotificationStatusReadIfUnread(e Engine, userID int64, opts *NotificationOpts) error {
	notification, err := getNotificationByOpts(e, userID, opts)
	// ignore if not exists
	if err != nil {
		return nil
	}

	if notification.Status != NotificationStatusUnread {
		return nil
	}

	notification.Status = NotificationStatusRead

	_, err = e.ID(notification.ID).Update(notification)
	return err
}

// SetNotificationStatus change the notification status
func SetNotificationStatus(notificationID int64, user *User, status NotificationStatus) error {
	notification, err := getNotificationByID(x, notificationID)
	if err != nil {
		return err
	}

	if notification.UserID != user.ID {
		return fmt.Errorf("Can't change notification of another user: %d, %d", notification.UserID, user.ID)
	}

	notification.Status = status

	_, err = x.ID(notificationID).Update(notification)
	return err
}

// GetNotificationByID return notification by ID
func GetNotificationByID(notificationID int64) (*Notification, error) {
	return getNotificationByID(x, notificationID)
}

func getNotificationByID(e Engine, notificationID int64) (*Notification, error) {
	notification := new(Notification)
	ok, err := e.
		Where("id = ?", notificationID).
		Get(notification)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrNotExist{ID: notificationID}
	}

	return notification, nil
}

// UpdateNotificationStatuses updates the statuses of all of a user's notifications that are of the currentStatus type to the desiredStatus
func UpdateNotificationStatuses(user *User, currentStatus NotificationStatus, desiredStatus NotificationStatus) error {
	n := &Notification{Status: desiredStatus, UpdatedBy: user.ID}
	_, err := x.
		Where("user_id = ? AND status = ?", user.ID, currentStatus).
		Cols("status", "updated_by", "updated_unix").
		Update(n)
	return err
}
