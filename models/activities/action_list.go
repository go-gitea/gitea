// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ActionList defines a list of actions
type ActionList []*Action

func (actions ActionList) getUserIDs() []int64 {
	return container.FilterSlice(actions, func(action *Action) (int64, bool) {
		return action.ActUserID, true
	})
}

func (actions ActionList) LoadActUsers(ctx context.Context) (map[int64]*user_model.User, error) {
	if len(actions) == 0 {
		return nil, nil //nolint:nilnil // returns nil when there are no actions
	}

	userIDs := actions.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := db.GetEngine(ctx).
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	for _, action := range actions {
		action.ActUser = userMaps[action.ActUserID]
	}
	return userMaps, nil
}

func (actions ActionList) getRepoIDs() []int64 {
	return container.FilterSlice(actions, func(action *Action) (int64, bool) {
		return action.RepoID, true
	})
}

func (actions ActionList) LoadRepositories(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	repoIDs := actions.getRepoIDs()
	repoMaps := make(map[int64]*repo_model.Repository, len(repoIDs))
	err := db.GetEngine(ctx).In("id", repoIDs).Find(&repoMaps)
	if err != nil {
		return fmt.Errorf("find repository: %w", err)
	}
	for _, action := range actions {
		action.Repo = repoMaps[action.RepoID]
	}
	repos := repo_model.RepositoryList(util.ValuesOfMap(repoMaps))
	return repos.LoadUnits(ctx)
}

func (actions ActionList) loadRepoOwner(ctx context.Context, userMap map[int64]*user_model.User) (err error) {
	if userMap == nil {
		userMap = make(map[int64]*user_model.User)
	}

	missingUserIDs := container.FilterSlice(actions, func(action *Action) (int64, bool) {
		if action.Repo == nil {
			return 0, false
		}
		_, alreadyLoaded := userMap[action.Repo.OwnerID]
		return action.Repo.OwnerID, !alreadyLoaded
	})
	if len(missingUserIDs) == 0 {
		return nil
	}

	if err := db.GetEngine(ctx).
		In("id", missingUserIDs).
		Find(&userMap); err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	for _, action := range actions {
		if action.Repo != nil {
			action.Repo.Owner = userMap[action.Repo.OwnerID]
		}
	}

	return nil
}

// LoadAttributes loads all attributes
func (actions ActionList) LoadAttributes(ctx context.Context) error {
	// the load sequence cannot be changed because of the dependencies
	userMap, err := actions.LoadActUsers(ctx)
	if err != nil {
		return err
	}
	if err := actions.LoadRepositories(ctx); err != nil {
		return err
	}
	if err := actions.loadRepoOwner(ctx, userMap); err != nil {
		return err
	}
	if err := actions.LoadIssues(ctx); err != nil {
		return err
	}
	return actions.LoadComments(ctx)
}

func (actions ActionList) LoadComments(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	commentIDs := make([]int64, 0, len(actions))
	for _, action := range actions {
		if action.CommentID > 0 {
			commentIDs = append(commentIDs, action.CommentID)
		}
	}
	if len(commentIDs) == 0 {
		return nil
	}

	commentsMap := make(map[int64]*issues_model.Comment, len(commentIDs))
	if err := db.GetEngine(ctx).In("id", commentIDs).Find(&commentsMap); err != nil {
		return fmt.Errorf("find comment: %w", err)
	}

	for _, action := range actions {
		if action.CommentID > 0 {
			action.Comment = commentsMap[action.CommentID]
			if action.Comment != nil {
				action.Comment.Issue = action.Issue
			}
		}
	}
	return nil
}

