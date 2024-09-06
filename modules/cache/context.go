// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"
)

// cacheContext is a context that can be used to cache data in a request level context
// This is useful for caching data that is expensive to calculate and is likely to be
// used multiple times in a request.
type cacheContext struct {
	data    map[any]map[any]any
	lock    sync.RWMutex
	discard bool
}

func (cc *cacheContext) Get(tp, key any) any {
	cc.lock.RLock()
	defer cc.lock.RUnlock()
	return cc.data[tp][key]
}

func (cc *cacheContext) Put(tp, key, value any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.discard {
		return
	}

	d := cc.data[tp]
	if d == nil {
		d = make(map[any]any)
		cc.data[tp] = d
	}
	d[key] = value
}

func (cc *cacheContext) Delete(tp, key any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	delete(cc.data[tp], key)
}

func (cc *cacheContext) Discard() {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	cc.data = nil
	cc.discard = true
}

func (cc *cacheContext) isDiscard() bool {
	cc.lock.RLock()
	defer cc.lock.RUnlock()
	return cc.discard
}

var cacheContextKey = struct{}{}

/*
Since there are both WithCacheContext and WithNoCacheContext,
it may be confusing when there is nesting.

Some cases to explain the design:

When:
- A, B or C means a cache context.
- A', B' or C' means a discard cache context.
- ctx means context.Backgrand().
- A(ctx) means a cache context with ctx as the parent context.
- B(A(ctx)) means a cache context with A(ctx) as the parent context.
- With is alias of WithCacheContext.
- WithNo is alias of WithNoCacheContext.

So:
- With(ctx) -> A(ctx)
- With(With(ctx)) -> A(ctx), not B(A(ctx)), always reuse parent cache context if possible.
- With(With(With(ctx))) -> A(ctx), not C(B(A(ctx))), ditto.
- WithNo(ctx) -> ctx, not A'(ctx), don't create new cache context if we don't have to.
- WithNo(With(ctx)) -> A'(ctx)
- WithNo(WithNo(With(ctx))) -> A'(ctx), not B'(A'(ctx)), don't create new cache context if we don't have to.
- With(WithNo(With(ctx))) -> B(A'(ctx)), not A(ctx), never reuse a discard cache context.
- WithNo(With(WithNo(With(ctx)))) -> B'(A'(ctx))
- With(WithNo(With(WithNo(With(ctx))))) -> C(B'(A'(ctx))), so there's always only one not-discard cache context.
*/

func WithCacheContext(ctx context.Context) context.Context {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if !c.isDiscard() {
			// reuse parent context
			return ctx
		}
	}
	return context.WithValue(ctx, cacheContextKey, &cacheContext{
		data: make(map[any]map[any]any),
	})
}

func WithNoCacheContext(ctx context.Context) context.Context {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		// The caller want to run long-life tasks, but the parent context is a cache context.
		// So we should disable and clean the cache data, or it will be kept in memory for a long time.
		c.Discard()
		return ctx
	}

	return ctx
}

func GetContextData(ctx context.Context, tp, key any) any {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		return c.Get(tp, key)
	}
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		c.Put(tp, key, value)
		return
	}
}

func RemoveContextData(ctx context.Context, tp, key any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		c.Delete(tp, key)
	}
}

// GetWithContextCache returns the cache value of the given key in the given context.
func GetWithContextCache[T any](ctx context.Context, cacheGroupKey string, cacheTargetID any, f func() (T, error)) (T, error) {
	v := GetContextData(ctx, cacheGroupKey, cacheTargetID)
	if vv, ok := v.(T); ok {
		return vv, nil
	}
	t, err := f()
	if err != nil {
		return t, err
	}
	SetContextData(ctx, cacheGroupKey, cacheTargetID, t)
	return t, nil
}
