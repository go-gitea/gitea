// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"bytes"
	"encoding/gob"
	"net/http"

	"gitea.com/go-chi/session"
)

type mockMemRawStore struct {
	s *session.MemStore
}

var _ session.RawStore = (*mockMemRawStore)(nil)

func (m *mockMemRawStore) Set(k, v any) error {
	// We need to use gob to encode the value, to make it have the same behavior as other stores and catch abuses.
	// Because gob needs to "Register" the type before it can encode it, and it's unable to decode a struct to "any" so use a map to help to decode the value.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(map[string]any{"v": v}); err != nil {
		return err
	}
	return m.s.Set(k, buf.Bytes())
}

func (m *mockMemRawStore) Get(k any) (ret any) {
	v, ok := m.s.Get(k).([]byte)
	if !ok {
		return nil
	}
	var w map[string]any
	_ = gob.NewDecoder(bytes.NewBuffer(v)).Decode(&w)
	return w["v"]
}

func (m *mockMemRawStore) Delete(k any) error {
	return m.s.Delete(k)
}

func (m *mockMemRawStore) ID() string {
	return m.s.ID()
}

func (m *mockMemRawStore) Release() error {
	return m.s.Release()
}

func (m *mockMemRawStore) Flush() error {
	return m.s.Flush()
}

type mockMemStore struct {
	*mockMemRawStore
}

var _ Store = (*mockMemStore)(nil)

func (m mockMemStore) Destroy(writer http.ResponseWriter, request *http.Request) error {
	return nil
}

func NewMockMemStore(sid string) Store {
	return &mockMemStore{&mockMemRawStore{session.NewMemStore(sid)}}
}
