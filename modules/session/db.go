// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gitea.dev/models/auth"
	"gitea.dev/modules/timeutil"

	"gitea.com/go-chi/session"
)

// DBStore represents a session store implementation based on the DB.
type DBStore struct {
	sid   string
	lock  sync.RWMutex
	data  map[any]any
	dirty bool
}

func dbContext() context.Context {
	return context.Background()
}

// NewDBStore creates and returns a DB session store.
func NewDBStore(sid string, kv map[any]any) *DBStore {
	return &DBStore{
		sid:  sid,
		data: kv,
	}
}

// Set sets value to given key in session.
func (s *DBStore) Set(key, val any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data[key] = val
	s.dirty = true
	return nil
}

// Get gets value by given key in session.
func (s *DBStore) Get(key any) any {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.data[key]
}

// Delete delete a key from session.
func (s *DBStore) Delete(key any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.data[key]; ok {
		s.dirty = true
	}
	delete(s.data, key)
	return nil
}

// ID returns current session ID.
func (s *DBStore) ID() string {
	return s.sid
}

// Release releases resource and save data to provider.
func (s *DBStore) Release() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Skip empty data, which includes expired rows whose expiry refresh would revive their data
	if len(s.data) == 0 {
		return nil
	}
	if !s.dirty {
		return auth.UpdateSessionExpiry(dbContext(), s.sid)
	}

	data, err := session.EncodeGob(s.data)
	if err != nil {
		return err
	}

	err = auth.UpdateSession(dbContext(), s.sid, data)
	s.dirty = err != nil
	return err
}

// Flush deletes all session data.
func (s *DBStore) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data = make(map[any]any)
	s.dirty = true
	return nil
}

// DBProvider represents a DB session provider implementation.
type DBProvider struct {
	maxLifetime int64
}

// Init initializes DB session provider.
// connStr: username:password@protocol(address)/dbname?param=value
func (p *DBProvider) Init(maxLifetime int64, connStr string) error {
	p.maxLifetime = maxLifetime
	return nil
}

// Read returns raw session store by session ID.
func (p *DBProvider) Read(sid string) (session.RawStore, error) {
	s, err := auth.ReadSession(dbContext(), sid)
	if err != nil {
		return nil, err
	}

	var kv map[any]any
	if len(s.Data) == 0 || s.Expiry.Add(p.maxLifetime) <= timeutil.TimeStampNow() {
		kv = make(map[any]any)
	} else {
		kv, err = session.DecodeGob(s.Data)
		if err != nil {
			return nil, err
		}
	}

	return NewDBStore(sid, kv), nil
}

// Exist returns true if session with given ID exists.
func (p *DBProvider) Exist(sid string) (bool, error) {
	has, err := auth.ExistSession(dbContext(), sid)
	if err != nil {
		return false, fmt.Errorf("session/DB: error checking existence: %w", err)
	}
	return has, nil
}

// Destroy deletes a session by session ID.
func (p *DBProvider) Destroy(sid string) error {
	return auth.DestroySession(dbContext(), sid)
}

// Regenerate regenerates a session store from old session ID to new one.
func (p *DBProvider) Regenerate(oldsid, sid string) (_ session.RawStore, err error) {
	s, err := auth.RegenerateSession(dbContext(), oldsid, sid)
	if err != nil {
		return nil, err
	}

	var kv map[any]any
	if len(s.Data) == 0 || s.Expiry.Add(p.maxLifetime) <= timeutil.TimeStampNow() {
		kv = make(map[any]any)
	} else {
		kv, err = session.DecodeGob(s.Data)
		if err != nil {
			return nil, err
		}
	}

	return NewDBStore(sid, kv), nil
}

// Count counts and returns number of sessions.
func (p *DBProvider) Count() (int, error) {
	total, err := auth.CountSessions(dbContext())
	if err != nil {
		return 0, fmt.Errorf("session/DB: error counting records: %w", err)
	}
	return int(total), nil
}

// GC calls GC to clean expired sessions.
func (p *DBProvider) GC() {
	if err := auth.CleanupSessions(dbContext(), p.maxLifetime); err != nil {
		log.Printf("session/DB: error garbage collecting: %v", err)
	}
}

func init() {
	session.Register("db", &DBProvider{})
}
