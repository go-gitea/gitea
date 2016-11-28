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

package goleveldb

import (
	"sync"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	_ engine.DB    = (*db)(nil)
	_ engine.Batch = (*leveldb.Batch)(nil)
)

var (
	p = sync.Pool{
		New: func() interface{} {
			return &leveldb.Batch{}
		},
	}
)

type db struct {
	*leveldb.DB
}

func (d *db) Get(key []byte) ([]byte, error) {
	v, err := d.DB.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, errors.Trace(engine.ErrNotFound)
	}
	return v, err
}

func (d *db) NewBatch() engine.Batch {
	b := p.Get().(*leveldb.Batch)
	return b
}

func (d *db) Seek(startKey []byte) ([]byte, []byte, error) {
	iter := d.DB.NewIterator(&util.Range{Start: startKey}, nil)
	defer iter.Release()
	if ok := iter.First(); !ok {
		return nil, nil, errors.Trace(engine.ErrNotFound)
	}
	return iter.Key(), iter.Value(), nil
}

func (d *db) MultiSeek(keys [][]byte) []*engine.MSeekResult {
	iter := d.DB.NewIterator(&util.Range{Start: []byte{0x0}}, nil)
	defer iter.Release()

	res := make([]*engine.MSeekResult, 0, len(keys))
	for _, k := range keys {
		if ok := iter.Seek(k); !ok {
			res = append(res, &engine.MSeekResult{Err: engine.ErrNotFound})
		} else {
			res = append(res, &engine.MSeekResult{
				Key:   append([]byte(nil), iter.Key()...),
				Value: append([]byte(nil), iter.Value()...),
			})
		}
	}
	return res
}

func (d *db) Commit(b engine.Batch) error {
	batch, ok := b.(*leveldb.Batch)
	if !ok {
		return errors.Errorf("invalid batch type %T", b)
	}
	err := d.DB.Write(batch, nil)
	batch.Reset()
	p.Put(batch)
	return err
}

func (d *db) Close() error {
	return d.DB.Close()
}

// Driver implements engine Driver.
type Driver struct {
}

// Open opens or creates a local storage database for the given path.
func (driver Driver) Open(path string) (engine.DB, error) {
	d, err := leveldb.OpenFile(path, &opt.Options{BlockCacheCapacity: 600 * 1024 * 1024})

	return &db{d}, err
}

// MemoryDriver implements engine Driver
type MemoryDriver struct {
}

// Open opens a memory storage database.
func (driver MemoryDriver) Open(path string) (engine.DB, error) {
	d, err := leveldb.Open(storage.NewMemStorage(), nil)
	return &db{d}, err
}
