// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

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
func (opts FindNotificationOptions) ToConds() builder.Cond {
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
		if len(opts.Status) == 1 {
			cond = cond.And(builder.Eq{"notification.status": opts.Status[0]})
		} else {
			cond = cond.And(builder.In("notification.status", opts.Status))
		}
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

func (opts FindNotificationOptions) ToOrders() string {
	return "notification.updated_unix DESC"
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

// GetUIDsAndNotificationCounts between the two provided times
func GetUIDsAndNotificationCounts(ctx context.Context, since, until timeutil.TimeStamp) ([]UserIDCount, error) {
	sql := `SELECT user_id, count(*) AS count FROM notification ` +
		`WHERE user_id IN (SELECT user_id FROM notification WHERE updated_unix >= ? AND ` +
		`updated_unix < ?) AND status = ? GROUP BY user_id`
	var res []UserIDCount
	return res, db.GetEngine(ctx).SQL(sql, since, until, NotificationStatusUnread).Find(&res)
}

// NotificationList contains a list of notifications
type NotificationList []*Notification

// LoadAttributes load Repo Issue User and Comment if not loaded
func (nl NotificationList) LoadAttributes(ctx context.Context) error {
	if _, _, err := nl.LoadRepos(ctx); err != nil {
		return err
	}
	if _, err := nl.LoadIssues(ctx); err != nil {
		return err
	}
	if _, err := nl.LoadUsers(ctx); err != nil {
		return err
	}
	if _, err := nl.LoadComments(ctx); err != nil {
		return err
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

func (nl NotificationList) getUserIDs() []int64 {
	ids := make(container.Set[int64], len(nl))
	for _, notification := range nl {
		if notification.UserID == 0 || notification.User != nil {
			continue
		}
		ids.Add(notification.UserID)
	}
	return ids.Values()
}

// LoadUsers loads users from database
func (nl NotificationList) LoadUsers(ctx context.Context) ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	userIDs := nl.getUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	left := len(userIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", userIDs[:limit]).
			Rows(new(user_model.User))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var user user_model.User
			err = rows.Scan(&user)
			if err != nil {
				rows.Close()
				return nil, err
			}

			users[user.ID] = &user
		}
		_ = rows.Close()

		left -= limit
		userIDs = userIDs[limit:]
	}

	failures := []int{}
	for i, notification := range nl {
		if notification.UserID > 0 && notification.User == nil && users[notification.UserID] != nil {
			notification.User = users[notification.UserID]
			if notification.User == nil {
				log.Error("Notification[%d]: UserID[%d] failed to load", notification.ID, notification.UserID)
				failures = append(failures, i)
				continue
			}
		}
	}
	return failures, nil
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
