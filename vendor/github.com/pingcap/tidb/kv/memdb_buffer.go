// Copyright 2015 PingCAP, Inc.
//
// Copyright 2015 Wenbin Xiao
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

package kv

import (
	"github.com/juju/errors"
	"github.com/pingcap/tidb/terror"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/memdb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type memDbBuffer struct {
	db *memdb.DB
}

type memDbIter struct {
	iter iterator.Iterator
}

// NewMemDbBuffer creates a new memDbBuffer.
func NewMemDbBuffer() MemBuffer {
	return &memDbBuffer{db: memdb.New(comparer.DefaultComparer, 4*1024)}
}

// Seek creates an Iterator.
func (m *memDbBuffer) Seek(k Key) (Iterator, error) {
	var i Iterator
	if k == nil {
		i = &memDbIter{iter: m.db.NewIterator(&util.Range{})}
	} else {
		i = &memDbIter{iter: m.db.NewIterator(&util.Range{Start: []byte(k)})}
	}
	i.Next()
	return i, nil
}

// Get returns the value associated with key.
func (m *memDbBuffer) Get(k Key) ([]byte, error) {
	v, err := m.db.Get(k)
	if terror.ErrorEqual(err, leveldb.ErrNotFound) {
		return nil, ErrNotExist
	}
	return v, nil
}

// Set associates key with value.
func (m *memDbBuffer) Set(k Key, v []byte) error {
	if len(v) == 0 {
		return errors.Trace(ErrCannotSetNilValue)
	}
	err := m.db.Put(k, v)
	return errors.Trace(err)
}

// Delete removes the entry from buffer with provided key.
func (m *memDbBuffer) Delete(k Key) error {
	err := m.db.Put(k, nil)
	return errors.Trace(err)
}

// Release reset the buffer.
func (m *memDbBuffer) Release() {
	m.db.Reset()
}

// Next implements the Iterator Next.
func (i *memDbIter) Next() error {
	i.iter.Next()
	return nil
}

// Valid implements the Iterator Valid.
func (i *memDbIter) Valid() bool {
	return i.iter.Valid()
}

// Key implements the Iterator Key.
func (i *memDbIter) Key() Key {
	return i.iter.Key()
}

// Value implements the Iterator Value.
func (i *memDbIter) Value() []byte {
	return i.iter.Value()
}

// Close Implements the Iterator Close.
func (i *memDbIter) Close() {
	i.iter.Release()
}