func (actions ActionList) LoadIssues(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	conditions := builder.NewCond()
	issueNum := 0
	for _, action := range actions {
		if action.IsIssueEvent() {
			infos := action.GetIssueInfos()
			if len(infos) == 0 {
				continue
			}
			index, _ := strconv.ParseInt(infos[0], 10, 64)
			if index > 0 {
				conditions = conditions.Or(builder.Eq{
					"repo_id": action.RepoID,
					"`index`": index,
				})
				issueNum++
			}
		}
	}
	if !conditions.IsValid() {
		return nil
	}

	issuesMap := make(map[string]*issues_model.Issue, issueNum)
	issues := make([]*issues_model.Issue, 0, issueNum)
	if err := db.GetEngine(ctx).Where(conditions).Find(&issues); err != nil {
		return fmt.Errorf("find issue: %w", err)
	}
	for _, issue := range issues {
		issuesMap[fmt.Sprintf("%d-%d", issue.RepoID, issue.Index)] = issue
	}

	for _, action := range actions {
		if !action.IsIssueEvent() {
			continue
		}
		if index := action.getIssueIndex(); index > 0 {
			if issue, ok := issuesMap[fmt.Sprintf("%d-%d", action.RepoID, index)]; ok {
				action.Issue = issue
				action.Issue.Repo = action.Repo
			}
		}
	}
	return nil
}

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts GetFeedsOptions) (ActionList, int64, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, 0, errors.New("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	var err error
	var cond builder.Cond
	// if the actor is the requested user or is an administrator, we can skip the ActivityQueryCondition
	if opts.Actor != nil && opts.RequestedUser != nil && (opts.Actor.IsAdmin || opts.Actor.ID == opts.RequestedUser.ID) {
		cond = builder.Eq{
			"user_id": opts.RequestedUser.ID,
		}.And(
			FeedDateCond(opts),
		)

		if !opts.IncludeDeleted {
			cond = cond.And(builder.Eq{"is_deleted": false})
		}

		if !opts.IncludePrivate {
			cond = cond.And(builder.Eq{"is_private": false})
		}
		if opts.OnlyPerformedBy {
			cond = cond.And(builder.Eq{"act_user_id": opts.RequestedUser.ID})
		}
	} else {
		cond, err = ActivityQueryCondition(ctx, opts)
		if err != nil {
			return nil, 0, err
		}
	}

	actions := make([]*Action, 0, opts.PageSize)
	var count int64
	opts.SetDefaultValues()

	if opts.Page < 10 { // TODO: why it's 10 but other values? It's an experience value.
		sess := db.GetEngine(ctx).Where(cond)
		db.SetSessionPagination(sess, &opts)

		if opts.DontCount {
			err = sess.Desc("`action`.created_unix").Find(&actions)
		} else {
			count, err = sess.Desc("`action`.created_unix").FindAndCount(&actions)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("FindAndCount: %w", err)
		}
	} else {
		// First, only query which IDs are necessary, and only then query all actions to speed up the overall query
		sess := db.GetEngine(ctx).Where(cond).Select("`action`.id")
		db.SetSessionPagination(sess, &opts)

		actionIDs := make([]int64, 0, opts.PageSize)
		if err := sess.Table("action").Desc("`action`.created_unix").Find(&actionIDs); err != nil {
			return nil, 0, fmt.Errorf("Find(actionsIDs): %w", err)
		}

		if !opts.DontCount {
			count, err = db.GetEngine(ctx).Where(cond).
				Table("action").
				Cols("`action`.id").Count()
			if err != nil {
				return nil, 0, fmt.Errorf("Count: %w", err)
			}
		}

		if err := db.GetEngine(ctx).In("`action`.id", actionIDs).Desc("`action`.created_unix").Find(&actions); err != nil {
			return nil, 0, fmt.Errorf("Find: %w", err)
		}
	}

	if err := ActionList(actions).LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
	}

	return actions, count, nil
}

// CountHiddenUserActivities counts the requested user's own actions that the actor
// cannot see within the time span covered by the given feed page. Page spans
// partition the timeline so per-page counts sum up to the feed-wide total: the
// upper bound is the oldest visible action of the previous page (none on the first
// page), the lower bound is the oldest visible action on this page (none on the
// last page, so older hidden actions surface there). opts must be the options the
// visible page was fetched with, pageActions the actions it returned.
func CountHiddenUserActivities(ctx context.Context, opts GetFeedsOptions, pageActions ActionList) (int64, error) {
	if opts.RequestedUser == nil || opts.PageSize <= 0 {
		return 0, errors.New("need RequestedUser and a positive PageSize")
	}

	visibleCond, err := ActivityQueryCondition(ctx, opts)
	if err != nil {
		return 0, err
	}

	cond := builder.Eq{"user_id": opts.RequestedUser.ID, "act_user_id": opts.RequestedUser.ID}.
		And(FeedDateCond(opts)).
		And(builder.Not{visibleCond})

	if opts.Page > 1 {
		var boundary int64
		has, err := db.GetEngine(ctx).Table("action").Where(visibleCond).
			Cols("created_unix").Desc("created_unix").
			Limit(1, (opts.Page-1)*opts.PageSize-1).Get(&boundary)
		if err != nil {
			return 0, err
		}
		if !has { // the page is beyond the visible feed, its span is already covered
			return 0, nil
		}
		cond = cond.And(builder.Lt{"`action`.created_unix": boundary})
	}
	if len(pageActions) == opts.PageSize { // a short page is the last one
		cond = cond.And(builder.Gte{"`action`.created_unix": pageActions[len(pageActions)-1].CreatedUnix})
	}

	return db.GetEngine(ctx).Table("action").Where(cond).Count()
}
