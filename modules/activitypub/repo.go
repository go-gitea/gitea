// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/forgefed"
	repo_module "code.gitea.io/gitea/modules/repository"
	repo_service "code.gitea.io/gitea/services/repository"
)

// Create a new federated repo from a Repository object
func FederatedRepoNew(ctx context.Context, repository *forgefed.Repository) error {
	user, err := PersonIRIToUser(ctx, repository.AttributedTo.GetLink())
	if err != nil {
		return err
	}

	_, err = repo_model.GetRepositoryByOwnerAndNameCtx(ctx, user.Name, repository.Name.String())
	if err == nil {
		return nil
	}

	repo, err := repo_service.CreateRepository(user, user, repo_module.CreateRepoOptions{
		Name: repository.Name.String(),
	})
	if err != nil {
		return err
	}

	if repository.ForkedFrom != nil {
		repo.IsFork = true
		forkedFrom, err := RepositoryIRIToRepository(ctx, repository.ForkedFrom.GetLink())
		if err != nil {
			return err
		}
		repo.ForkID = forkedFrom.ID
	}
	return nil
}
