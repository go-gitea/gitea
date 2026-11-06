// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

// Some interesting names could be found in https://github.com/open-telemetry/opentelemetry-go/tree/main/semconv

const (
	TraceSpanContext  = "context"
	TraceSpanHTTP     = "http"
	TraceSpanGitRun   = "git-run"
	TraceSpanDatabase = "database"
)

const (
	TraceAttrGeneralName = "general.name"
	TraceAttrGeneralDesc = "general.desc"
	TraceAttrFuncCaller  = "func.caller"
	TraceAttrDbSQL       = "db.sql"
	TraceAttrGitCommand  = "git.command"
	TraceAttrHTTPRoute   = "http.route"
)
