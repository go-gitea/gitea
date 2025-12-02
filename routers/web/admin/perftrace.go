// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"code.gitea.io/gitea/modules/tailmsg"
	"code.gitea.io/gitea/services/context"
)

func PerfTrace(ctx *context.Context) {
	monitorTraceCommon(ctx)
	ctx.Data["PageIsAdminMonitorPerfTrace"] = true
	ctx.Data["PerfTraceRecords"] = tailmsg.GetManager().GetTraceRecorder().GetRecords()
	ctx.HTML(http.StatusOK, tplPerfTrace)
}
