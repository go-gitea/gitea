// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caches

import (
	"log"

	"github.com/syndtr/goleveldb/leveldb"
)

// LevelDBStore implements CacheStore provide local machine
type LevelDBStore struct {
	store *leveldb.DB
	Debug bool
	v     interface{}
}

var _ CacheStore = &LevelDBStore{}

// NewLevelDBStore creates a leveldb store
func NewLevelDBStore(dbfile string) (*LevelDBStore, error) {
	db := &LevelDBStore{}
	h, err := leveldb.OpenFile(dbfile, nil)
	if err != nil {
		return nil, err
	}
	db.store = h
	return db, nil
}

// Put implements CacheStore
func (s *LevelDBStore) Put(key string, value interface{}) error {
	val, err := Encode(value)
	if err != nil {
		if s.Debug {
			log.Println("[LevelDB]EncodeErr: ", err, "Key:", key)
		}
		return err
	}
	err = s.store.Put([]byte(key), val, nil)
	if err != nil {
		if s.Debug {
			log.Println("[LevelDB]PutErr: ", err, "Key:", key)
		}
		return err
	}
	if s.Debug {
		log.Println("[LevelDB]Put: ", key)
	}
	return err
}

// Get implements CacheStore
func (s *LevelDBStore) Get(key string) (interface{}, error) {
	data, err := s.store.Get([]byte(key), nil)
	if err != nil {
		if s.Debug {
			log.Println("[LevelDB]GetErr: ", err, "Key:", key)
		}
		if err == leveldb.ErrNotFound {
			return nil, ErrNotExist
		}
		return nil, err
	}

	err = Decode(data, &s.v)
	if err != nil {
		if s.Debug {
			log.Println("[LevelDB]DecodeErr: ", err, "Key:", key)
		}
		return nil, err
	}
	if s.Debug {
		log.Println("[LevelDB]Get: ", key, s.v)
	}
	return s.v, err
}

// Del implements CacheStore
func (s *LevelDBStore) Del(key string) error {
	err := s.store.Delete([]byte(key), nil)
	if err != nil {
		if s.Debug {
			log.Println("[LevelDB]DelErr: ", err, "Key:", key)
		}
		return err
	}
	if s.Debug {
		log.Println("[LevelDB]Del: ", key)
	}
	return err
}

// Close implements CacheStore
func (s *LevelDBStore) Close() {
	s.store.Close()
}
