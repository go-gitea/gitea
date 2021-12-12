// Copyright 2020 Andrew Thornton. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package levelqueue

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

const (
	uniqueQueuePrefixStr = "unique"
)

// UniqueQueue defines an unique queue struct
type UniqueQueue struct {
	q                 *Queue
	set               *Set
	db                *leveldb.DB
	closeUnderlyingDB bool
}

// OpenUnique opens an unique queue from the db path or creates a set if it doesn't exist.
// The keys in the queue portion will not be prefixed, and the set keys will be prefixed with "set-"
func OpenUnique(dataDir string) (*UniqueQueue, error) {
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
	return NewUniqueQueue(db, []byte{}, []byte(uniqueQueuePrefixStr), true)
}

// NewUniqueQueue creates a new unique queue from a db.
// The queue keys will be prefixed with queuePrefix and the set keys with setPrefix
// and at close the db will be closed as per closeUnderlyingDB
func NewUniqueQueue(db *leveldb.DB, queuePrefix []byte, setPrefix []byte, closeUnderlyingDB bool) (*UniqueQueue, error) {
	internal, err := NewQueue(db, queuePrefix, false)
	if err != nil {
		return nil, err
	}
	set, err := NewSet(db, setPrefix, false)
	if err != nil {
		return nil, err
	}
	queue := &UniqueQueue{
		q:                 internal,
		set:               set,
		db:                db,
		closeUnderlyingDB: closeUnderlyingDB,
	}

	return queue, err
}

// LPush pushes data to the left of the queue
func (queue *UniqueQueue) LPush(data []byte) error {
	return queue.LPushFunc(data, nil)
}

// LPushFunc pushes data to the left of the queue and calls the callback if it is added
func (queue *UniqueQueue) LPushFunc(data []byte, fn func() error) error {
	added, err := queue.set.Add(data)
	if err != nil {
		return err
	}
	if !added {
		return ErrAlreadyInQueue
	}

	if fn != nil {
		err = fn()
		if err != nil {
			_, remErr := queue.set.Remove(data)
			if remErr != nil {
				return fmt.Errorf("%v & %v", err, remErr)
			}
			return err
		}
	}

	return queue.q.LPush(data)
}

// RPush pushes data to the right of the queue
func (queue *UniqueQueue) RPush(data []byte) error {
	return queue.RPushFunc(data, nil)
}

// RPushFunc pushes data to the right of the queue and calls the callback if is added
func (queue *UniqueQueue) RPushFunc(data []byte, fn func() error) error {
	added, err := queue.set.Add(data)
	if err != nil {
		return err
	}
	if !added {
		return ErrAlreadyInQueue
	}

	if fn != nil {
		err = fn()
		if err != nil {
			_, remErr := queue.set.Remove(data)
			if remErr != nil {
				return fmt.Errorf("%v & %v", err, remErr)
			}
			return err
		}
	}

	return queue.q.RPush(data)
}

// RPop pop data from the right of the queue
func (queue *UniqueQueue) RPop() ([]byte, error) {
	popped, err := queue.q.RPop()
	if err != nil {
		return popped, err
	}
	_, err = queue.set.Remove(popped)

	return popped, err
}

// RHandle receives a user callback function to handle the right element of the queue, if the function returns nil, then delete the element, otherwise keep the element.
func (queue *UniqueQueue) RHandle(h func([]byte) error) error {
	return queue.q.RHandle(func(data []byte) error {
		err := h(data)
		if err != nil {
			return err
		}
		_, err = queue.set.Remove(data)
		return err
	})
}

// LPop pops data from left of the queue
func (queue *UniqueQueue) LPop() ([]byte, error) {
	popped, err := queue.q.LPop()
	if err != nil {
		return popped, err
	}
	_, err = queue.set.Remove(popped)

	return popped, err
}

// LHandle receives a user callback function to handle the left element of the queue, if the function returns nil, then delete the element, otherwise keep the element.
func (queue *UniqueQueue) LHandle(h func([]byte) error) error {
	return queue.q.LHandle(func(data []byte) error {
		err := h(data)
		if err != nil {
			return err
		}
		_, err = queue.set.Remove(data)
		return err
	})
}

// Has checks whether the data is already in the queue
func (queue *UniqueQueue) Has(data []byte) (bool, error) {
	return queue.set.Has(data)
}

// Len returns the length of the queue
func (queue *UniqueQueue) Len() int64 {
	queue.set.lock.Lock()
	defer queue.set.lock.Unlock()
	return queue.q.Len()
}

// Close closes the queue (and the underlying DB if set to closeUnderlyingDB)
func (queue *UniqueQueue) Close() error {
	_ = queue.q.Close()
	_ = queue.set.Close()
	queue.set.lock.Lock()
	defer queue.set.lock.Unlock()
	if !queue.closeUnderlyingDB {
		queue.db = nil
		return nil
	}
	err := queue.db.Close()
	queue.db = nil
	return err
}
