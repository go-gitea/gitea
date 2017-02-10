// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"github.com/Unknwon/com"
)

// UniqueQueue is a queue which guarantees only one instance of same
// identity is in the line. Instances with same identity will be
// discarded if there is already one in the line.
//
// This queue is particularly useful for preventing duplicated task
// of same purpose.
type UniqueQueue struct {
	table *StatusTable
	queue chan string
}

// NewUniqueQueue initializes and returns a new UniqueQueue object.
func NewUniqueQueue(queueLength int) *UniqueQueue {
	if queueLength <= 0 {
		queueLength = 100
	}

	return &UniqueQueue{
		table: NewStatusTable(),
		queue: make(chan string, queueLength),
	}
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
	if q.Exist(id) {
		return
	}

	idStr := com.ToStr(id)
	q.table.lock.Lock()
	q.table.pool[idStr] = struct{}{}
	if fn != nil {
		fn()
	}
	q.table.lock.Unlock()
	q.queue <- idStr
}

// Add adds new instance to the queue.
func (q *UniqueQueue) Add(id interface{}) {
	q.AddFunc(id, nil)
}

// Remove removes instance from the queue.
func (q *UniqueQueue) Remove(id interface{}) {
	q.table.Stop(com.ToStr(id))
}
