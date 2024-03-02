// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	audit_model "code.gitea.io/gitea/models/audit"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/audit"
	"code.gitea.io/gitea/services/context"
)

const (
	tplAuditLogs base.TplName = "org/settings/audit_logs"
)

func ViewAuditLogs(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.audit.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsAudit"] = true

	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}

	opts := &audit_model.EventSearchOptions{
		Sort:      ctx.FormString("sort"),
		ScopeType: audit_model.TypeOrganization,
		ScopeID:   ctx.ContextUser.ID,
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

	pager := context.NewPagination(int(total), setting.UI.Admin.NoticePagingNum, page, 5)
	pager.AddParamString("sort", opts.Sort)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplAuditLogs)
}
