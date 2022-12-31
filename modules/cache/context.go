// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import "context"

type CacheContext struct {
	ctx  context.Context
	Data map[any]map[any]any
}

type CacheContextKey struct{}

func WithCacheContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, &CacheContextKey{}, &CacheContext{
		ctx:  ctx,
		Data: make(map[any]map[any]any),
	})
}

func GetContextData(ctx context.Context, tp, key any) any {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(&CacheContextKey{}).(*CacheContext); ok {
		if c.Data[tp] != nil {
			return c.Data[tp][key]
		}
	}
	return nil
}

func SetContextData(ctx context.Context, tp, key, value any) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.Value(&CacheContextKey{}).(*CacheContext); ok {
		if c.Data[tp] == nil {
			c.Data[tp] = make(map[any]any)
		}
		c.Data[tp][key] = value
	}
}

func RemoveContextData(ctx context.Context, tp, key any) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.Value(&CacheContextKey{}).(*CacheContext); ok {
		if c.Data[tp] == nil {
			c.Data[tp] = make(map[any]any)
		}
		delete(c.Data[tp], key)
	}
}
