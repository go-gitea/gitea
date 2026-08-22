// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"maps"
	"net/http"
	"slices"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
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

var filterableOrigins = []audit_model.Origin{audit_model.OriginUI, audit_model.OriginAPI, audit_model.OriginCLI, audit_model.OriginSystem}

// SearchOptionsFromRequest builds the event query from the request filters and
// publishes the applied values to ctx.Data so the filter form can render its
// current state. The listing and the export share it to stay in sync.
func SearchOptionsFromRequest(ctx *context.Context, scopeType audit_model.ScopeType, scopeID int64) *audit_model.EventSearchOptions {
	// only the two known sort values are accepted, anything else falls back to the default
	sort := util.Iif(audit_model.EventSort(ctx.FormString("sort")) == audit_model.SortTimestampAsc, audit_model.SortTimestampAsc, audit_model.SortTimestampDesc)

	opts := &audit_model.EventSearchOptions{
		Sort:      sort,
		ScopeType: scopeType,
		ScopeID:   scopeID,
	}

	if action := audit_model.Action(ctx.FormString("action")); action != "" {
		if _, ok := audit_model.MessageTemplate(action); ok {
			opts.Action = action
		}
	}
	if origin := audit_model.Origin(ctx.FormString("origin")); slices.Contains(filterableOrigins, origin) {
		opts.Origin = origin
	}
	if actor := ctx.FormTrim("actor"); actor != "" {
		u, err := user_model.GetUserByName(ctx, actor)
		if err != nil {
			opts.ActorID = -1 // an unknown actor matches nothing rather than everything
		} else {
			opts.ActorID = u.ID
		}
	}

	ctx.Data["AuditSort"] = string(opts.Sort)
	ctx.Data["AuditFilterAction"] = string(opts.Action)
	ctx.Data["AuditFilterOrigin"] = string(opts.Origin)
	ctx.Data["AuditFilterActor"] = ctx.FormTrim("actor")
	ctx.Data["AuditActions"] = audit_model.AllActions()
	ctx.Data["AuditOrigins"] = filterableOrigins

	return opts
}

// View renders a paginated audit log listing shared by the admin, user, org and
// repo settings pages. Only the scope filter, template and page flags differ.
func View(ctx *context.Context, opts ViewOptions) {
	ctx.Data["Title"] = ctx.Tr("audit.title")
	maps.Copy(ctx.Data, opts.PageData)

	page := max(ctx.FormInt("page"), 1)

	searchOpts := SearchOptionsFromRequest(ctx, opts.ScopeType, opts.ScopeID)
	searchOpts.ListOptions = db.ListOptions{
		Page:     page,
		PageSize: setting.UI.Admin.NoticePagingNum,
	}

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
