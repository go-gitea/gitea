// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"

	"code.gitea.io/gitea/modules/log"
)

// cacheContext is a context that can be used to cache data in a request level context
// This is useful for caching data that is expensive to calculate and is likely to be
// used multiple times in a request.
type cacheContext struct {
	ctx  context.Context
	data map[any]map[any]any
	lock sync.RWMutex
}

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

var cacheContextKey = struct{}{}

func WithCacheContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheContextKey, &cacheContext{
		ctx:  ctx,
		data: make(map[any]map[any]any),
	})
}

func GetContextData(ctx context.Context, tp, key any) any {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		return c.Get(tp, key)
	}
	log.Warn("cannot get cache context when getting data: %v", ctx)
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		c.Put(tp, key, value)
		return
	}
	log.Warn("cannot get cache context when setting data: %v", ctx)
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
