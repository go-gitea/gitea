// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

// PrepareForRepoSubMenu will prepare all sub menus' data for some repositories pages
// TODO: moving preparation for commits count, branches count, tags count and repository size to this function
func PrepareForRepoSubMenu(ctx *context.Context) bool {
	return prepareForLicenses(ctx)
}

func prepareForLicenses(ctx *context.Context) bool {
	repoLicenses, err := repo_model.GetRepoLicenses(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoLicenses", err)
		return false
	}
	ctx.Data["DetectedRepoLicenses"] = repoLicenses.StringList()
	ctx.Data["LicenseFileName"] = repo_service.LicenseFileName
	return true
}
