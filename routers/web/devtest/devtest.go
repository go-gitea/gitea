// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/templates"
)

// List all devtest templates, they will be used for e2e tests for the UI components
func List(ctx *context.Context) {
	templateNames := templates.GetTemplateAssetNames()
	var subNames []string
	const prefix = "templates/devtest/"
	for _, tmplName := range templateNames {
		if strings.HasPrefix(tmplName, prefix) {
			subName := strings.TrimSuffix(strings.TrimPrefix(tmplName, prefix), ".tmpl")
			if subName != "list" {
				subNames = append(subNames, subName)
			}
		}
	}
	ctx.Data["SubNames"] = subNames
	ctx.HTML(http.StatusOK, "devtest/list")
}

func Tmpl(ctx *context.Context) {
	ctx.HTML(http.StatusOK, base.TplName("devtest"+path.Clean("/"+ctx.Params("sub"))))
}
