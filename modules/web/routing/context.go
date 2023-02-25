// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type contextKeyType struct{}

var contextKey contextKeyType

// UpdateFuncInfo updates a context's func info
func UpdateFuncInfo(ctx context.Context, funcInfo *FuncInfo) (context.Context, context.CancelFunc) {
	tracer := otel.GetTracerProvider().Tracer("routing")

	traceCtx, span := tracer.Start(ctx, funcInfo.String())
	cancel := func() {
		span.End()
	}

	record, ok := ctx.Value(contextKey).(*requestRecord)
	if !ok {
		return traceCtx, cancel
	}
	record.lock.Lock()
	record.funcInfo = funcInfo
	record.lock.Unlock()
	return traceCtx, cancel
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
func UpdatePanicError(ctx context.Context, err interface{}) {
	span := trace.SpanFromContext(ctx)
	if errErr, ok := err.(error); ok {
		span.RecordError(errErr, trace.WithStackTrace(true))
	} else {
		span.RecordError(fmt.Errorf("%v", err), trace.WithStackTrace(true))
	}

	record, ok := ctx.Value(contextKey).(*requestRecord)
	if !ok {
		return
	}

	record.lock.Lock()
	record.panicError = err
	record.lock.Unlock()
}
