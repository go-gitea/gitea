// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"maps"
	"net/http"
	"sync"
	"time"

	"gitea.dev/modules/util"
)

type memoryBackend struct {
	lock        sync.Mutex
	maxLifetime time.Duration
	sessions    map[string]memorySession
}

type memorySession struct {
	data     []byte
	accessed time.Time
}

func newMemoryBackend(maxLifetime int64) *memoryBackend {
	return &memoryBackend{maxLifetime: time.Duration(maxLifetime) * time.Second, sessions: map[string]memorySession{}}
}

func (b *memoryBackend) expired(sess memorySession) bool {
	return time.Since(sess.accessed) > b.maxLifetime
}

func (b *memoryBackend) load(sid string) ([]byte, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	sess, ok := b.sessions[sid]
	if !ok || b.expired(sess) {
		return nil, nil
	}
	sess.accessed = time.Now()
	b.sessions[sid] = sess
	return sess.data, nil
}

func (b *memoryBackend) save(sid string, data []byte, create bool) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if _, exists := b.sessions[sid]; exists || create {
		b.sessions[sid] = memorySession{data: data, accessed: time.Now()}
	}
	return nil
}

func (b *memoryBackend) touch(string) error {
	return nil
}

func (b *memoryBackend) destroy(sid string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.sessions, sid)
	return nil
}

func (b *memoryBackend) gc() {
	b.lock.Lock()
	defer b.lock.Unlock()
	maps.DeleteFunc(b.sessions, func(_ string, sess memorySession) bool { return b.expired(sess) })
}

type mockMemStore struct {
	sid  string
	data map[any][]byte
}

var _ Store = (*mockMemStore)(nil)

// NewMockMemStore returns a store encoding each value like the real backends do, to catch values that can't be stored
func NewMockMemStore(sid string) Store {
	return &mockMemStore{sid: sid, data: map[any][]byte{}}
}

func (m *mockMemStore) Set(key, value any) error {
	encoded, err := util.PackData(map[any]any{key: value})
	if err == nil {
		m.data[key] = encoded
	}
	return err
}

func (m *mockMemStore) Get(key any) any {
	var decoded map[any]any
	_ = util.UnpackData(m.data[key], &decoded)
	return decoded[key]
}

func (m *mockMemStore) Delete(key any) error {
	delete(m.data, key)
	return nil
}

func (m *mockMemStore) ID() string {
	return m.sid
}

func (m *mockMemStore) Release() error {
	return nil
}

func (m *mockMemStore) Flush() error {
	clear(m.data)
	return nil
}

func (m *mockMemStore) Destroy(http.ResponseWriter, *http.Request) error {
	return nil
}

func (m *mockMemStore) Regenerate(http.ResponseWriter, *http.Request) {}
