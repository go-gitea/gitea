// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/json"

	"gitea.com/go-chi/session"
	couchbase "gitea.com/go-chi/session/couchbase"
	memcache "gitea.com/go-chi/session/memcache"
	mysql "gitea.com/go-chi/session/mysql"
	postgres "gitea.com/go-chi/session/postgres"
)

// tombstoneTTL is how long a destroyed session ID is remembered so that
// concurrent requests releasing after destruction cannot recreate the session
// or be re-authenticated.
const tombstoneTTL = 10 * time.Minute

// VirtualSessionProvider represents a shadowed session provider implementation.
// It wraps a real session provider and adds "tombstone" tracking for destroyed
// sessions so that concurrent requests (e.g. EventSource) cannot accidentally
// recreate a session file by calling Release() after the file was deleted.
type VirtualSessionProvider struct {
	lock     sync.RWMutex
	provider session.Provider

	// destroyedSIDs tracks recently destroyed session IDs.
	// When a session is destroyed, concurrent requests that already hold
	// a FileStore reference may call Release() and recreate the file.
	// By tracking destroyed IDs, Read() returns an inert VirtualStore
	// that prevents re-authentication and avoids recreating the file.
	// Entries self-expire after tombstoneTTL via time.AfterFunc so the map
	// stays bounded regardless of session GC interval.
	destroyedSIDs sync.Map // sid -> time.Time
}

// Init initializes the cookie session provider with the given config.
func (o *VirtualSessionProvider) Init(gcLifetime int64, config string) error {
	var opts session.Options
	if err := json.Unmarshal([]byte(config), &opts); err != nil {
		return err
	}
	// Note that these options are unprepared so we can't just use NewManager here.
	// Nor can we access the provider map in session.
	// So we will just have to do this by hand.
	// This is only slightly more wrong than modules/setting/session.go:23
	switch opts.Provider {
	case "memory":
		o.provider = &session.MemProvider{}
	case "file":
		o.provider = &session.FileProvider{}
	case "redis":
		o.provider = &RedisProvider{}
	case "db":
		o.provider = &DBProvider{}
	case "mysql":
		o.provider = &mysql.MysqlProvider{}
	case "postgres":
		o.provider = &postgres.PostgresProvider{}
	case "couchbase":
		o.provider = &couchbase.CouchbaseProvider{}
	case "memcache":
		o.provider = &memcache.MemcacheProvider{}
	default:
		return fmt.Errorf("VirtualSessionProvider: Unknown Provider: %s", opts.Provider)
	}
	SetGlobalProvider(o)
	return o.provider.Init(gcLifetime, opts.ProviderConfig)
}

// Read returns raw session store by session ID.
func (o *VirtualSessionProvider) Read(sid string) (session.RawStore, error) {
	// Check tombstone first: if this session was recently destroyed, return
	// an inert store regardless of whether the file was recreated by a
	// concurrent request's Release(). Also re-delete the file to clean up.
	if _, destroyed := o.destroyedSIDs.Load(sid); destroyed {
		o.lock.Lock()
		_ = o.provider.Destroy(sid)
		o.lock.Unlock()
		return NewInertVirtualStore(sid), nil
	}

	o.lock.RLock()
	defer o.lock.RUnlock()
	if exist, err := o.provider.Exist(sid); err == nil && exist {
		return o.provider.Read(sid)
	} else if err != nil {
		return nil, fmt.Errorf("check if '%s' exist failed: %w", sid, err)
	}
	kv := make(map[any]any)
	return NewVirtualStore(o, sid, kv), nil
}

// Exist returns true if session with given ID exists.
func (o *VirtualSessionProvider) Exist(sid string) (bool, error) {
	return true, nil
}

// Destroy deletes a session by session ID.
func (o *VirtualSessionProvider) Destroy(sid string) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.destroyedSIDs.Store(sid, time.Now())
	// Self-expire the tombstone so the map stays bounded between session GCs.
	time.AfterFunc(tombstoneTTL, func() {
		o.destroyedSIDs.Delete(sid)
	})
	return o.provider.Destroy(sid)
}

// Regenerate regenerates a session store from old session ID to new one.
func (o *VirtualSessionProvider) Regenerate(oldsid, sid string) (session.RawStore, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.provider.Regenerate(oldsid, sid)
}

// Count counts and returns number of sessions.
func (o *VirtualSessionProvider) Count() (int, error) {
	o.lock.RLock()
	defer o.lock.RUnlock()
	return o.provider.Count()
}

// GC calls GC to clean expired sessions.
func (o *VirtualSessionProvider) GC() {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.provider.GC()

	// Re-destroy any sessions that may have been recreated by concurrent
	// requests releasing after destruction. Tombstones themselves expire
	// via time.AfterFunc in Destroy() so no manual cleanup is needed here.
	o.destroyedSIDs.Range(func(key, _ any) bool {
		_ = o.provider.Destroy(key.(string))
		return true
	})
}

func init() {
	session.Register("VirtualSession", &VirtualSessionProvider{})
}

// VirtualStore represents a virtual session store implementation.
type VirtualStore struct {
	p           *VirtualSessionProvider
	sid         string
	lock        sync.RWMutex
	data        map[any]any
	released    bool
	invalidated bool // true for destroyed sessions — all writes are no-ops
}

// NewVirtualStore creates and returns a virtual session store.
func NewVirtualStore(p *VirtualSessionProvider, sid string, kv map[any]any) *VirtualStore {
	return &VirtualStore{
		p:    p,
		sid:  sid,
		data: kv,
	}
}

// NewInertVirtualStore creates a VirtualStore for a destroyed (tombstoned) session.
// It silently ignores all Set and Release calls so that concurrent requests
// cannot inadvertently recreate the session file or store authentication data.
func NewInertVirtualStore(sid string) *VirtualStore {
	return &VirtualStore{
		sid:         sid,
		data:        make(map[any]any),
		invalidated: true,
	}
}

// Set sets value to given key in session.
func (s *VirtualStore) Set(key, val any) error {
	if s.invalidated {
		return nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data[key] = val
	return nil
}

// Get gets value by given key in session.
func (s *VirtualStore) Get(key any) any {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.data[key]
}

// Delete delete a key from session.
func (s *VirtualStore) Delete(key any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.data, key)
	return nil
}

// ID returns current session ID.
func (s *VirtualStore) ID() string {
	return s.sid
}

// Release releases resource and save data to provider.
func (s *VirtualStore) Release() error {
	if s.invalidated {
		return nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	// Now need to lock the provider
	s.p.lock.Lock()
	defer s.p.lock.Unlock()
	if len(s.data) > 0 {
		// Now ensure that we don't exist!
		realProvider := s.p.provider

		if !s.released {
			if exist, err := realProvider.Exist(s.sid); err == nil && exist {
				// This is an error!
				return fmt.Errorf("new sid '%s' already exists", s.sid)
			} else if err != nil {
				return fmt.Errorf("check if '%s' exist failed: %w", s.sid, err)
			}
		}
		realStore, err := realProvider.Read(s.sid)
		if err != nil {
			return err
		}
		if err := realStore.Flush(); err != nil {
			return err
		}
		for key, value := range s.data {
			if err := realStore.Set(key, value); err != nil {
				return err
			}
		}
		err = realStore.Release()
		if err == nil {
			s.released = true
		}
		return err
	}
	return nil
}

// Flush deletes all session data.
func (s *VirtualStore) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data = make(map[any]any)
	return nil
}
