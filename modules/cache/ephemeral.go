// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// EphemeralCache is a cache that can be used to store data in a request level context
// This is useful for caching data that is expensive to calculate and is likely to be
// used multiple times in a request.
type EphemeralCache struct {
	data          map[any]map[any]any
	lock          sync.RWMutex
	created       time.Time
	checkLifeTime time.Duration
}

var timeNow = time.Now

func NewEphemeralCache(checkLifeTime ...time.Duration) *EphemeralCache {
	return &EphemeralCache{
		data:          make(map[any]map[any]any),
		created:       timeNow(),
		checkLifeTime: util.OptionalArg(checkLifeTime, 0),
	}
}

func (cc *EphemeralCache) checkExceededLifeTime(tp, key any) bool {
	if cc.checkLifeTime > 0 && timeNow().Sub(cc.created) > cc.checkLifeTime {
		log.Warn("EphemeralCache is expired, is highly likely to be abused for long-life tasks: %v, %v", tp, key)
		return true
	}
	return false
}

func (cc *EphemeralCache) Get(tp, key any) (any, bool) {
	if cc.checkExceededLifeTime(tp, key) {
		return nil, false
	}
	cc.lock.RLock()
	defer cc.lock.RUnlock()
	ret, ok := cc.data[tp][key]
	return ret, ok
}

func (cc *EphemeralCache) Put(tp, key, value any) {
	if cc.checkExceededLifeTime(tp, key) {
		return
	}

	cc.lock.Lock()
	defer cc.lock.Unlock()

	d := cc.data[tp]
	if d == nil {
		d = make(map[any]any)
		cc.data[tp] = d
	}
	d[key] = value
}

func (cc *EphemeralCache) Delete(tp, key any) {
	if cc.checkExceededLifeTime(tp, key) {
		return
	}

	cc.lock.Lock()
	defer cc.lock.Unlock()
	delete(cc.data[tp], key)
}

func GetWithEphemeralCache[T, K any](ctx context.Context, c *EphemeralCache, groupKey string, targetKey K, f func(context.Context, K) (T, error)) (T, error) {
	v, has := c.Get(groupKey, targetKey)
	if vv, ok := v.(T); has && ok {
		return vv, nil
	}
	t, err := f(ctx, targetKey)
	if err != nil {
		return t, err
	}
	c.Put(groupKey, targetKey, t)
	return t, nil
}
