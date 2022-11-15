// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
func TeamAddRepository(t *organization.Team, repo *repo_model.Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("repository does not belong to organization")
	} else if models.HasRepository(t, repo.ID) {
		return nil
	}

	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		return models.AddRepository(ctx, t, repo)
	})
}
