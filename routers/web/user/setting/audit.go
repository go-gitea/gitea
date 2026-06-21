// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

const (
	tplAuditLogs templates.TplName = "user/settings/audit_logs"
)

func ViewAuditLogs(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.audit.title")
	ctx.Data["PageIsSettingsAudit"] = true

	page := max(ctx.FormInt("page"), 1)

	opts := &audit_model.EventSearchOptions{
		Sort:      ctx.FormString("sort"),
		ScopeType: audit_model.ScopeUser,
		ScopeID:   ctx.Doer.ID,
		Paginator: &db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.NoticePagingNum,
		},
	}

	ctx.Data["AuditSort"] = opts.Sort

	evs, total, err := audit.FindEvents(ctx, opts)
	if err != nil {
		ctx.ServerError("", err)
		return
	}

	ctx.Data["AuditEvents"] = evs

	pager := context.NewPagination(total, setting.UI.Admin.NoticePagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplAuditLogs)
}
