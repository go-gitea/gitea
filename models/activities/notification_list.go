// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// FindNotificationOptions represent the filters for notifications. If an ID is 0 it will be ignored.
type FindNotificationOptions struct {
	db.ListOptions
	UserID            int64
	RepoID            int64
	Status            []NotificationStatus
	Source            []NotificationSource
	UpdatedAfterUnix  int64
	UpdatedBeforeUnix int64

	// SubjectID and SubjectRef narrow the search to one subject. Combine them with Source
	// to hit the index on (source, subject_id).
	SubjectID  int64
	SubjectRef string
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
	if opts.SubjectID != 0 {
		cond = cond.And(builder.Eq{"notification.subject_id": opts.SubjectID})
	}
	if opts.SubjectRef != "" {
		cond = cond.And(builder.Eq{"notification.subject_ref": opts.SubjectRef})
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

func (opts *FindNotificationOptions) FilterByIssue(issueID int64, isPull bool) {
	opts.Source = []NotificationSource{util.Iif(isPull, NotificationSourcePullRequest, NotificationSourceIssue)}
	opts.SubjectID = issueID
}

func (opts *FindNotificationOptions) FilterByCommit(repoID int64, commitID string) {
	opts.Source = []NotificationSource{NotificationSourceCommit}
	opts.RepoID = repoID
	opts.SubjectRef = commitID
}

func (opts *FindNotificationOptions) FilterByRelease(releaseID int64) {
	opts.Source = []NotificationSource{NotificationSourceRelease}
	opts.SubjectID = releaseID
}

// CreateOrUpdateIssueNotifications creates an issue notification
// for each watcher, or updates it if already exists
// receiverID > 0 just send to receiver, else send to all watcher
// Returns the set of user IDs whose notification rows were created or updated.
func CreateOrUpdateIssueNotifications(ctx context.Context, issueID, commentID, notificationAuthorID, receiverID int64) ([]int64, error) {
	var notifiedIDs []int64
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var innerErr error
		notifiedIDs, innerErr = createOrUpdateIssueNotifications(ctx, issueID, commentID, notificationAuthorID, receiverID)
		return innerErr
	})
	return notifiedIDs, err
}

func createOrUpdateIssueNotifications(ctx context.Context, issueID, commentID, notificationAuthorID, receiverID int64) ([]int64, error) {
	// init
	var toNotify container.Set[int64]
	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return nil, err
	}

	if receiverID > 0 {
		toNotify = make(container.Set[int64], 1)
		toNotify.Add(receiverID)
	} else {
		toNotify = make(container.Set[int64], 32)
		issueWatches, err := issues_model.GetIssueWatchersIDs(ctx, issueID, true)
		if err != nil {
			return nil, err
		}
		toNotify.AddMultiple(issueWatches...)
		if !(issue.IsPull && issues_model.HasWorkInProgressPrefix(issue.Title)) {
			watchType := util.Iif(issue.IsPull, repo_model.WatchPullRequests, repo_model.WatchIssues)
			repoWatches, err := repo_model.GetRepoWatchersIDs(ctx, issue.RepoID, watchType)
			if err != nil {
				return nil, err
			}
			toNotify.AddMultiple(repoWatches...)
		}
		issueParticipants, err := issue.GetParticipantIDsByIssue(ctx)
		if err != nil {
			return nil, err
		}
		toNotify.AddMultiple(issueParticipants...)
		issueAssignees, err := issues_model.GetAssigneeIDsByIssue(ctx, issueID)
		if err != nil {
			return nil, err
		}
		toNotify.AddMultiple(issueAssignees...)
		if issue.IsPull {
			issueReviewers, err := issues_model.GetPullRequestRequestedReviewerIDs(ctx, issueID)
			if err != nil {
				return nil, err
			}
			toNotify.AddMultiple(issueReviewers...)
		}

		// don't notify user who cause notification
		delete(toNotify, notificationAuthorID)
		// explicit unwatch on issue
		issueUnWatches, err := issues_model.GetIssueWatchersIDs(ctx, issueID, false)
		if err != nil {
			return nil, err
		}
		for _, id := range issueUnWatches {
			toNotify.Remove(id)
		}
	}

	// muting the repository outranks every other source, including mentions
	ignorers, err := repo_model.GetRepoIgnorersIDs(ctx, issue.RepoID)
	if err != nil {
		return nil, err
	}
	for _, id := range ignorers {
		toNotify.Remove(id)
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	requiredUnit := util.Iif(issue.IsPull, unit.TypePullRequests, unit.TypeIssues)

	// notify
	notifiedIDs := make([]int64, 0, len(toNotify))
	for userID := range toNotify {
		issue.Repo.Units = nil
		user, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				continue
			}

			return nil, err
		}
		if !access_model.CheckRepoUnitUser(ctx, issue.Repo, user, requiredUnit) {
			continue
		}

		existing, err := GetIssueNotification(ctx, userID, issue.ID)
		if err != nil && !db.IsErrNotExist(err) {
			return nil, err
		}
		if existing != nil {
			if err = updateIssueNotification(ctx, existing, commentID, notificationAuthorID); err != nil {
				return nil, err
			}
			notifiedIDs = append(notifiedIDs, userID)
			continue
		}
		if err = createIssueNotification(ctx, userID, issue, commentID, notificationAuthorID); err != nil {
			return nil, err
		}
		notifiedIDs = append(notifiedIDs, userID)
	}
	return notifiedIDs, nil
}

