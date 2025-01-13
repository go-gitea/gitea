// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/git"
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
	ref := git.RefNameFromUserInput(ctx.FormTrim("ref"), git.RefTypeBranch, git.RefTypeTag, git.RefTypeCommit)
	ctx.Data["TreeLink"] = ctx.Repo.RepoLink + "/src/" + ref.RefWebLinkPath() + "/" + util.PathEscapeSegments(path)
	ctx.Data["DataLink"] = ctx.Repo.RepoLink + "/tree-list/" + util.PathEscapeSegments(path) + "?ref=" + url.QueryEscape(ctx.FormTrim("ref"))
	ctx.HTML(http.StatusOK, tplFindFiles)
}
