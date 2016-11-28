// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package localstore

import (
	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/pingcap/tidb/terror"
)

var (
	_ kv.Snapshot = (*dbSnapshot)(nil)
	_ kv.Iterator = (*dbIter)(nil)
)

// dbSnapshot implements MvccSnapshot interface.
type dbSnapshot struct {
	store   *dbStore
	version kv.Version // transaction begin version
}

func newSnapshot(store *dbStore, ver kv.Version) *dbSnapshot {
	ss := &dbSnapshot{
		store:   store,
		version: ver,
	}

	return ss
}

// mvccSeek seeks for the first key in db which has a k >= key and a version <=
// snapshot's version, returns kv.ErrNotExist if such key is not found. If exact
// is true, only k == key can be returned.
func (s *dbSnapshot) mvccSeek(key kv.Key, exact bool) (kv.Key, []byte, error) {
	// Key layout:
	// ...
	// Key_verMax      -- (1)
	// ...
	// Key_ver+1       -- (2)
	// Key_ver         -- (3)
	// Key_ver-1       -- (4)
	// ...
	// Key_0           -- (5)
	// NextKey_verMax  -- (6)
	// ...
	// NextKey_ver+1   -- (7)
	// NextKey_ver     -- (8)
	// NextKey_ver-1   -- (9)
	// ...
	// NextKey_0       -- (10)
	// ...
	// EOF
	for {
		mvccKey := MvccEncodeVersionKey(key, s.version)
		mvccK, v, err := s.store.Seek([]byte(mvccKey)) // search for [3...EOF)
		if err != nil {
			if terror.ErrorEqual(err, engine.ErrNotFound) { // EOF
				return nil, nil, errors.Trace(kv.ErrNotExist)
			}
			return nil, nil, errors.Trace(err)
		}
		k, ver, err := MvccDecode(mvccK)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		// quick test for exact mode
		if exact {
			if key.Cmp(k) != 0 || isTombstone(v) {
				return nil, nil, errors.Trace(kv.ErrNotExist)
			}
			return k, v, nil
		}
		if ver.Ver > s.version.Ver {
			// currently on [6...7]
			key = k // search for [8...EOF) next loop
			continue
		}
		// currently on [3...5] or [8...10]
		if isTombstone(v) {
			key = k.Next() // search for (5...EOF) or (10..EOF) next loop
			continue
		}
		// target found
		return k, v, nil
	}
}

func (s *dbSnapshot) Get(key kv.Key) ([]byte, error) {
	_, v, err := s.mvccSeek(key, true)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return v, nil
}

func (s *dbSnapshot) BatchGet(keys []kv.Key) (map[string][]byte, error) {
	m := make(map[string][]byte)
	for _, k := range keys {
		v, err := s.Get(k)
		if err != nil && !kv.IsErrNotFound(err) {
			return nil, errors.Trace(err)
		}
		if len(v) > 0 {
			m[string(k)] = v
		}
	}
	return m, nil
}

func (s *dbSnapshot) Seek(k kv.Key) (kv.Iterator, error) {
	it, err := newDBIter(s, k)
	return it, errors.Trace(err)
}

func (s *dbSnapshot) Release() {
}

type dbIter struct {
	s     *dbSnapshot
	valid bool
	k     kv.Key
	v     []byte
}

func newDBIter(s *dbSnapshot, startKey kv.Key) (*dbIter, error) {
	k, v, err := s.mvccSeek(startKey, false)
	if err != nil {
		if terror.ErrorEqual(err, kv.ErrNotExist) {
			err = nil
		}
		return &dbIter{valid: false}, errors.Trace(err)
	}

	return &dbIter{
		s:     s,
		valid: true,
		k:     k,
		v:     v,
	}, nil
}

func (it *dbIter) Next() error {
	k, v, err := it.s.mvccSeek(it.k.Next(), false)
	if err != nil {
		it.valid = false
		if !terror.ErrorEqual(err, kv.ErrNotExist) {
			return errors.Trace(err)
		}
	}
	it.k, it.v = k, v
	return nil
}

func (it *dbIter) Valid() bool {
	return it.valid
}

func (it *dbIter) Key() kv.Key {
	return it.k
}

func (it *dbIter) Value() []byte {
	return it.v
}

func (it *dbIter) Close() {}
