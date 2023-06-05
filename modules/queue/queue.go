// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package queue implements a specialized queue system for Gitea.
//
// There are two major kinds of concepts:
//
// * The "base queue": channel, level, redis:
//   - They have the same abstraction, the same interface, and they are tested by the same testing code.
//   - The dummy(immediate) queue is special, it's not a real queue, it's only used as a no-op queue or a testing queue.
//
// * The WorkerPoolQueue: it uses the "base queue" to provide "worker pool" function.
//   - It calls the "handler" to process the data in the base queue.
//   - Its "Push" function doesn't block forever,
//     it will return an error if the queue is full after the timeout.
//
// A queue can be "simple" or "unique". A unique queue will try to avoid duplicate items.
// Unique queue's "Has" function can be used to check whether an item is already in the queue,
// although it's not 100% reliable due to there is no proper transaction support.
// Simple queue's "Has" function always returns "has=false".
//
// The HandlerFuncT function is called by the WorkerPoolQueue to process the data in the base queue.
// If the handler returns "unhandled" items, they will be re-queued to the base queue after a slight delay,
// in case the item processor (eg: document indexer) is not available.
package queue

import "code.gitea.io/gitea/modules/util"

type HandlerFuncT[T any] func(...T) (unhandled []T)

var ErrAlreadyInQueue = util.NewAlreadyExistErrorf("already in queue")
