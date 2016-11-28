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

import "io"

const (
	// PresumeKeyNotExists directives that when dealing with a Get operation but failing to read data from cache,
	// we presume that the key does not exist in Store. The actual existence will be checked before the
	// transaction's commit.
	// This option is an optimization for frequent checks during a transaction, e.g. batch inserts.
	PresumeKeyNotExists Option = iota + 1
	// PresumeKeyNotExistsError is the option key for error.
	// When PresumeKeyNotExists is set and condition is not match, should throw the error.
	PresumeKeyNotExistsError
)

// Retriever is the interface wraps the basic Get and Seek methods.
type Retriever interface {
	// Get gets the value for key k from kv store.
	// If corresponding kv pair does not exist, it returns nil and ErrNotExist.
	Get(k Key) ([]byte, error)
	// Seek creates an Iterator positioned on the first entry that k <= entry's key.
	// If such entry is not found, it returns an invalid Iterator with no error.
	// The Iterator must be Closed after use.
	Seek(k Key) (Iterator, error)
}

// Mutator is the interface wraps the basic Set and Delete methods.
type Mutator interface {
	// Set sets the value for key k as v into kv store.
	// v must NOT be nil or empty, otherwise it returns ErrCannotSetNilValue.
	Set(k Key, v []byte) error
	// Delete removes the entry for key k from kv store.
	Delete(k Key) error
}

// RetrieverMutator is the interface that groups Retriever and Mutator interfaces.
type RetrieverMutator interface {
	Retriever
	Mutator
}

// MemBuffer is an in-memory kv collection. It should be released after use.
type MemBuffer interface {
	RetrieverMutator
	// Release releases the buffer.
	Release()
}

// Transaction defines the interface for operations inside a Transaction.
// This is not thread safe.
type Transaction interface {
	RetrieverMutator
	// Commit commits the transaction operations to KV store.
	Commit() error
	// Rollback undoes the transaction operations to KV store.
	Rollback() error
	// String implements fmt.Stringer interface.
	String() string
	// LockKeys tries to lock the entries with the keys in KV store.
	LockKeys(keys ...Key) error
	// SetOption sets an option with a value, when val is nil, uses the default
	// value of this option.
	SetOption(opt Option, val interface{})
	// DelOption deletes an option.
	DelOption(opt Option)
	// IsReadOnly checks if the transaction has only performed read operations.
	IsReadOnly() bool
	// GetClient gets a client instance.
	GetClient() Client
	// StartTS returns the transaction start timestamp.
	StartTS() int64
}

// Client is used to send request to KV layer.
type Client interface {
	// Send sends request to KV layer, returns a Response.
	Send(req *Request) Response

	// SupportRequestType checks if reqType and subType is supported.
	SupportRequestType(reqType, subType int64) bool
}

// ReqTypes.
const (
	ReqTypeSelect = 101
	ReqTypeIndex  = 102
)

// KeyRange represents a range where StartKey <= key < EndKey.
type KeyRange struct {
	StartKey Key
	EndKey   Key
}

// Request represents a kv request.
type Request struct {
	// The request type.
	Tp   int64
	Data []byte
	// Key Ranges
	KeyRanges []KeyRange
	// If desc is true, the request is sent in descending order.
	Desc bool
	// If concurrency is 1, it only sends the request to a single storage unit when
	// ResponseIterator.Next is called. If concurrency is greater than 1, the request will be
	// sent to multiple storage units concurrently.
	Concurrency int
}

// Response represents the response returned from KV layer.
type Response interface {
	// Next returns a resultSubset from a single storage unit.
	// When full result set is returned, nil is returned.
	Next() (resultSubset io.ReadCloser, err error)
}

// Snapshot defines the interface for the snapshot fetched from KV store.
type Snapshot interface {
	Retriever
	// BatchGet gets a batch of values from snapshot.
	BatchGet(keys []Key) (map[string][]byte, error)
	// Release releases the snapshot to store.
	Release()
}

// Driver is the interface that must be implemented by a KV storage.
type Driver interface {
	// Open returns a new Storage.
	// The path is the string for storage specific format.
	Open(path string) (Storage, error)
}

// Storage defines the interface for storage.
// Isolation should be at least SI(SNAPSHOT ISOLATION)
type Storage interface {
	// Begin transaction
	Begin() (Transaction, error)
	// GetSnapshot gets a snapshot that is able to read any data which data is <= ver.
	// if ver is MaxVersion or > current max committed version, we will use current version for this snapshot.
	GetSnapshot(ver Version) (Snapshot, error)
	// Close store
	Close() error
	// Storage's unique ID
	UUID() string
	// CurrentVersion returns current max committed version.
	CurrentVersion() (Version, error)
}

// FnKeyCmp is the function for iterator the keys
type FnKeyCmp func(key Key) bool

// Iterator is the interface for a iterator on KV store.
type Iterator interface {
	Valid() bool
	Key() Key
	Value() []byte
	Next() error
	Close()
}
