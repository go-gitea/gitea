// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"
)

type contextKeyType struct{}

var contextKey contextKeyType

// UpdateFuncInfo updates a context's func info
func UpdateFuncInfo(ctx context.Context, funcInfo *FuncInfo) {
	record, ok := ctx.Value(contextKey).(*requestRecord)
	if !ok {
		return
	}

	record.lock.Lock()
	record.funcInfo = funcInfo
	record.lock.Unlock()
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
