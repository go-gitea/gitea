// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package project

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// List is a list of projects
type List []*Project

// LoadAttributes load repos and creators of projects.
func (pl List) LoadAttributes(ctx context.Context) (err error) {
	repos := make(map[int64]*repo_model.Repository)
	creators := make(map[int64]*user_model.User)
	var ok bool

	for i := range pl {
		// Organization projects don't have a Repo assgined and the repo_id for it is 0
		// So lets make sure we handle that case as well
		if pl[i].Repo == nil && pl[i].RepoID != 0 {
			pl[i].Repo, ok = repos[pl[i].RepoID]
			if !ok {
				repo, err := repo_model.GetRepositoryByID(ctx, pl[i].RepoID)
				if err != nil {
					return fmt.Errorf("getRepositoryByID [%d]: %v", pl[i].RepoID, err)
				}
				pl[i].Repo = repo
				repos[pl[i].RepoID] = repo
			}
		}

		if pl[i].Creator == nil {
			pl[i].Creator, ok = creators[pl[i].CreatorID]
			if !ok {
				creator, err := user_model.GetUserByID(ctx, pl[i].CreatorID)
				if err != nil {
					return fmt.Errorf("getUserByID [%d]: %v", pl[i].CreatorID, err)
				}
				pl[i].Creator = creator
				creators[pl[i].CreatorID] = creator
			}
		}
	}

	return nil
}
