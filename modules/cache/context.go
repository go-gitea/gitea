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
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if !c.discard {
			// reuse parent context
			return ctx
		}
	}
	return context.WithValue(ctx, cacheContextKey, &cacheContext{
		data:    make(map[any]map[any]any),
		created: timeNow(),
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
		if c.Expired() {
			// The warning means that the cache context is misused for long-life task,
			// it can be resolved with WithNoCacheContext(ctx).
			log.Warn("cache context is expired, may be misused for long-life tasks: %v", c)
			return nil
		}
		return c.Get(tp, key)
	}
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if c.Expired() {
			// The warning means that the cache context is misused for long-life task,
			// it can be resolved with WithNoCacheContext(ctx).
			log.Warn("cache context is expired, may be misused for long-life tasks: %v", c)
			return
		}
		c.Put(tp, key, value)
		return
	}
}

func RemoveContextData(ctx context.Context, tp, key any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		if c.Expired() {
			// The warning means that the cache context is misused for long-life task,
			// it can be resolved with WithNoCacheContext(ctx).
			log.Warn("cache context is expired, may be misused for long-life tasks: %v", c)
			return
		}
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
