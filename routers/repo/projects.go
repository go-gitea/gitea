// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplProjects base.TplName = "repo/projects/list"

	projectTemplateKey = "ProjectTemplate"
)

func MustEnableProjects(ctx *context.Context) {

	if !setting.Admin.EnableKanbanBoard {
		ctx.NotFound("EnableKanbanBoard", nil)
		return
	}

	if !ctx.Repo.CanRead(models.UnitTypeProjects) {
		ctx.NotFound("MustEnableProjects", nil)
		return
	}
}

// Projects renders the home page
func Projects(ctx *context.Context) {

	ctx.HTML(200, tplProjects)
}
