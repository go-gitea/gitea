// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// UserActivityList defines a list of UserActivity
type UserActivityList []*UserActivity

func (actions UserActivityList) getUserIDs() []int64 {
	return container.FilterSlice(actions, func(action *UserActivity) (int64, bool) {
		return action.ActUserID, true
	})
}

func (actions UserActivityList) LoadActUsers(ctx context.Context) (map[int64]*user_model.User, error) {
	if len(actions) == 0 {
		return nil, nil
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

func (actions UserActivityList) getRepoIDs() []int64 {
	return container.FilterSlice(actions, func(action *UserActivity) (int64, bool) {
		return action.RepoID, true
	})
}

func (actions UserActivityList) LoadRepositories(ctx context.Context) error {
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

func (actions UserActivityList) loadRepoOwner(ctx context.Context, userMap map[int64]*user_model.User) (err error) {
	if userMap == nil {
		userMap = make(map[int64]*user_model.User)
	}

	missingUserIDs := container.FilterSlice(actions, func(action *UserActivity) (int64, bool) {
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
func (actions UserActivityList) LoadAttributes(ctx context.Context) error {
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

func (actions UserActivityList) LoadComments(ctx context.Context) error {
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

func (actions UserActivityList) LoadIssues(ctx context.Context) error {
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

// GetFeedsOptions options for retrieving feeds
type GetFeedsOptions struct {
	db.ListOptions
	RequestedUser   *user_model.User       // the user we want activity for
	RequestedTeam   *organization.Team     // the team we want activity for
	RequestedRepo   *repo_model.Repository // the repo we want activity for
	Actor           *user_model.User       // the user viewing the activity
	IncludePrivate  bool                   // include private actions
	OnlyPerformedBy bool                   // only actions performed by requested user
	IncludeDeleted  bool                   // include deleted actions
	Date            string                 // the day we want activity for: YYYY-MM-DD
}

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts GetFeedsOptions) (UserActivityList, int64, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, 0, fmt.Errorf("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	cond, err := activityQueryCondition(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	actions := make([]*UserActivity, 0, opts.PageSize)
	var count int64

	if opts.Page < 10 { // TODO: why it's 10 but other values? It's an experience value.
		sess := db.GetEngine(ctx).Where(cond).
			Select("`user_activity`.*"). // this line will avoid select other joined table's columns
			Join("INNER", "repository", "`repository`.id = `user_activity`.repo_id")

		opts.SetDefaultValues()
		sess = db.SetSessionPagination(sess, &opts)

		count, err = sess.Desc("`user_activity`.created_unix").FindAndCount(&actions)
		if err != nil {
			return nil, 0, fmt.Errorf("FindAndCount: %w", err)
		}
	} else {
		// First, only query which IDs are necessary, and only then query all actions to speed up the overall query
		sess := db.GetEngine(ctx).Where(cond).
			Select("`user_activity`.id").
			Join("INNER", "repository", "`repository`.id = `user_activity`.repo_id")

		opts.SetDefaultValues()
		sess = db.SetSessionPagination(sess, &opts)

		actionIDs := make([]int64, 0, opts.PageSize)
		if err := sess.Table("action").Desc("`user_activity`.created_unix").Find(&actionIDs); err != nil {
			return nil, 0, fmt.Errorf("Find(actionsIDs): %w", err)
		}

		count, err = db.GetEngine(ctx).Where(cond).
			Table("action").
			Cols("`user_activity`.id").
			Join("INNER", "repository", "`repository`.id = `user_activity`.repo_id").Count()
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %w", err)
		}

		if err := db.GetEngine(ctx).In("`user_activity`.id", actionIDs).Desc("`user_activity`.created_unix").Find(&actions); err != nil {
			return nil, 0, fmt.Errorf("Find: %w", err)
		}
	}

	if err := UserActivityList(actions).LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
	}

	return actions, count, nil
}
