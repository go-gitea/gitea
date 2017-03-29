// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package openid

import (
	"sync"
	"time"

	"github.com/yohcop/openid-go"
)

type timedDiscoveredInfo struct {
	info openid.DiscoveredInfo
	time time.Time
}

type timedDiscoveryCache struct {
	cache map[string]timedDiscoveredInfo
	ttl   time.Duration
	mutex *sync.Mutex
}

func newTimedDiscoveryCache(ttl time.Duration) *timedDiscoveryCache {
	return &timedDiscoveryCache{cache: map[string]timedDiscoveredInfo{}, ttl: ttl, mutex: &sync.Mutex{}}
}

func (s *timedDiscoveryCache) Put(id string, info openid.DiscoveredInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cache[id] = timedDiscoveredInfo{info: info, time: time.Now()}
}

// Delete timed-out cache entries
func (s *timedDiscoveryCache) cleanTimedOut() {
	now := time.Now()
	for k, e := range s.cache {
		diff := now.Sub(e.time)
		if diff > s.ttl {
			delete(s.cache, k)
		}
	}
}

func (s *timedDiscoveryCache) Get(id string) openid.DiscoveredInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Delete old cached while we are at it.
	s.cleanTimedOut()

	if info, has := s.cache[id]; has {
		return info.info
	}
	return nil
}
