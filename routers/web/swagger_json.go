// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"html/template"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	ctx.Data["SwaggerAppVer"] = template.HTML(template.JSEscapeString(setting.AppVer))
	ctx.Data["SwaggerAppSubUrl"] = setting.AppSubURL // it is JS-safe
	ctx.JSONTemplate("swagger/v1_json")
}
