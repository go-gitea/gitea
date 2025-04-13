// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"time"
)

type cacheContextKeyType struct{}

var cacheContextKey = cacheContextKeyType{}

// contextCacheLifetime is the max lifetime of context cache.
// Since context cache is used to cache data in a request level context, 5 minutes is enough.
// If a context cache is used more than 5 minutes, it's probably abused.
const contextCacheLifetime = 5 * time.Minute

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
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		if !c.isDiscard() {
			return ctx // reuse parent context
		}
	}
	return context.WithValue(ctx, cacheContextKey, NewEphemeralCache(contextCacheLifetime))
}

func withNoCacheContext(ctx context.Context) context.Context {
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		// The caller want to run long-life tasks, but the parent context is a cache context.
		// So we should disable and clean the cache data, or it will be kept in memory for a long time.
		c.discard()
		return ctx
	}
	return ctx
}

func getContextData(ctx context.Context, tp, key any) (any, bool) {
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		return c.Get(tp, key)
	}
	return nil, false
}

func setContextData(ctx context.Context, tp, key, value any) {
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		c.Put(tp, key, value)
	}
}

func removeContextData(ctx context.Context, tp, key any) {
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		c.Delete(tp, key)
	}
}

// GetWithContextCache returns the cache value of the given key in the given context.
// FIXME: in most cases, the "context cache" should not be used, because it has uncontrollable behaviors
// For example, these calls:
// * GetWithContextCache(TargetID) -> OtherCodeCreateModel(TargetID) -> GetWithContextCache(TargetID)
// Will cause the second call is not able to get the correct created target.
// UNLESS it is certain that the target won't be changed during the request, DO NOT use it.
func GetWithContextCache[T, K any](ctx context.Context, groupKey string, targetKey K, f func(context.Context, K) (T, error)) (T, error) {
	if c, ok := ctx.Value(cacheContextKey).(*EphemeralCache); ok {
		return GetWithEphemeralCache(ctx, c, groupKey, targetKey, f)
	}
	return f(ctx, targetKey)
}
