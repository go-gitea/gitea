// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/modules/gtprof"
	"code.gitea.io/gitea/modules/reqctx"
)

type contextKeyType struct{}

var contextKey contextKeyType

// RecordFuncInfo records a func info into context
func RecordFuncInfo(ctx context.Context, funcInfo *FuncInfo) (end func()) {
	end = func() {}
	if reqCtx := reqctx.FromContext(ctx); reqCtx != nil {
		var traceSpan *gtprof.TraceSpan
		traceSpan, end = gtprof.GetTracer().StartInContext(reqCtx, "http.func")
		traceSpan.SetAttributeString("func", funcInfo.shortName)
	}
	if record, ok := ctx.Value(contextKey).(*requestRecord); ok {
		record.lock.Lock()
		record.funcInfo = funcInfo
		record.lock.Unlock()
	}
	return end
}

// MarkLongPolling marks the request is a long-polling request, and the logger may output different message for it
func MarkLongPolling(resp http.ResponseWriter, req *http.Request) {
	record, ok := req.Context().Value(contextKey).(*requestRecord)
	if !ok {
		return
	}

	record.lock.Lock()
	record.isLongPolling = true
	record.lock.Unlock()
}

// UpdatePanicError updates a context's error info, a panic may be recovered by other middlewares, but we still need to know that.
func UpdatePanicError(ctx context.Context, err any) {
	record, ok := ctx.Value(contextKey).(*requestRecord)
	if !ok {
		return
	}

	record.lock.Lock()
	record.panicError = err
	record.lock.Unlock()
}
