// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package queue implements a specialized concurrent queue system for Gitea.
//
// Terminology:
//
//  1. Item:
//     - An item can be a simple value, such as an integer, or a more complex structure that has multiple fields.
//     Usually a item serves as a task or a message. Sets of items will be sent to a queue handler to be processed.
//     - It's represented as a JSON-marshaled binary slice in the queue
//     - Since the item is marshaled by JSON, and JSON doesn't have stable key-order/type support,
//     so the decoded handler item may not be the same as the original "pushed" one if you use map/any types,
//
//  2. Batch:
//     - A collection of items that are grouped together for processing. Each worker receives a batch of items.
//
//  3. Worker:
//     - Individual unit of execution designed to process items from the queue. It's a goroutine that calls the Handler.
//     - Workers will get new items through a channel (WorkerPoolQueue is responsible for the distribution).
//     - Workers operate in parallel. The default value of max workers is determined by the setting system.
//
//  4. Handler (represented by HandlerFuncT type):
//     - It's the function responsible for processing items. Each active worker will call it.
//     - If an item or some items are not psuccessfully rocessed, the handler could return them as "unhandled items".
//     In such scenarios, the queue system ensures these unhandled items are returned to the base queue after a brief delay.
//     This mechanism is particularly beneficial in cases where the processing entity (like a document indexer) is
//     temporarily unavailable. It ensures that no item is skipped or lost due to transient failures in the processing
//     mechanism.
//
//  5. Base queue:
//     - Represents the underlying storage mechanism for the queue. There are several implementations:
//     - Channel: Uses Go's native channel constructs to manage the queue, suitable for in-memory queuing.
//     - LevelDB: Especially useful in persistent queues for single instances.
//     - Redis: Suitable for clusters, where we may have multiple nodes.
//     - Dummy: This is special, it's not a real queue, it's a immediate no-op queue, which is useful for tests.
//     - They all have the same abstraction, the same interface, and they are tested by the same testing code.
//
// 6. WorkerPoolQueue:
//   - It's responsible to glue all together, using the "base queue" to provide "worker pool" functionality. It creates
//     new workers if needed and can flush the queue, running all the items synchronously till it finishes.
//   - Its "Push" function doesn't block forever, it will return an error if the queue is full after the timeout.
//
// 7. Manager:
//   - The purpose of it is to serve as a centralized manager for multiple WorkerPoolQueue instances. Whenever we want
//     to create a new queue, flush, or get a specific queue, we could use it.
//
// A queue can be "simple" or "unique". A unique queue will try to avoid duplicate items.
// Unique queue's "Has" function can be used to check whether an item is already in the queue,
// although it's not 100% reliable due to the lack of proper transaction support.
// Simple queue's "Has" function always returns "has=false".
//
// A WorkerPoolQueue is a generic struct; this means it will work with any type but just for that type.
// If you want another kind of items to run, you would have to call the manager to create a new WorkerPoolQueue for you
// with a different handler that works with this new type of item. As an example of this:
//
//	 func Init() error {
//		 itemQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "queue-name", handler)
//		 ...
//	 }
//	 func handler(items ...*mypkg.QueueItem) []*mypkg.QueueItem { ... }
package queue

import "code.gitea.io/gitea/modules/util"

type HandlerFuncT[T any] func(...T) (unhandled []T)

var ErrAlreadyInQueue = util.NewAlreadyExistErrorf("already in queue")
