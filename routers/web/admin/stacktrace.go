// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"runtime"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func monitorTraceCommon(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor")
	ctx.Data["PageIsAdminMonitorTrace"] = true
	// Hide the performance trace tab in production, because it shows a lot of SQLs and is not that useful for end users.
	// To avoid confusing end users, do not let them know this tab. End users should "download diagnosis report" instead.
	ctx.Data["ShowAdminPerformanceTraceTab"] = !setting.IsProd
}

// Stacktrace show admin monitor goroutines page
func Stacktrace(ctx *context.Context) {
	monitorTraceCommon(ctx)

	ctx.Data["GoroutineCount"] = runtime.NumGoroutine()

	show := ctx.FormString("show")
	ctx.Data["ShowGoroutineList"] = show
	// by default, do not do anything which might cause server errors, to avoid unnecessary 500 pages.
	// this page is the entrance of the chance to collect diagnosis report.
	if show != "" {
		showNoSystem := show == "process"
		processStacks, processCount, _, err := process.GetManager().ProcessStacktraces(false, showNoSystem)
		if err != nil {
			ctx.ServerError("GoroutineStacktrace", err)
			return
		}

		ctx.Data["ProcessStacks"] = processStacks
		ctx.Data["ProcessCount"] = processCount
	}

	ctx.HTML(http.StatusOK, tplStacktrace)
}

// StacktraceCancel cancels a process
func StacktraceCancel(ctx *context.Context) {
	pid := ctx.PathParam("pid")
	process.GetManager().Cancel(process.IDType(pid))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/monitor/stacktrace")
}
