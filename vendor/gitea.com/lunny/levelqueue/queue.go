// Copyright 2019 Lunny Xiao. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package levelqueue

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

const (
	lowKeyStr  = "low"
	highKeyStr = "high"
)

// Queue defines a queue struct
type Queue struct {
	db                *leveldb.DB
	highLock          sync.Mutex
	lowLock           sync.Mutex
	low               int64
	high              int64
	lowKey            []byte
	highKey           []byte
	prefix            []byte
	closeUnderlyingDB bool
}

// Open opens a queue from the db path or creates a
// queue if it doesn't exist.
// The keys will not be prefixed by default
func Open(dataDir string) (*Queue, error) {
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
	return NewQueue(db, []byte{}, true)
}

// NewQueue creates a queue from a db. The keys will be prefixed with prefix
// and at close the db will be closed as per closeUnderlyingDB
func NewQueue(db *leveldb.DB, prefix []byte, closeUnderlyingDB bool) (*Queue, error) {
	var err error

	var queue = &Queue{
		db:                db,
		closeUnderlyingDB: closeUnderlyingDB,
	}

	queue.prefix = make([]byte, len(prefix))
	copy(queue.prefix, prefix)
	queue.lowKey = withPrefix(prefix, []byte(lowKeyStr))
	queue.highKey = withPrefix(prefix, []byte(highKeyStr))

	queue.low, err = queue.readID(queue.lowKey)
	if err == leveldb.ErrNotFound {
		queue.low = 1
		err = db.Put(queue.lowKey, id2bytes(1), nil)
	}
	if err != nil {
		return nil, err
	}

	queue.high, err = queue.readID(queue.highKey)
	if err == leveldb.ErrNotFound {
		err = db.Put(queue.highKey, id2bytes(0), nil)
	}
	if err != nil {
		return nil, err
	}

	return queue, nil
}

func (queue *Queue) readID(key []byte) (int64, error) {
	bs, err := queue.db.Get(key, nil)
	if err != nil {
		return 0, err
	}
	return bytes2id(bs)
}

func (queue *Queue) highincrement() (int64, error) {
	id := queue.high + 1
	queue.high = id
	err := queue.db.Put(queue.highKey, id2bytes(queue.high), nil)
	if err != nil {
		queue.high = queue.high - 1
		return 0, err
	}
	return id, nil
}

func (queue *Queue) highdecrement() (int64, error) {
	queue.high = queue.high - 1
	err := queue.db.Put(queue.highKey, id2bytes(queue.high), nil)
	if err != nil {
		queue.high = queue.high + 1
		return 0, err
	}
	return queue.high, nil
}

func (queue *Queue) lowincrement() (int64, error) {
	queue.low = queue.low + 1
	err := queue.db.Put(queue.lowKey, id2bytes(queue.low), nil)
	if err != nil {
		queue.low = queue.low - 1
		return 0, err
	}
	return queue.low, nil
}

func (queue *Queue) lowdecrement() (int64, error) {
	queue.low = queue.low - 1
	err := queue.db.Put(queue.lowKey, id2bytes(queue.low), nil)
	if err != nil {
		queue.low = queue.low + 1
		return 0, err
	}
	return queue.low, nil
}

// Len returns the length of the queue
func (queue *Queue) Len() int64 {
	queue.lowLock.Lock()
	queue.highLock.Lock()
	l := queue.high - queue.low + 1
	queue.highLock.Unlock()
	queue.lowLock.Unlock()
	return l
}

func id2bytes(id int64) []byte {
	var buf = make([]byte, 8)
	binary.PutVarint(buf, id)
	return buf
}

func bytes2id(b []byte) (int64, error) {
	return binary.ReadVarint(bytes.NewReader(b))
}

func withPrefix(prefix []byte, value []byte) []byte {
	if len(prefix) == 0 {
		return value
	}
	prefixed := make([]byte, len(prefix)+1+len(value))
	copy(prefixed[0:len(prefix)], prefix)
	prefixed[len(prefix)] = '-'
	copy(prefixed[len(prefix)+1:], value)
	return prefixed
}

// RPush pushes a data from right of queue
func (queue *Queue) RPush(data []byte) error {
	queue.highLock.Lock()
	id, err := queue.highincrement()
	if err != nil {
		queue.highLock.Unlock()
		return err
	}
	err = queue.db.Put(withPrefix(queue.prefix, id2bytes(id)), data, nil)
	queue.highLock.Unlock()
	return err
}

// LPush pushes a data from left of queue
func (queue *Queue) LPush(data []byte) error {
	queue.lowLock.Lock()
	id, err := queue.lowdecrement()
	if err != nil {
		queue.lowLock.Unlock()
		return err
	}
	err = queue.db.Put(withPrefix(queue.prefix, id2bytes(id)), data, nil)
	queue.lowLock.Unlock()
	return err
}

// RPop pop a data from right of queue
func (queue *Queue) RPop() ([]byte, error) {
	queue.highLock.Lock()
	defer queue.highLock.Unlock()
	currentID := queue.high

	res, err := queue.db.Get(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	_, err = queue.highdecrement()
	if err != nil {
		return nil, err
	}

	err = queue.db.Delete(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// RHandle receives a user callback function to handle the right element of the queue, if function return nil, then delete the element, otherwise keep the element.
func (queue *Queue) RHandle(h func([]byte) error) error {
	queue.highLock.Lock()
	defer queue.highLock.Unlock()
	currentID := queue.high

	res, err := queue.db.Get(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	if err = h(res); err != nil {
		return err
	}

	_, err = queue.highdecrement()
	if err != nil {
		return err
	}

	return queue.db.Delete(withPrefix(queue.prefix, id2bytes(currentID)), nil)
}

// LPop pop a data from left of queue
func (queue *Queue) LPop() ([]byte, error) {
	queue.lowLock.Lock()
	defer queue.lowLock.Unlock()
	currentID := queue.low

	res, err := queue.db.Get(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	_, err = queue.lowincrement()
	if err != nil {
		return nil, err
	}

	err = queue.db.Delete(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// LHandle receives a user callback function to handle the left element of the queue, if function return nil, then delete the element, otherwise keep the element.
func (queue *Queue) LHandle(h func([]byte) error) error {
	queue.lowLock.Lock()
	defer queue.lowLock.Unlock()
	currentID := queue.low

	res, err := queue.db.Get(withPrefix(queue.prefix, id2bytes(currentID)), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	if err = h(res); err != nil {
		return err
	}

	_, err = queue.lowincrement()
	if err != nil {
		return err
	}

	return queue.db.Delete(withPrefix(queue.prefix, id2bytes(currentID)), nil)
}

// Close closes the queue (and the underlying db is set to closeUnderlyingDB)
func (queue *Queue) Close() error {
	queue.highLock.Lock()
	queue.lowLock.Lock()
	defer queue.highLock.Unlock()
	defer queue.lowLock.Unlock()

	if !queue.closeUnderlyingDB {
		queue.db = nil
		return nil
	}
	err := queue.db.Close()
	queue.db = nil
	return err
}
