// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"maps"
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

// ViewOptions configures a scoped audit log listing. The admin view leaves
// ScopeType empty to list every event; the user/org/repo views constrain the
// query to their own scope.
type ViewOptions struct {
	Template  templates.TplName
	ScopeType audit_model.ScopeType
	ScopeID   int64
	// PageData holds ctx.Data flags to enable for the active navigation tab.
	PageData map[string]any
}

// View renders a paginated audit log listing shared by the admin, user, org and
// repo settings pages. Only the scope filter, template and page flags differ.
func View(ctx *context.Context, opts ViewOptions) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.audit.title")
	maps.Copy(ctx.Data, opts.PageData)

	page := max(ctx.FormInt("page"), 1)

	searchOpts := &audit_model.EventSearchOptions{
		Sort:      ctx.FormString("sort"),
		ScopeType: opts.ScopeType,
		ScopeID:   opts.ScopeID,
		Paginator: &db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.NoticePagingNum,
		},
	}

	ctx.Data["AuditSort"] = searchOpts.Sort

	evs, total, err := audit.FindEvents(ctx, searchOpts)
	if err != nil {
		ctx.ServerError("FindEvents", err)
		return
	}

	ctx.Data["AuditEvents"] = evs

	pager := context.NewPagination(total, setting.UI.Admin.NoticePagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, opts.Template)
}
