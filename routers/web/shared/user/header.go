// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/repo/render"
	"code.gitea.io/gitea/services/user"
)

func RenderUserHeader(ctx *context.Context) {
	ctx.Data["IsProjectEnabled"] = true
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["ContextUser"] = ctx.ContextUser
	tab := ctx.FormString("tab")
	ctx.Data["TabName"] = tab

	repo, gitRepo, err := user.OpenUserProfileRepo(ctx, ctx.ContextUser)
	if err != nil {
		if !errors.Is(err, user.ErrProfileRepoNotExist) {
			log.Error("OpenUserProfileRepo: %v", err)
		}
		return
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
	if err != nil {
		log.Error("GetBranchCommit: %v", err)
		return
	}
	entry, err := commit.SubTree("/")
	if err != nil {
		log.Error("GetBranchCommit: %v", err)
		return
	}
	allEntrys, err := entry.ListEntries()
	if err != nil {
		log.Error("entry.Tree().ListEntries: %v", err)
		return
	}
	_, readmeFile, err := render.FindReadmeFileInEntries(ctx, allEntrys, true)
	if err != nil {
		log.Error("FindReadmeFileInEntries: %v", err)
		return
	}

	ctx.Data["ProfileReadme"] = readmeFile != nil
}
