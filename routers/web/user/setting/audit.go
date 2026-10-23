// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	audit_model "codenest.dev/models/audit"
	shared_audit "codenest.dev/routers/web/shared/audit"
	"codenest.dev/services/context"
)

func ViewAuditLogs(ctx *context.Context) {
	shared_audit.View(ctx, shared_audit.ViewOptions{
		Template:  "user/settings/audit_logs",
		ScopeType: audit_model.ScopeUser,
		ScopeID:   ctx.Doer.ID,
		PageData:  map[string]any{"PageIsSettingsAudit": true},
	})
}
