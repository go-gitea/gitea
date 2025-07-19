// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"bytes"
	"encoding/gob"
	"net/http"

	"gitea.com/go-chi/session"
)

type MemStore struct {
	s *session.MemStore
}

var _ session.RawStore = (*MemStore)(nil)

func (m *MemStore) Set(k, v any) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(map[string]any{"v": v}); err != nil {
		return err
	}
	return m.s.Set(k, buf.Bytes())
}

func (m *MemStore) Get(k any) (ret any) {
	v, ok := m.s.Get(k).([]byte)
	if !ok {
		return nil
	}
	var w map[string]any
	_ = gob.NewDecoder(bytes.NewBuffer(v)).Decode(&w)
	return w["v"]
}

func (m *MemStore) Delete(k any) error {
	return m.s.Delete(k)
}

func (m *MemStore) ID() string {
	return m.s.ID()
}

func (m *MemStore) Release() error {
	return m.s.Release()
}

func (m *MemStore) Flush() error {
	return m.s.Flush()
}

type mockMemStore struct {
	*MemStore
}

var _ Store = (*mockMemStore)(nil)

func (m mockMemStore) Destroy(writer http.ResponseWriter, request *http.Request) error {
	return nil
}

func NewMockStore(sid string) Store {
	return &mockMemStore{&MemStore{session.NewMemStore(sid)}}
}
