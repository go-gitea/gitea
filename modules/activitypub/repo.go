// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/forgefed"
)

// Create a new federated repo from a Repository object
func FederatedRepoNew(ctx context.Context, repository forgefed.Repository) error {
	ownerIRI, err := repositoryIRIToOwnerIRI(repository.GetLink())
	if err != nil {
		return err
	}
	user, err := personIRIToUser(ctx, ownerIRI)
	if err != nil {
		return err
	}

	repo := repo_model.Repository{
		Name: repository.Name.String(),
	}
	if repository.ForkedFrom != nil {
		repo.IsFork = true
		forkedFrom, err := repositoryIRIToRepository(ctx, repository.ForkedFrom.GetLink())
		if err != nil {
			return err
		}
		repo.ForkID = forkedFrom.ID
	}

	// TODO: Check if repo already exists
	return models.CreateRepository(ctx, user, user, &repo, false)
}
