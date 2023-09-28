// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
)

// TeamAddRepository adds new repository to team of organization.
func TeamAddRepository(ctx context.Context, t *organization.Team, repo *repo_model.Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("repository does not belong to organization")
	} else if organization.HasTeamRepo(ctx, t.OrgID, t.ID, repo.ID) {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		return models.AddRepository(ctx, t, repo)
	})
}
