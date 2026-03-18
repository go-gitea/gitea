// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVirtualStore_BasicOperations(t *testing.T) {
	p := &VirtualSessionProvider{provider: &mockProvider{}}
	store := NewVirtualStore(p, "test-sid", make(map[any]any))

	assert.Equal(t, "test-sid", store.ID())

	require.NoError(t, store.Set("uid", int64(42)))
	assert.Equal(t, int64(42), store.Get("uid"))

	require.NoError(t, store.Delete("uid"))
	assert.Nil(t, store.Get("uid"))
}

func TestInertVirtualStore_IgnoresWrites(t *testing.T) {
	store := NewInertVirtualStore("dead-sid")

	assert.Equal(t, "dead-sid", store.ID())

	// Set should be silently ignored
	require.NoError(t, store.Set("uid", int64(42)))
	assert.Nil(t, store.Get("uid"))

	// Release should be a no-op
	require.NoError(t, store.Release())
}

func TestVirtualSessionProvider_DestroyTombstone(t *testing.T) {
	mp := &mockProvider{sessions: map[string]map[any]any{
		"sid-1": {"uid": int64(1)},
	}}
	vsp := &VirtualSessionProvider{provider: mp}

	// Before destroy, Read returns data from the mock provider
	store, err := vsp.Read("sid-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), store.Get("uid"))

	// Destroy the session
	require.NoError(t, vsp.Destroy("sid-1"))

	// Simulate concurrent request recreating the session file:
	// the mock provider now has the session again
	mp.sessions["sid-1"] = map[any]any{"uid": int64(1)}

	// Read after destroy should return inert store due to tombstone
	store, err = vsp.Read("sid-1")
	require.NoError(t, err)
	assert.Nil(t, store.Get("uid"), "tombstoned session should return empty store")

	// The inert store should ignore writes and releases
	require.NoError(t, store.Set("uid", int64(99)))
	assert.Nil(t, store.Get("uid"))
	require.NoError(t, store.Release())
}

func TestVirtualSessionProvider_ReadNonExistent(t *testing.T) {
	mp := &mockProvider{sessions: map[string]map[any]any{}}
	vsp := &VirtualSessionProvider{provider: mp}

	// Read for a session that doesn't exist returns a VirtualStore
	store, err := vsp.Read("no-such-sid")
	require.NoError(t, err)
	assert.Nil(t, store.Get("uid"))
	assert.Equal(t, "no-such-sid", store.ID())
}

func TestVirtualSessionProvider_ExistAlwaysTrue(t *testing.T) {
	vsp := &VirtualSessionProvider{provider: &mockProvider{}}

	exists, err := vsp.Exist("anything")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestDestroySessionByID_NilProvider(t *testing.T) {
	// Ensure DestroySessionByID doesn't panic when globalProvider is nil
	old := globalProvider
	globalProvider = nil
	defer func() { globalProvider = old }()

	assert.NoError(t, DestroySessionByID("anything"))
}

// mockProvider is a minimal in-memory session.Provider for testing
type mockProvider struct {
	sessions map[string]map[any]any
}

func (m *mockProvider) Init(_ int64, _ string) error { return nil }

func (m *mockProvider) Read(sid string) (RawStore, error) {
	if m.sessions == nil {
		m.sessions = make(map[string]map[any]any)
	}
	data, ok := m.sessions[sid]
	if !ok {
		data = make(map[any]any)
		m.sessions[sid] = data
	}
	return &mockStore{sid: sid, data: data}, nil
}

func (m *mockProvider) Exist(sid string) (bool, error) {
	if m.sessions == nil {
		return false, nil
	}
	_, ok := m.sessions[sid]
	return ok, nil
}

func (m *mockProvider) Destroy(sid string) error {
	delete(m.sessions, sid)
	return nil
}

func (m *mockProvider) Regenerate(oldsid, sid string) (RawStore, error) {
	data := m.sessions[oldsid]
	delete(m.sessions, oldsid)
	if data == nil {
		data = make(map[any]any)
	}
	m.sessions[sid] = data
	return &mockStore{sid: sid, data: data}, nil
}

func (m *mockProvider) Count() (int, error) {
	return len(m.sessions), nil
}

func (m *mockProvider) GC() {}

// mockStore is a minimal in-memory RawStore for testing
type mockStore struct {
	sid  string
	data map[any]any
}

func (s *mockStore) Set(key, val any) error { s.data[key] = val; return nil }
func (s *mockStore) Get(key any) any        { return s.data[key] }
func (s *mockStore) Delete(key any) error   { delete(s.data, key); return nil }
func (s *mockStore) ID() string             { return s.sid }
func (s *mockStore) Release() error         { return nil }
func (s *mockStore) Flush() error {
	for k := range s.data {
		delete(s.data, k)
	}
	return nil
}
