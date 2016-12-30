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

package kv

import (
	"github.com/juju/errors"
)

// BufferStore wraps a Retriever for read and a MemBuffer for buffered write.
// Common usage pattern:
//	bs := NewBufferStore(r) // use BufferStore to wrap a Retriever
//	defer bs.Release()      // make sure it will be released
//	// ...
//	// read/write on bs
//	// ...
//	bs.SaveTo(m)	        // save above operations to a Mutator
type BufferStore struct {
	MemBuffer
	r Retriever
}

// NewBufferStore creates a BufferStore using r for read.
func NewBufferStore(r Retriever) *BufferStore {
	return &BufferStore{
		r:         r,
		MemBuffer: &lazyMemBuffer{},
	}
}

// Get implements the Retriever interface.
func (s *BufferStore) Get(k Key) ([]byte, error) {
	val, err := s.MemBuffer.Get(k)
	if IsErrNotFound(err) {
		val, err = s.r.Get(k)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(val) == 0 {
		return nil, errors.Trace(ErrNotExist)
	}
	return val, nil
}

// Seek implements the Retriever interface.
func (s *BufferStore) Seek(k Key) (Iterator, error) {
	bufferIt, err := s.MemBuffer.Seek(k)
	if err != nil {
		return nil, errors.Trace(err)
	}
	retrieverIt, err := s.r.Seek(k)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return newUnionIter(bufferIt, retrieverIt), nil
}

// WalkBuffer iterates all buffered kv pairs.
func (s *BufferStore) WalkBuffer(f func(k Key, v []byte) error) error {
	iter, err := s.MemBuffer.Seek(nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		if err := f(iter.Key(), iter.Value()); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// SaveTo saves all buffered kv pairs into a Mutator.
func (s *BufferStore) SaveTo(m Mutator) error {
	err := s.WalkBuffer(func(k Key, v []byte) error {
		if len(v) == 0 {
			return errors.Trace(m.Delete(k))
		}
		return errors.Trace(m.Set(k, v))
	})
	return errors.Trace(err)
}
