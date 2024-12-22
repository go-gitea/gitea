// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package reqctx

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/process"
)

type RequestContextKeyType struct{}

var RequestContextKey RequestContextKeyType

// RequestContext is a short-lived context that is used to store request-specific data.
type RequestContext struct {
	ctx  context.Context
	data ContextData

	mu     sync.RWMutex
	values map[any]any

	cleanUpFuncs []func()
}

func (r *RequestContext) Deadline() (deadline time.Time, ok bool) {
	return r.ctx.Deadline()
}

func (r *RequestContext) Done() <-chan struct{} {
	return r.ctx.Done()
}

func (r *RequestContext) Err() error {
	return r.ctx.Err()
}

func (r *RequestContext) Value(key any) any {
	if key == RequestContextKey {
		return r
	}
	r.mu.RLock()
	if v, ok := r.values[key]; ok {
		r.mu.RUnlock()
		return v
	}
	r.mu.RUnlock()
	return r.ctx.Value(key)
}

func (r *RequestContext) SetContextValue(k, v any) {
	r.mu.Lock()
	r.values[k] = v
	r.mu.Unlock()
}

// GetData and the underlying ContextData are not thread-safe, callers should ensure thread-safety.
func (r *RequestContext) GetData() ContextData {
	if r.data == nil {
		r.data = make(ContextData)
	}
	return r.data
}

func (r *RequestContext) AddCleanUp(f func()) {
	r.mu.Lock()
	r.cleanUpFuncs = append(r.cleanUpFuncs, f)
	r.mu.Unlock()
}

func (r *RequestContext) cleanUp() {
	for _, f := range r.cleanUpFuncs {
		f()
	}
}

func GetRequestContext(ctx context.Context) *RequestContext {
	if req, ok := ctx.Value(RequestContextKey).(*RequestContext); ok {
		return req
	}
	return nil
}

func NewRequestContext(parentCtx context.Context, profDesc string) (_ *RequestContext, finished func()) {
	ctx, _, processFinished := process.GetManager().AddTypedContext(parentCtx, profDesc, process.RequestProcessType, true)
	reqCtx := &RequestContext{ctx: ctx, values: make(map[any]any)}
	return reqCtx, func() {
		reqCtx.cleanUp()
		processFinished()
	}
}

// NewRequestContextForTest creates a new RequestContext for testing purposes
// It doesn't add the context to the process manager, nor do cleanup
func NewRequestContextForTest(parentCtx context.Context) *RequestContext {
	reqCtx := &RequestContext{ctx: parentCtx, values: make(map[any]any)}
	return reqCtx
}
