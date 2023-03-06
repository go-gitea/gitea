// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// cacheContext is a context that can be used to cache data in a request level context
// This is useful for caching data that is expensive to calculate and is likely to be
// used multiple times in a request.
type cacheContext struct {
	data    map[any]map[any]any
	lock    sync.RWMutex
	created time.Time
}

// noCacheContext is a context that should discard cache data in context
type noCacheContext struct{}

func (cc *cacheContext) Get(tp, key any) any {
	cc.lock.RLock()
	defer cc.lock.RUnlock()
	if cc.data[tp] == nil {
		return nil
	}
	return cc.data[tp][key]
}

func (cc *cacheContext) Put(tp, key, value any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	if cc.data[tp] == nil {
		cc.data[tp] = make(map[any]any)
	}
	cc.data[tp][key] = value
}

func (cc *cacheContext) Delete(tp, key any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	if cc.data[tp] == nil {
		return
	}
	delete(cc.data[tp], key)
}

// cacheContextLifetime is the max lifetime of cacheContext.
// Since cacheContext is used to cache data in a request level context, 10s is enough.
// If a cacheContext is used more than 10s, it's probably misuse.
const cacheContextLifetime = 10 * time.Second

var timeNow = time.Now

func (cc *cacheContext) Expired() bool {
	return timeNow().Sub(cc.created) > cacheContextLifetime
}

var cacheContextKey = struct{}{}

func WithCacheContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheContextKey, &cacheContext{
		data:    make(map[any]map[any]any),
		created: timeNow(),
	})
}

func WithNoCacheContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheContextKey, noCacheContext{})
}

func GetContextData(ctx context.Context, tp, key any) any {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if c.Expired() {
			// The warning means that the cache context is misused for long-life task,
			// it can be resolved with WithNoCacheContext(ctx).
			log.Warn("cache context is expired: %v", c)
			return nil
		}
		return c.Get(tp, key)
	}
	if _, ok := ctx.Value(cacheContextKey).(noCacheContext); ok {
		return nil
	}
	// The warning means that an original context is treated as a cache context,
	// it can be resolved with WithNoCacheContext(ctx) or WithCacheContext(ctx).
	// If you are not sure which one should be picked, it's always a safe way to use WithNoCacheContext(ctx).
	log.Warn("cannot get cache context when getting data: %v", ctx)
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if c.Expired() {
			// The warning means that the cache context is misused for long-life task,
			// it can be resolved with WithNoCacheContext(ctx).
			log.Warn("cache context is expired: %v", c)
			return
		}
		c.Put(tp, key, value)
		return
	}
	if _, ok := ctx.Value(cacheContextKey).(noCacheContext); ok {
		return
	}
	// The warning means that an original context is treated as a cache context,
	// it can be resolved with WithNoCacheContext(ctx) or WithCacheContext(ctx).
	// If you are not sure which one should be picked, it's always a safe way to use WithNoCacheContext(ctx).
	log.Warn("cannot get cache context when setting data: %v", ctx)
}

func RemoveContextData(ctx context.Context, tp, key any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if c.Expired() {
			log.Warn("cache context is expired: %v", c)
			return
		}
		c.Delete(tp, key)
	}
	if _, ok := ctx.Value(cacheContextKey).(noCacheContext); ok {
		return
	}
	// The warning means that an original context is treated as a cache context,
	// it can be resolved with WithNoCacheContext(ctx) or WithCacheContext(ctx).
	// If you are not sure which one should be picked, it's always a safe way to use WithNoCacheContext(ctx).
	log.Warn("cannot get cache context when removing data: %v", ctx)
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
