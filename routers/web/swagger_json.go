// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

// tplSwaggerV1Json swagger v1 json template
const tplSwaggerV1Json base.TplName = "swagger/v1_json"

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	t, err := ctx.Render.TemplateLookup(string(tplSwaggerV1Json))
	if err != nil {
		ctx.ServerError("unable to find template", err)
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err = t.Execute(ctx.Resp, ctx.Data); err != nil {
		ctx.ServerError("unable to execute template", err)
	}
}
