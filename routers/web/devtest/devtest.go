// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"net/http"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/templates"
)

// List all devtest templates, they will be used for e2e tests for the UI components
func List(ctx *context.Context) {
	templateNames, err := templates.AssetFS().ListFiles("devtest", true)
	if err != nil {
		ctx.ServerError("AssetFS().ListFiles", err)
		return
	}
	var subNames []string
	for _, tmplName := range templateNames {
		subName := strings.TrimSuffix(tmplName, ".tmpl")
		if subName != "list" {
			subNames = append(subNames, subName)
		}
	}
	ctx.Data["SubNames"] = subNames
	ctx.HTML(http.StatusOK, "devtest/list")
}

func Tmpl(ctx *context.Context) {
	now := time.Now()
	ctx.Data["TimeNow"] = now
	ctx.Data["TimePast5s"] = now.Add(-5 * time.Second)
	ctx.Data["TimeFuture5s"] = now.Add(5 * time.Second)
	ctx.Data["TimePast2m"] = now.Add(-2 * time.Minute)
	ctx.Data["TimeFuture2m"] = now.Add(2 * time.Minute)
	ctx.Data["TimePast1y"] = now.Add(-1 * 366 * 86400 * time.Second)
	ctx.Data["TimeFuture1y"] = now.Add(1 * 366 * 86400 * time.Second)

	ctx.HTML(http.StatusOK, base.TplName("devtest"+path.Clean("/"+ctx.Params("sub"))))
}
