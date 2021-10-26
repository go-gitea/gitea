// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplActions base.TplName = "repo/actions"
)

// Github Actions for gitea
func Actions(ctx *context.Context) {
	/*ctx.Data["Title"] = ctx.Tr("repo.activity")
	ctx.Data["PageIsActivity"] = true

	ctx.Data["Period"] = ctx.Params("period")*/

	ctx.HTML(http.StatusOK, tplActions)
}
