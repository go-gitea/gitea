// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activities

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
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

func (actions ActionList) loadUsers(ctx context.Context) (map[int64]*user_model.User, error) {
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

func (actions ActionList) loadRepositories(ctx context.Context) error {
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

	for _, action := range actions {
		if action.Repo == nil {
			continue
		}
		repoOwner, ok := userMap[action.Repo.OwnerID]
		if !ok {
			repoOwner, err = user_model.GetUserByIDCtx(ctx, action.Repo.OwnerID)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					continue
				}
				return err
			}
			userMap[repoOwner.ID] = repoOwner
		}
		action.Repo.Owner = repoOwner
	}

	return nil
}

// loadAttributes loads all attributes
func (actions ActionList) loadAttributes(ctx context.Context) error {
	userMap, err := actions.loadUsers(ctx)
	if err != nil {
		return err
	}

	if err := actions.loadRepositories(ctx); err != nil {
		return err
	}

	return actions.loadRepoOwner(ctx, userMap)
}
