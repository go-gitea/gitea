// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

func RenderUserHeader(ctx *context.Context) {
	ctx.Data["IsProjectEnabled"] = true
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["ContextUser"] = ctx.ContextUser
	tab := ctx.FormString("tab")
	ctx.Data["TabName"] = tab
	repo, err := repo_model.GetRepositoryByName(ctx.ContextUser.ID, ".profile")
	if err == nil && !repo.IsEmpty {
		gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer gitRepo.Close()
		commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			ctx.ServerError("GetBranchCommit", err)
			return
		}
		blob, err := commit.GetBlobByPath("README.md")
		if err == nil && blob != nil {
			ctx.Data["ProfileReadme"] = true
		}
	}
}
