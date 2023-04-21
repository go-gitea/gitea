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
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/audit"
)

// TeamAddRepository adds new repository to team of organization.
func TeamAddRepository(doer *user_model.User, t *organization.Team, repo *repo_model.Repository) error {
	if repo.OwnerID != t.OrgID {
		return errors.New("repository does not belong to organization")
	} else if models.HasRepository(t, repo.ID) {
		return nil
	}

	err := db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		return models.AddRepository(ctx, t, repo)
	})
	if err != nil {
		return err
	}

	audit.Record(audit.RepositoryCollaboratorTeamAdd, doer, repo, t, "Added team %s as collaborator for %s.", t.Name, repo.FullName())

	return nil
}
