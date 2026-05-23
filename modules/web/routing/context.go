// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/modules/gtprof"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/web/types"
)

type contextKeyType struct{}

var contextKey contextKeyType

func getRequestRecord(ctx context.Context) *requestRecord {
	record, _ := ctx.Value(contextKey).(*requestRecord)
	return record
}

// RecordFuncInfo records a func info into context
func RecordFuncInfo(ctx context.Context, funcInfo *FuncInfo) (end func()) {
	end = func() {}
	if reqCtx := reqctx.FromContext(ctx); reqCtx != nil {
		var traceSpan *gtprof.TraceSpan
		traceSpan, end = gtprof.GetTracer().StartInContext(reqCtx, "http.func")
		traceSpan.SetAttributeString("func", funcInfo.shortName)
	}
	if record := getRequestRecord(ctx); record != nil {
		record.lock.Lock()
		record.funcInfo = funcInfo
		record.lock.Unlock()
	}
	return end
}

func GetRequestRecordInfo(reqCtx context.Context) (ret struct {
	HasRecord     bool
	IsLongPolling bool
},
) {
	record := getRequestRecord(reqCtx)
	if record == nil {
		return ret
	}
	ret.HasRecord = true
	record.lock.RLock()
	ret.IsLongPolling = record.isLongPolling
	record.lock.RUnlock()
	return ret
}

// MarkLongPolling marks the request is a long-polling request, and the logger may output different message for it
func MarkLongPolling() types.PreMiddlewareProvider {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			record := getRequestRecord(req.Context()) // it must exist
			record.lock.Lock()
			record.isLongPolling = true
			record.logLevel = log.TRACE
			record.lock.Unlock()
			next.ServeHTTP(w, req)
		})
	}
}

func MarkLogLevelTrace(resp http.ResponseWriter, req *http.Request) {
	record := getRequestRecord(req.Context())
	if record == nil {
		return
	}

	record.lock.Lock()
	record.logLevel = log.TRACE
	record.lock.Unlock()
}

// UpdatePanicError updates a context's error info, a panic may be recovered by other middlewares, but we still need to know that.
func UpdatePanicError(ctx context.Context, err error) {
	record := getRequestRecord(ctx)
	if record == nil {
		return
	}

	record.lock.Lock()
	record.panicError = err
	record.lock.Unlock()
}
