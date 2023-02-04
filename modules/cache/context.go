// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"
)

type cacheContext struct {
	ctx  context.Context
	Data map[any]map[any]any
	lock sync.RWMutex
}

func (cc *cacheContext) Get(tp, key any) any {
	cc.lock.RLock()
	defer cc.lock.RUnlock()
	if cc.Data[tp] == nil {
		return nil
	}
	return cc.Data[tp][key]
}

func (cc *cacheContext) Put(tp, key, value any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	if cc.Data[tp] == nil {
		cc.Data[tp] = make(map[any]any)
	}
	cc.Data[tp][key] = value
}

func (cc *cacheContext) Delete(tp, key any) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	if cc.Data[tp] == nil {
		cc.Data[tp] = make(map[any]any)
	}
	delete(cc.Data[tp], key)
}

var cacheContextKey = struct{}{}

func WithCacheContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheContextKey, &cacheContext{
		ctx:  ctx,
		Data: make(map[any]map[any]any),
	})
}

func GetContextData(ctx context.Context, tp, key any) any {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		return c.Get(tp, key)
	}
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		c.Put(tp, key, value)
	}
}

func RemoveContextData(ctx context.Context, tp, key any) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.Value(cacheContextKey).(*cacheContext); ok {
		c.Delete(tp, key)
	}
}
