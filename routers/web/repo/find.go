// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplFindFiles base.TplName = "repo/find/files"
)

// FindFiles render the page to find repository files
func FindFiles(ctx *context.Context) {
	path := ctx.Params("*")
	ctx.Data["TreeLink"] = ctx.Repo.RepoLink + "/src/" + path
	ctx.Data["DataLink"] = ctx.Repo.RepoLink + "/tree-list/" + path
	ctx.HTML(http.StatusOK, tplFindFiles)
}
