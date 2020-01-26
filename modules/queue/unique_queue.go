// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
)

// UniqueQueue defines a queue which guarantees only one instance of same
// data is in the queue. Instances with same identity will be
// discarded if there is already one in the line.
//
// This queue is particularly useful for preventing duplicated task
// of same purpose.
//
// Users of this queue should be careful to push only the identifier of the
// data
type UniqueQueue interface {
	Run(atShutdown, atTerminate func(context.Context, func()))
	Push(Data) error
	PushFunc(Data, func() error) error
	Has(Data) (bool, error)
}

// ErrAlreadyInQueue is returned when trying to push data to the queue that is already in the queue
var ErrAlreadyInQueue = fmt.Errorf("already in queue")
