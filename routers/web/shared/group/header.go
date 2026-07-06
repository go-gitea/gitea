// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

func LoadHeaderCount(ctx *context.Context) error {
	if ctx.RepoGroup.Group.Owner.IsIndividual() {
		if _, err := user.RenderUserOrgHeader(ctx); err != nil {
			return err
		}
	}
	repoCount, err := repo_model.CountRepository(ctx, repo_model.SearchRepoOptions{
		Actor:              ctx.Doer,
		Private:            ctx.IsSigned,
		GroupID:            ctx.RepoGroup.Group.ID,
		Collaborate:        optional.Some(false),
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		return err
	}
	ctx.Data["RepoCount"] = repoCount

	return nil
}
