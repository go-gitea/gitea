// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	shared_audit "gitea.dev/routers/web/shared/audit"
	"gitea.dev/services/context"
)

func ViewAuditLogs(ctx *context.Context) {
	shared_audit.View(ctx, shared_audit.ViewOptions{
		Template: "admin/audit/list",
		PageData: map[string]any{"PageIsAdminMonitorAudit": true},
	})
}
