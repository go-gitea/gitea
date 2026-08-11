// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"time"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	shared_audit "gitea.dev/routers/web/shared/audit"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

const auditExportPageSize = 1000

func ViewAuditLogs(ctx *context.Context) {
	shared_audit.View(ctx, shared_audit.ViewOptions{
		Template: "admin/audit/list",
		PageData: map[string]any{"PageIsAdminMonitorAudit": true},
	})
}

func ExportAuditLogs(ctx *context.Context) {
	page := 1
	findPage := func() ([]*audit.Event, error) {
		events, _, err := audit.FindEvents(ctx, &audit_model.EventSearchOptions{
			Sort: audit_model.SortTimestampAsc,
			Paginator: &db.ListOptions{
				Page:     page,
				PageSize: auditExportPageSize,
			},
		})
		return events, err
	}

	events, err := findPage()
	if err != nil {
		ctx.ServerError("FindEvents", err)
		return
	}

	httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{
		ContentType:        "application/x-ndjson; charset=utf-8",
		Filename:           fmt.Sprintf("gitea-audit-log-%s.jsonl", time.Now().UTC().Format("20060102T150405Z")),
		ContentDisposition: httplib.ContentDispositionAttachment,
	})
	ctx.Resp.WriteHeader(http.StatusOK)

	for {
		if err := audit.WriteEventsAsJSON(ctx.Resp, events); err != nil {
			log.Debug("Unable to write audit log export: %v", err)
			return
		}
		if len(events) < auditExportPageSize {
			return
		}

		page++
		events, err = findPage()
		if err != nil {
			log.Error("Unable to continue audit log export: %v", err)
			return
		}
	}
}
