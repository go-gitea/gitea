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

func WithCacheContext(ctx context.Context) context.Context {
	if c := GetContextCache(ctx); c != nil {
		return ctx
	}
	return context.WithValue(ctx, cacheContextKey, NewEphemeralCache(contextCacheLifetime))
}

func GetContextCache(ctx context.Context) *EphemeralCache {
	c, _ := ctx.Value(cacheContextKey).(*EphemeralCache)
	return c
}

// GetWithContextCache returns the cache value of the given key in the given context.
// FIXME: in some cases, the "context cache" should not be used, because it has uncontrollable behaviors
// For example, these calls:
// * GetWithContextCache(TargetID) -> OtherCodeCreateModel(TargetID) -> GetWithContextCache(TargetID)
// Will cause the second call is not able to get the correct created target.
// UNLESS it is certain that the target won't be changed during the request, DO NOT use it.
func GetWithContextCache[T, K any](ctx context.Context, groupKey string, targetKey K, f func(context.Context, K) (T, error)) (T, error) {
	if c := GetContextCache(ctx); c != nil {
		return GetWithEphemeralCache(ctx, c, groupKey, targetKey, f)
	}
	return f(ctx, targetKey)
}
