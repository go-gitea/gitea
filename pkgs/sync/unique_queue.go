// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"github.com/unknwon/com"
)

// UniqueQueue is a queue which guarantees only one instance of same
// identity is in the line. Instances with same identity will be
// discarded if there is already one in the line.
//
// This queue is particularly useful for preventing duplicated task
// of same purpose.
type UniqueQueue struct {
	table  *StatusTable
	queue  chan string
	closed chan struct{}
}

// NewUniqueQueue initializes and returns a new UniqueQueue object.
func NewUniqueQueue(queueLength int) *UniqueQueue {
	if queueLength <= 0 {
		queueLength = 100
	}

	return &UniqueQueue{
		table:  NewStatusTable(),
		queue:  make(chan string, queueLength),
		closed: make(chan struct{}),
	}
}

// Close closes this queue
func (q *UniqueQueue) Close() {
	select {
	case <-q.closed:
	default:
		q.table.lock.Lock()
		select {
		case <-q.closed:
		default:
			close(q.closed)
		}
		q.table.lock.Unlock()
	}
}

// IsClosed returns a channel that is closed when this Queue is closed
func (q *UniqueQueue) IsClosed() <-chan struct{} {
	return q.closed
}

// IDs returns the current ids in the pool
func (q *UniqueQueue) IDs() []interface{} {
	q.table.lock.Lock()
	defer q.table.lock.Unlock()
	ids := make([]interface{}, 0, len(q.table.pool))
	for id := range q.table.pool {
		ids = append(ids, id)
	}
	return ids
}

// Queue returns channel of queue for retrieving instances.
func (q *UniqueQueue) Queue() <-chan string {
	return q.queue
}

// Exist returns true if there is an instance with given identity
// exists in the queue.
func (q *UniqueQueue) Exist(id interface{}) bool {
	return q.table.IsRunning(com.ToStr(id))
}

// AddFunc adds new instance to the queue with a custom runnable function,
// the queue is blocked until the function exits.
func (q *UniqueQueue) AddFunc(id interface{}, fn func()) {
	idStr := com.ToStr(id)
	q.table.lock.Lock()
	if _, ok := q.table.pool[idStr]; ok {
		q.table.lock.Unlock()
		return
	}
	q.table.pool[idStr] = struct{}{}
	if fn != nil {
		fn()
	}
	q.table.lock.Unlock()
	select {
	case <-q.closed:
		return
	case q.queue <- idStr:
		return
	}
}

// Add adds new instance to the queue.
func (q *UniqueQueue) Add(id interface{}) {
	q.AddFunc(id, nil)
}

// Remove removes instance from the queue.
func (q *UniqueQueue) Remove(id interface{}) {
	q.table.Stop(com.ToStr(id))
}
