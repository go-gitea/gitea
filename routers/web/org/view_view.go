package org

import (
	"net/http"

	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplOrgViewHome templates.TplName = "org/view/home"
)

// View render repository view page
func View(ctx *context.Context) {
	ctx.HTML(http.StatusOK, tplOrgViewHome)
}
