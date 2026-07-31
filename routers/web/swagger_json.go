// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

func swaggerJsonServe(ctx *context.Context, file string) {
	buf, err := templates.AssetFS().ReadFile(file)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, "unable to read api json file: "+file)
		return
	}
	r := strings.NewReplacer(
		"0.0.0+GITEA-API-APP-VERSION", setting.AppVer,
		"/GITEA-API-APP-SUBURL/", setting.AppSubURL+"/",
	)
	ctx.Resp.Header().Set("Content-Type", "application/json")
	_, _ = r.WriteString(ctx.Resp, util.UnsafeBytesToString(buf))
}

func SwaggerV1Json(ctx *context.Context) {
	swaggerJsonServe(ctx, "swagger/v1-swagger.generated.json")
}

func OpenAPI3Json(ctx *context.Context) {
	swaggerJsonServe(ctx, "swagger/v1-openapi3.generated.json")
}
