package dev

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
)

func BuildView(ctx *context.Context) {
	ctx.HTML(http.StatusOK, "dev/buildview")
}
