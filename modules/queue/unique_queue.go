// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"fmt"
)

// UniqueQueue defines a queue which guarantees only one instance of same
// data is in the queue. Instances with same identity will be
// discarded if there is already one in the line.
//
// This queue is particularly useful for preventing duplicated task
// of same purpose - please note that this does not guarantee that a particular
// task cannot be processed twice or more at the same time. Uniqueness is
// only guaranteed whilst the task is waiting in the queue.
//
// Users of this queue should be careful to push only the identifier of the
// data
type UniqueQueue interface {
	Queue
	PushFunc(Data, func() error) error
	Has(Data) (bool, error)
}

// ErrAlreadyInQueue is returned when trying to push data to the queue that is already in the queue
var ErrAlreadyInQueue = fmt.Errorf("already in queue")
