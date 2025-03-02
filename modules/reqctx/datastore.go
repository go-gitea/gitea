// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package reqctx

import (
	"context"
	"io"
	"sync"

	"code.gitea.io/gitea/modules/process"
)

type ContextDataProvider interface {
	GetData() ContextData
}

type ContextData map[string]any

func (ds ContextData) GetData() ContextData {
	return ds
}

func (ds ContextData) MergeFrom(other ContextData) ContextData {
	for k, v := range other {
		ds[k] = v
	}
	return ds
}

// RequestDataStore is a short-lived context-related object that is used to store request-specific data.
type RequestDataStore interface {
	GetData() ContextData
	SetContextValue(k, v any)
	GetContextValue(key any) any
	AddCleanUp(f func())
	AddCloser(c io.Closer)
}

type requestDataStoreKeyType struct{}

var RequestDataStoreKey requestDataStoreKeyType

type requestDataStore struct {
	data ContextData

	mu           sync.RWMutex
	values       map[any]any
	cleanUpFuncs []func()
}

func (r *requestDataStore) GetContextValue(key any) any {
	if key == RequestDataStoreKey {
		return r
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.values[key]
}

func (r *requestDataStore) SetContextValue(k, v any) {
	r.mu.Lock()
	r.values[k] = v
	r.mu.Unlock()
}

// GetData and the underlying ContextData are not thread-safe, callers should ensure thread-safety.
func (r *requestDataStore) GetData() ContextData {
	if r.data == nil {
		r.data = make(ContextData)
	}
	return r.data
}

func (r *requestDataStore) AddCleanUp(f func()) {
	r.mu.Lock()
	r.cleanUpFuncs = append(r.cleanUpFuncs, f)
	r.mu.Unlock()
}

func (r *requestDataStore) AddCloser(c io.Closer) {
	r.AddCleanUp(func() { _ = c.Close() })
}

func (r *requestDataStore) cleanUp() {
	for _, f := range r.cleanUpFuncs {
		f()
	}
}

type RequestContext interface {
	context.Context
	RequestDataStore
}

func FromContext(ctx context.Context) RequestContext {
	// here we must use the current ctx and the underlying store
	// the current ctx guarantees that the ctx deadline/cancellation/values are respected
	// the underlying store guarantees that the request-specific data is available
	if store := GetRequestDataStore(ctx); store != nil {
		return &requestContext{Context: ctx, RequestDataStore: store}
	}
	return nil
}

func GetRequestDataStore(ctx context.Context) RequestDataStore {
	if req, ok := ctx.Value(RequestDataStoreKey).(*requestDataStore); ok {
		return req
	}
	return nil
}

type requestContext struct {
	context.Context
	RequestDataStore
}

func (c *requestContext) Value(key any) any {
	if v := c.GetContextValue(key); v != nil {
		return v
	}
	return c.Context.Value(key)
}

func NewRequestContext(parentCtx context.Context, profDesc string) (_ context.Context, finished func()) {
	ctx, _, processFinished := process.GetManager().AddTypedContext(parentCtx, profDesc, process.RequestProcessType, true)
	store := &requestDataStore{values: make(map[any]any)}
	reqCtx := &requestContext{Context: ctx, RequestDataStore: store}
	return reqCtx, func() {
		store.cleanUp()
		processFinished()
	}
}

// NewRequestContextForTest creates a new RequestContext for testing purposes
// It doesn't add the context to the process manager, nor do cleanup
func NewRequestContextForTest(parentCtx context.Context) context.Context {
	return &requestContext{Context: parentCtx, RequestDataStore: &requestDataStore{values: make(map[any]any)}}
}
