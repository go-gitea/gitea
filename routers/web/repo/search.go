// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/routers/common"
	"gitea.dev/services/codesearch"
	"gitea.dev/services/context"
)

const tplSearch templates.TplName = "repo/search"

func Search(ctx *context.Context) {
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["Repo"] = ctx.Repo.Repository
	engine := codesearch.GitGrep
	if setting.Indexer.RepoIndexerEnabled {
		engine = codesearch.Indexer
	}
	common.SearchCode(ctx, &codesearch.Options{
		Engine: engine, RepoID: ctx.Repo.Repository.ID, GitRepo: ctx.Repo.GitRepo, Ref: git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch),
	})
	if !ctx.Written() {
		ctx.HTML(http.StatusOK, tplSearch)
	}
}
