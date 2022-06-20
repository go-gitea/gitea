// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

type RunnerList []*Runner

// GetUserIDs returns a slice of user's id
func (runners RunnerList) GetUserIDs() []int64 {
	userIDsMap := make(map[int64]struct{})
	for _, runner := range runners {
		if runner.OwnerID == 0 {
			continue
		}
		userIDsMap[runner.OwnerID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(userIDsMap))
	for userID := range userIDsMap {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (runners RunnerList) LoadOwners(ctx context.Context) error {
	userIDs := runners.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, runner := range runners {
		if runner.OwnerID > 0 && runner.Owner == nil {
			runner.Owner = users[runner.OwnerID]
		}
	}
	return nil
}

func (runners RunnerList) getRepoIDs() []int64 {
	repoIDs := make(map[int64]struct{}, len(runners))
	for _, runner := range runners {
		if runner.RepoID == 0 {
			continue
		}
		if _, ok := repoIDs[runner.RepoID]; !ok {
			repoIDs[runner.RepoID] = struct{}{}
		}
	}
	return container.KeysInt64(repoIDs)
}

func (runners RunnerList) LoadRepos(ctx context.Context) error {
	repoIDs := runners.getRepoIDs()
	repos := make(map[int64]*repo_model.Repository, len(repoIDs))
	if err := db.GetEngine(ctx).In("id", repoIDs).Find(&repos); err != nil {
		return err
	}

	for _, runner := range runners {
		if runner.RepoID > 0 && runner.Repo == nil {
			runner.Repo = repos[runner.RepoID]
		}
	}
	return nil
}

func (runners RunnerList) LoadAttributes(ctx context.Context) error {
	if err := runners.LoadOwners(ctx); err != nil {
		return err
	}
	if err := runners.LoadRepos(ctx); err != nil {
		return err
	}
	return nil
}
