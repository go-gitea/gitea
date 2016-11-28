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

package boltdb

import (
	"os"
	"path"

	"github.com/boltdb/bolt"
	"github.com/juju/errors"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/pingcap/tidb/util/bytes"
)

var (
	_ engine.DB = (*db)(nil)
)

var (
	bucketName = []byte("tidb")
)

type db struct {
	*bolt.DB
}

func (d *db) Get(key []byte) ([]byte, error) {
	var value []byte

	err := d.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		v := b.Get(key)
		if v == nil {
			return errors.Trace(engine.ErrNotFound)
		}
		value = bytes.CloneBytes(v)
		return nil
	})

	return value, errors.Trace(err)
}

func (d *db) MultiSeek(keys [][]byte) []*engine.MSeekResult {
	res := make([]*engine.MSeekResult, 0, len(keys))
	d.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()
		for _, key := range keys {
			var k, v []byte
			if key == nil {
				k, v = c.First()
			} else {
				k, v = c.Seek(key)
			}

			r := &engine.MSeekResult{}
			if k == nil {
				r.Err = engine.ErrNotFound
			} else {
				r.Key, r.Value, r.Err = bytes.CloneBytes(k), bytes.CloneBytes(v), nil
			}

			res = append(res, r)
		}
		return nil
	})

	return res
}

func (d *db) Seek(startKey []byte) ([]byte, []byte, error) {
	var key, value []byte
	err := d.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()
		var k, v []byte
		if startKey == nil {
			k, v = c.First()
		} else {
			k, v = c.Seek(startKey)
		}
		if k != nil {
			key, value = bytes.CloneBytes(k), bytes.CloneBytes(v)
		}
		return nil
	})

	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	if key == nil {
		return nil, nil, errors.Trace(engine.ErrNotFound)
	}
	return key, value, nil
}

func (d *db) NewBatch() engine.Batch {
	return &batch{}
}

func (d *db) Commit(b engine.Batch) error {
	bt, ok := b.(*batch)
	if !ok {
		return errors.Errorf("invalid batch type %T", b)
	}
	err := d.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		// err1 is used for passing `go tool vet --shadow` check.
		var err1 error
		for _, w := range bt.writes {
			if !w.isDelete {
				err1 = b.Put(w.key, w.value)
			} else {
				err1 = b.Delete(w.key)
			}

			if err1 != nil {
				return errors.Trace(err1)
			}
		}

		return nil
	})
	return errors.Trace(err)
}

func (d *db) Close() error {
	return d.DB.Close()
}

type write struct {
	key      []byte
	value    []byte
	isDelete bool
}

type batch struct {
	writes []write
}

func (b *batch) Put(key []byte, value []byte) {
	w := write{
		key:   append([]byte(nil), key...),
		value: append([]byte(nil), value...),
	}
	b.writes = append(b.writes, w)
}

func (b *batch) Delete(key []byte) {
	w := write{
		key:      append([]byte(nil), key...),
		value:    nil,
		isDelete: true,
	}
	b.writes = append(b.writes, w)
}

func (b *batch) Len() int {
	return len(b.writes)
}

// Driver implements engine Driver.
type Driver struct {
}

// Open opens or creates a local storage database with given path.
func (driver Driver) Open(dbPath string) (engine.DB, error) {
	base := path.Dir(dbPath)
	os.MkdirAll(base, 0755)

	d, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	tx, err := d.Begin(true)
	if err != nil {
		return nil, err
	}

	if _, err = tx.CreateBucketIfNotExists(bucketName); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &db{d}, nil
}
