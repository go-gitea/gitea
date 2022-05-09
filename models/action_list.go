// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

// ActionList defines a list of actions
type ActionList []*Action

func (actions ActionList) getUserIDs() []int64 {
	userIDs := make(map[int64]struct{}, len(actions))
	for _, action := range actions {
		if _, ok := userIDs[action.ActUserID]; !ok {
			userIDs[action.ActUserID] = struct{}{}
		}
	}
	return container.KeysInt64(userIDs)
}

func (actions ActionList) loadUsers(e db.Engine) (map[int64]*user_model.User, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	userIDs := actions.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := e.
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %v", err)
	}

	for _, action := range actions {
		action.ActUser = userMaps[action.ActUserID]
	}
	return userMaps, nil
}

func (actions ActionList) getRepoIDs() []int64 {
	repoIDs := make(map[int64]struct{}, len(actions))
	for _, action := range actions {
		if _, ok := repoIDs[action.RepoID]; !ok {
			repoIDs[action.RepoID] = struct{}{}
		}
	}
	return container.KeysInt64(repoIDs)
}

func (actions ActionList) loadRepositories(e db.Engine) error {
	if len(actions) == 0 {
		return nil
	}

	repoIDs := actions.getRepoIDs()
	repoMaps := make(map[int64]*repo_model.Repository, len(repoIDs))
	err := e.In("id", repoIDs).Find(&repoMaps)
	if err != nil {
		return fmt.Errorf("find repository: %v", err)
	}

	for _, action := range actions {
		action.Repo = repoMaps[action.RepoID]
	}
	return nil
}

func (actions ActionList) loadRepoOwner(e db.Engine, userMap map[int64]*user_model.User) (err error) {
	if userMap == nil {
		userMap = make(map[int64]*user_model.User)
	}

	for _, action := range actions {
		if action.Repo == nil {
			continue
		}
		repoOwner, ok := userMap[action.Repo.OwnerID]
		if !ok {
			repoOwner, err = user_model.GetUserByID(action.Repo.OwnerID)
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
func (actions ActionList) loadAttributes(e db.Engine) error {
	userMap, err := actions.loadUsers(e)
	if err != nil {
		return err
	}

	if err := actions.loadRepositories(e); err != nil {
		return err
	}

	return actions.loadRepoOwner(e, userMap)
}
