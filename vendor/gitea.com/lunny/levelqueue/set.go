// Copyright 2020 Andrew Thornton. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package levelqueue

import (
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const (
	setPrefixStr = "set"
)

// Set defines a set struct
type Set struct {
	db                *leveldb.DB
	closeUnderlyingDB bool
	lock              sync.Mutex
	prefix            []byte
}

// OpenSet opens a set from the db path or creates a set if it doesn't exist.
// The keys will be prefixed with "set-" by default
func OpenSet(dataDir string) (*Set, error) {
	db, err := leveldb.OpenFile(dataDir, nil)
	if err != nil {
		if !errors.IsCorrupted(err) {
			return nil, err
		}
		db, err = leveldb.RecoverFile(dataDir, nil)
		if err != nil {
			return nil, err
		}
	}
	return NewSet(db, []byte(setPrefixStr), true)
}

// NewSet creates a set from a db. The keys will be prefixed with prefix
// and at close the db will be closed as per closeUnderlyingDB
func NewSet(db *leveldb.DB, prefix []byte, closeUnderlyingDB bool) (*Set, error) {
	set := &Set{
		db:                db,
		closeUnderlyingDB: closeUnderlyingDB,
	}
	set.prefix = make([]byte, len(prefix))
	copy(set.prefix, prefix)

	return set, nil
}

// Add adds a member string to a key set, returns true if the member was not already present
func (set *Set) Add(value []byte) (bool, error) {
	set.lock.Lock()
	defer set.lock.Unlock()
	setKey := withPrefix(set.prefix, value)
	has, err := set.db.Has(setKey, nil)
	if err != nil || has {
		return !has, err
	}
	return !has, set.db.Put(setKey, []byte(""), nil)
}

// Members returns the current members of the set
func (set *Set) Members() ([][]byte, error) {
	set.lock.Lock()
	defer set.lock.Unlock()
	var members [][]byte
	prefix := withPrefix(set.prefix, []byte{})
	iter := set.db.NewIterator(util.BytesPrefix(prefix), nil)
	for iter.Next() {
		slice := iter.Key()[len(prefix):]
		value := make([]byte, len(slice))
		copy(value, slice)
		members = append(members, value)
	}
	iter.Release()
	return members, iter.Error()
}

// Has returns if the member is in the set
func (set *Set) Has(value []byte) (bool, error) {
	set.lock.Lock()
	defer set.lock.Unlock()
	setKey := withPrefix(set.prefix, value)

	return set.db.Has(setKey, nil)
}

// Remove removes a member from the set, returns true if the member was present
func (set *Set) Remove(value []byte) (bool, error) {
	set.lock.Lock()
	defer set.lock.Unlock()
	setKey := withPrefix(set.prefix, value)

	has, err := set.db.Has(setKey, nil)
	if err != nil || !has {
		return has, err
	}

	return has, set.db.Delete(setKey, nil)
}

// Close closes the set (and the underlying db if set to closeUnderlyingDB)
func (set *Set) Close() error {
	set.lock.Lock()
	defer set.lock.Unlock()
	if !set.closeUnderlyingDB {
		set.db = nil
		return nil
	}
	err := set.db.Close()
	set.db = nil
	return err
}
