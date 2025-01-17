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

// StartContextSpan starts a trace span in Gitea's web request context
// Due to the design limitation of Gitea's web framework, it can't use `context.WithValue` to bind a new span into a new context.
// So here we use our "reqctx" framework to achieve the same result: web request context could always see the latest "span".
func StartContextSpan(ctx reqctx.RequestContext, spanName string) (*gtprof.TraceSpan, func()) {
	curTraceSpan := gtprof.GetContextSpan(ctx)
	_, newTraceSpan := gtprof.GetTracer().Start(ctx, spanName)
	ctx.SetContextValue(gtprof.ContextKeySpan, newTraceSpan)
	return newTraceSpan, func() {
		newTraceSpan.End()
		ctx.SetContextValue(gtprof.ContextKeySpan, curTraceSpan)
	}
}

// RecordFuncInfo records a func info into context
func RecordFuncInfo(ctx context.Context, funcInfo *FuncInfo) (end func()) {
	end = func() {}
	if reqCtx := reqctx.FromContext(ctx); reqCtx != nil {
		var traceSpan *gtprof.TraceSpan
		traceSpan, end = StartContextSpan(reqCtx, "http.func")
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
