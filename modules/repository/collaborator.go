// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

func addCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User) error {
	collaboration := &repo_model.Collaboration{
		RepoID: repo.ID,
		UserID: u.ID,
	}

	has, err := db.GetByBean(ctx, collaboration)
	if err != nil {
		return err
	} else if has {
		return nil
	}
	collaboration.Mode = perm.AccessModeWrite

	if err = db.Insert(ctx, collaboration); err != nil {
		return err
	}

	return access_model.RecalculateUserAccess(ctx, repo, u.ID)
}

// AddCollaborator adds new collaboration to a repository with default access mode.
func AddCollaborator(repo *repo_model.Repository, u *user_model.User) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		return addCollaborator(ctx, repo, u)
	})
}
