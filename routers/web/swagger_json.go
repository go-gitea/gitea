// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"code.gitea.io/gitea/modules/context"
)

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	ctx.JSONTemplate("swagger/v1_json")
}
