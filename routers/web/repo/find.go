// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplFindFiles base.TplName = "repo/find/files"
)

// FindFiles render the page to find repository files
func FindFiles(ctx *context.Context) {
	path := ctx.Params("*")
	ctx.Data["TreeLink"] = ctx.Repo.RepoLink + "/src/" + util.PathEscapeSegments(path)
	ctx.Data["DataLink"] = ctx.Repo.RepoLink + "/tree-list/" + util.PathEscapeSegments(path)
	ctx.HTML(http.StatusOK, tplFindFiles)
}
