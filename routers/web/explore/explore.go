// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared"
	"code.gitea.io/gitea/services/context"
)

// /explore/* routes
func ProvideExploreRoutes(m *web.Router) func() {
	return func() {
		m.Get("", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", Repos)
		m.Get("/repos/sitemap-{idx}.xml", shared.SitemapEnabled, Repos)
		m.Get("/users", Users)
		m.Get("/users/sitemap-{idx}.xml", shared.SitemapEnabled, Users)
		m.Get("/organizations", Organizations)
		m.Get("/code", func(ctx *context.Context) {
			if unit.TypeCode.UnitGlobalDisabled() {
				ctx.NotFound(nil)
				return
			}
		}, Code)
		m.Get("/topics/search", TopicSearch)
	}
}