// NotificationList contains a list of notifications
type NotificationList []*Notification

// LoadAttributes loads the repo, user and comment of every notification, then makes a
// best-effort attempt at the subjects. It returns the indices of notifications whose
// repository could not be loaded — those are the only ones that cannot be rendered.
// A missing issue or release is not a failure: the notification still has its Title.
func (nl NotificationList) LoadAttributes(ctx context.Context) ([]int, error) {
	repos, failures, err := nl.LoadRepos(ctx)
	if err != nil {
		return nil, err
	}
	if err := repos.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	if _, err := nl.LoadUsers(ctx); err != nil {
		return nil, err
	}
	nl.LoadSubjects(ctx) // before LoadComments, so it can reuse the loaded issues
	if _, err := nl.LoadComments(ctx); err != nil {
		return nil, err
	}
	return failures, nil
}

// LoadSubjects enriches the list with the live issues and releases that still exist, so a
// renamed issue shows its current title. Every error is swallowed — notifications render
// from their stored Title when the subject is gone.
func (nl NotificationList) LoadSubjects(ctx context.Context) {
	if _, err := nl.LoadIssues(ctx); err != nil {
		log.Debug("LoadIssues: %v", err)
		return
	}
	if err := nl.LoadIssuePullRequests(ctx); err != nil {
		log.Debug("LoadIssuePullRequests: %v", err)
	}
	if _, err := nl.LoadReleases(ctx); err != nil {
		log.Debug("LoadReleases: %v", err)
	}
}

func (nl NotificationList) getPendingRepoIDs() []int64 {
	return container.FilterSlice(nl, func(n *Notification) (int64, bool) {
		if n.Repository != nil {
			return 0, false
		}
		return n.RepoID, true
	})
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
		limit := min(left, db.DefaultMaxInSize)
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
		if notification.Issue != nil || notification.IssueID() == 0 {
			continue
		}
		ids.Add(notification.IssueID())
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
		limit := min(left, db.DefaultMaxInSize)
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
		if notification.Issue != nil || notification.IssueID() == 0 {
			continue
		}
		notification.Issue = issues[notification.IssueID()]
		if notification.Issue == nil {
			// the issue is gone; the notification still renders from its stored Title
			log.Debug("Notification[%d]: issue %d not found", notification.ID, notification.IssueID())
			failures = append(failures, i)
			continue
		}
		notification.Issue.Repo = notification.Repository
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
		limit := min(left, db.DefaultMaxInSize)
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
		limit := min(left, db.DefaultMaxInSize)
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

func (nl NotificationList) getPendingReleaseIDs() []int64 {
	ids := make(container.Set[int64], len(nl))
	for _, notification := range nl {
		if notification.Release != nil || notification.ReleaseID() == 0 {
			continue
		}
		ids.Add(notification.ReleaseID())
	}
	return ids.Values()
}

func (nl NotificationList) LoadReleases(ctx context.Context) ([]int, error) {
	if len(nl) == 0 {
		return []int{}, nil
	}

	releaseIDs := nl.getPendingReleaseIDs()
	if len(releaseIDs) == 0 {
		return []int{}, nil
	}

	releases := make(map[int64]*repo_model.Release, len(releaseIDs))
	left := len(releaseIDs)
	for left > 0 {
		limit := min(left, db.DefaultMaxInSize)
		if err := db.GetEngine(ctx).In("id", releaseIDs[:limit]).Find(&releases); err != nil {
			return nil, err
		}
		left -= limit
		releaseIDs = releaseIDs[limit:]
	}

	failures := []int{}
	for i, notification := range nl {
		if notification.Release != nil || notification.ReleaseID() == 0 {
			continue
		}
		release := releases[notification.ReleaseID()]
		if release == nil {
			// the release is gone; the notification still renders from its stored Title
			log.Debug("Notification[%d]: release %d not found", notification.ID, notification.ReleaseID())
			failures = append(failures, i)
			continue
		}
		notification.Release = release
		notification.Release.Repo = notification.Repository
	}
	return failures, nil
}

// LoadIssuePullRequests loads all issues' pull requests if possible
func (nl NotificationList) LoadIssuePullRequests(ctx context.Context) error {
	issues := make(map[int64]*issues_model.Issue, len(nl))
	for _, notification := range nl {
		if notification.Issue != nil && notification.Issue.IsPull && notification.Issue.PullRequest == nil {
			issues[notification.Issue.ID] = notification.Issue
		}
	}

	if len(issues) == 0 {
		return nil
	}

	pulls, err := issues_model.GetPullRequestByIssueIDs(ctx, util.KeysOfMap(issues))
	if err != nil {
		return err
	}

	for _, pull := range pulls {
		if issue := issues[pull.IssueID]; issue != nil {
			issue.PullRequest = pull
			issue.PullRequest.Issue = issue
		}
	}

	return nil
}
