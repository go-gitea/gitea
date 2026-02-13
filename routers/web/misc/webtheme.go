// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"

	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"
	user_service "code.gitea.io/gitea/services/user"
	"code.gitea.io/gitea/services/webtheme"
)

func WebThemeList(ctx *context.Context) {
	curWebTheme := ctx.TemplateContext.CurrentWebTheme()
	renderUtils := templates.NewRenderUtils(ctx)
	allThemes := webtheme.GetAvailableThemes()

	var results []map[string]any
	for _, theme := range allThemes {
		results = append(results, map[string]any{
			"name":  renderUtils.RenderThemeItem(theme, 14),
			"value": theme.InternalName,
			"class": "item js-aria-clickable" + util.Iif(theme.InternalName == curWebTheme.InternalName, " selected", ""),
		})
	}
	ctx.JSON(http.StatusOK, map[string]any{"results": results})
}

func WebThemeApply(ctx *context.Context) {
	themeName := ctx.FormString("theme")
	if ctx.Doer != nil {
		opts := &user_service.UpdateOptions{Theme: optional.Some(themeName)}
		_ = user_service.UpdateUser(ctx, ctx.Doer, opts)
	} else {
		middleware.SetSiteCookie(ctx.Resp, "gitea_theme", themeName, 0)
	}
}
