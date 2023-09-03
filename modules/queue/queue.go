// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package queue implements a specialized concurrent queue system for Gitea.
//
// Terminology:
//
//  1. Task:
//     -	A task can be a simple value, such as an integer, or a more complex structure that has multiple fields and
//     methods. The aim of a task is to be a unit of work, a set of tasks will be sent to a handler to be processed.
//
//  2. Batch:
//     - A collection of tasks that are grouped together for processing. Each worker receives a batch of tasks.
//
//  3. Worker:
//     - Individual unit of execution designed to process tasks from the queue. It's a goroutine that calls the Handler
//     - Workers will get new tasks through a channel (WorkerPoolQueue is responsible for the distribution)
//     - As workers operate in parallel, the default value of max workers is n/2, where n is the number of logical CPUs
//
//  4. Handler (represented by HandlerFuncT type):
//     - It's the function responsible to process tasks. Each active worker will call this.
//     - When processing these batches, there might be instances where certain tasks remain unprocessed or "unhandled".
//     In such scenarios, the Handler ensures these unhandled tasks are returned to the base queue after a brief delay.
//     This mechanism is particularly beneficial in cases where the processing entity (like a document indexer) is
//     temporarily unavailable. It ensures that no task is skipped or lost due to transient failures in the processing
//     mechanism.
//
//  5. Base queue:
//     - Represents the underlying storage mechanism for the queue. There are several implementations:
//     - Channel: Uses Go's native channel constructs to manage the queue, suitable for in-memory queuing.
//     - Level, Redis: Especially useful in persistent queues and clusters, where we may have multiple nodes.
//     - Dummy: This is special, it's not a real queue, it's only used as a no-op queue or a testing queue.
//     - They all have the same abstraction, the same interface, and they are tested by the same testing code.
//
// 6. WorkerPoolQueue:
//   - It's responsible to glue all together, using the "base queue" to provide "worker pool" functionality. He creates
//     new workers if needed and can flush the queue, running all the tasks synchronously till it finishes.
//   - Its "Push" function doesn't block forever, it will return an error if the queue is full after the timeout.
//
// 7. Manager:
//   - The purpose of it is to serve as a centralized manager for multiple WorkerPoolQueue instances. Whenever we want
//     to create a new queue, flush, or get a specific queue, we have to use it.
//
// A queue can be "simple" or "unique". A unique queue will try to avoid duplicate items.
// Unique queue's "Has" function can be used to check whether an item is already in the queue,
// although it's not 100% reliable due to there is no proper transaction support.
// Simple queue's "Has" function always returns "has=false".
//
// A WorkerPoolQueue is a generic struct; this means it will work with any type but just for that type.
// If you want another kind of tasks to run, you would have to call the manager to create a new WorkerPoolQueue for you
// with a different handler that works with this new type of task. As an example of this:
//
//	func Init() error {
//		taskQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "task", handler)
//		...
//	}
//	func handler(items ...*admin_model.Task) []*admin_model.Task { ... }
//
// As you can see, the handler defined the admin_model.Task type for the queue
package queue

import "code.gitea.io/gitea/modules/util"

type HandlerFuncT[T any] func(...T) (unhandled []T)

var ErrAlreadyInQueue = util.NewAlreadyExistErrorf("already in queue")
