// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	tplFindFiles templates.TplName = "repo/find/files"
)

// FindFiles render the page to find repository files
func FindFiles(ctx *context.Context) {
	path := ctx.PathParam("*")
	ctx.Data["TreeLink"] = ctx.Repo.RepoLink + "/src/" + util.PathEscapeSegments(path)
	ctx.Data["DataLink"] = ctx.Repo.RepoLink + "/tree-list/" + util.PathEscapeSegments(path)
	ctx.HTML(http.StatusOK, tplFindFiles)
}
