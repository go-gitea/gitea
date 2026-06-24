// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	audit_model "gitea.dev/models/audit"
	shared_audit "gitea.dev/routers/web/shared/audit"
	"gitea.dev/services/context"
)

func ViewAuditLogs(ctx *context.Context) {
	shared_audit.View(ctx, shared_audit.ViewOptions{
		Template:  "org/settings/audit_logs",
		ScopeType: audit_model.ScopeOrganization,
		ScopeID:   ctx.ContextUser.ID,
		PageData: map[string]any{
			"PageIsOrgSettings":   true,
			"PageIsSettingsAudit": true,
		},
	})
}
