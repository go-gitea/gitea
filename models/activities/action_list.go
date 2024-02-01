// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

// ActionList defines a list of actions
type ActionList []*Action

func (actions ActionList) getUserIDs() []int64 {
	userIDs := make(container.Set[int64], len(actions))
	for _, action := range actions {
		userIDs.Add(action.ActUserID)
	}
	return userIDs.Values()
}

func (actions ActionList) LoadActUsers(ctx context.Context) (map[int64]*user_model.User, error) {
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

func (actions ActionList) getRepoIDs() []int64 {
	repoIDs := make(container.Set[int64], len(actions))
	for _, action := range actions {
		repoIDs.Add(action.RepoID)
	}
	return repoIDs.Values()
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
	return nil
}

func (actions ActionList) loadRepoOwner(ctx context.Context, userMap map[int64]*user_model.User) (err error) {
	if userMap == nil {
		userMap = make(map[int64]*user_model.User)
	}

	userIDs := make([]int64, 0, len(actions))
	userSet := make(container.Set[int64], len(actions))
	for _, action := range actions {
		if action.Repo == nil {
			continue
		}
		if _, ok := userMap[action.Repo.OwnerID]; !ok && !userSet.Contains(action.Repo.OwnerID) {
			userIDs = append(userIDs, action.Repo.OwnerID)
			userSet.Add(action.Repo.OwnerID)
		}
	}

	if err := db.GetEngine(ctx).
		In("id", userIDs).
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

// loadAttributes loads all attributes
func (actions ActionList) loadAttributes(ctx context.Context) error {
	userMap, err := actions.LoadActUsers(ctx)
	if err != nil {
		return err
	}

	if err := actions.LoadRepositories(ctx); err != nil {
		return err
	}

	return actions.loadRepoOwner(ctx, userMap)
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

	commentsMap := make(map[int64]*issues_model.Comment, len(commentIDs))
	if err := db.GetEngine(ctx).In("id", commentIDs).Find(&commentsMap); err != nil {
		return fmt.Errorf("find comment: %w", err)
	}

	for _, action := range actions {
		if action.CommentID > 0 {
			action.Comment = commentsMap[action.CommentID]
		}
	}
	return nil
}
