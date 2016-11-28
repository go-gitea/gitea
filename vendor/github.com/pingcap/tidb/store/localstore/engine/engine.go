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

package engine

import "github.com/juju/errors"

// ErrNotFound indicates no key is found when trying Get or Seek an entry from DB.
var ErrNotFound = errors.New("local engine: key not found")

// Driver is the interface that must be implemented by a local storage db engine.
type Driver interface {
	// Open opens or creates a local storage DB.
	// The schema is a string for a local storage DB specific format.
	Open(schema string) (DB, error)
}

// MSeekResult is used to get multiple seek results.
type MSeekResult struct {
	Key   []byte
	Value []byte
	Err   error
}

// DB is the interface for local storage.
type DB interface {
	// Get gets the associated value with key, returns (nil, ErrNotFound) if no value found.
	Get(key []byte) ([]byte, error)
	// Seek searches for the first key in the engine which is >= key in byte order, returns (nil, nil, ErrNotFound)
	// if such key is not found.
	Seek(key []byte) ([]byte, []byte, error)
	// MultiSeek seeks multiple keys from the engine.
	MultiSeek(keys [][]byte) []*MSeekResult
	// NewBatch creates a Batch for writing.
	NewBatch() Batch
	// Commit writes the changed data in Batch.
	Commit(b Batch) error
	// Close closes database.
	Close() error
}

// Batch is the interface for local storage.
type Batch interface {
	// Put appends 'put operation' of the key/value to the batch.
	Put(key []byte, value []byte)
	// Delete appends 'delete operation' of the key/value to the batch.
	Delete(key []byte)
	// Len return length of the batch
	Len() int
}
