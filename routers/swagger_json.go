// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

// tplSwaggerV1Json swagger v1 json template
const tplSwaggerV1Json base.TplName = "swagger/v1_json"

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	ctx.HTML(200, tplSwaggerV1Json)
}
