// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	infiniteTimerC        = make(chan time.Time)
	batchDebounceDuration = 100 * time.Millisecond
	workerIdleDuration    = 1 * time.Second

	unhandledItemRequeueDuration atomic.Int64 // to avoid data race during test
)

func init() {
	unhandledItemRequeueDuration.Store(int64(5 * time.Second))
}

// workerGroup is a group of workers to work with a WorkerPoolQueue
type workerGroup[T any] struct {
	q  *WorkerPoolQueue[T]
	wg sync.WaitGroup

	ctxWorker       context.Context
	ctxWorkerCancel context.CancelFunc

	batchBuffer []T
	popItemChan chan []byte
	popItemErr  chan error
}

func (wg *workerGroup[T]) doPrepareWorkerContext() {
	wg.ctxWorker, wg.ctxWorkerCancel = context.WithCancel(wg.q.ctxRun)
}

// doDispatchBatchToWorker dispatches a batch of items to worker's channel.
// If the channel is full, it tries to start a new worker if possible.
func (q *WorkerPoolQueue[T]) doDispatchBatchToWorker(wg *workerGroup[T], flushChan chan flushType) {
	batch := wg.batchBuffer
	wg.batchBuffer = nil

	if len(batch) == 0 {
		return
	}

	full := false
	select {
	case q.batchChan <- batch:
	default:
		full = true
	}

	q.workerNumMu.Lock()
	noWorker := q.workerNum == 0
	if full || noWorker {
		if q.workerNum < q.workerMaxNum || noWorker && q.workerMaxNum <= 0 {
			q.workerNum++
			q.doStartNewWorker(wg)
		}
	}
	q.workerNumMu.Unlock()

	if full {
		select {
		case q.batchChan <- batch:
		case flush := <-flushChan:
			q.doWorkerHandle(batch)
			q.doFlush(wg, flush)
		case <-q.ctxRun.Done():
			q.safeHandler(batch...)
		}
	}
}

// doWorkerHandle calls the safeHandler to handle a batch of items, and it increases/decreases the active worker number.
// If the context has been canceled, it should not be caller because the "Push" still needs the context, in such case, call q.safeHandler directly
func (q *WorkerPoolQueue[T]) doWorkerHandle(batch []T) {
	q.workerNumMu.Lock()
	q.workerActiveNum++
	q.workerNumMu.Unlock()

	defer func() {
		q.workerNumMu.Lock()
		q.workerActiveNum--
		q.workerNumMu.Unlock()
	}()

	unhandled := q.safeHandler(batch...)
	// if none of the items were handled, it should back-off for a few seconds
	// in this case the handler (eg: document indexer) may have encountered some errors/failures
	if len(unhandled) == len(batch) && unhandledItemRequeueDuration.Load() != 0 {
		log.Error("Queue %q failed to handle batch of %d items, backoff for a few seconds", q.GetName(), len(batch))
		time.Sleep(time.Duration(unhandledItemRequeueDuration.Load()))
	}
	for _, item := range unhandled {
		if err := q.Push(item); err != nil {
			log.Error("Failed to requeue item for queue %q: %v", q.GetName(), err)
		}
	}
}

// doStartNewWorker starts a new worker for the queue, the worker reads from worker's channel and handles the items.
func (q *WorkerPoolQueue[T]) doStartNewWorker(wp *workerGroup[T]) {
	wp.wg.Add(1)

	go func() {
		defer wp.wg.Done()

		log.Debug("Queue %q starts new worker", q.GetName())
		defer log.Debug("Queue %q stops idle worker", q.GetName())

		t := time.NewTicker(workerIdleDuration)
		keepWorking := true
		stopWorking := func() {
			q.workerNumMu.Lock()
			keepWorking = false
			q.workerNum--
			q.workerNumMu.Unlock()
		}
		for keepWorking {
			select {
			case <-wp.ctxWorker.Done():
				stopWorking()
			case batch, ok := <-q.batchChan:
				if !ok {
					stopWorking()
				} else {
					q.doWorkerHandle(batch)
					t.Reset(workerIdleDuration)
				}
			case <-t.C:
				q.workerNumMu.Lock()
				keepWorking = q.workerNum <= 1
				if !keepWorking {
					q.workerNum--
				}
				q.workerNumMu.Unlock()
			}
		}
	}()
}

// doFlush flushes the queue: it tries to read all items from the queue and handles them.
// It is for testing purpose only. It's not designed to work for a cluster.
func (q *WorkerPoolQueue[T]) doFlush(wg *workerGroup[T], flush flushType) {
	log.Debug("Queue %q starts flushing", q.GetName())
	defer log.Debug("Queue %q finishes flushing", q.GetName())

	// stop all workers, and prepare a new worker context to start new workers

	wg.ctxWorkerCancel()
	wg.wg.Wait()

	defer func() {
		close(flush)
		wg.doPrepareWorkerContext()
	}()

	// drain the batch channel first
loop:
	for {
		select {
		case batch := <-q.batchChan:
			q.doWorkerHandle(batch)
		default:
			break loop
		}
	}

	// drain the popItem channel
	emptyCounter := 0
	for {
		select {
		case data, dataOk := <-wg.popItemChan:
			if !dataOk {
				return
			}
			emptyCounter = 0
			if v, jsonOk := q.unmarshal(data); !jsonOk {
				continue
			} else {
				q.doWorkerHandle([]T{v})
			}
		case err := <-wg.popItemErr:
			if !q.isCtxRunCanceled() {
				log.Error("Failed to pop item from queue %q (doFlush): %v", q.GetName(), err)
			}
			return
		case <-q.ctxRun.Done():
			log.Debug("Queue %q is shutting down", q.GetName())
			return
		case <-time.After(20 * time.Millisecond):
			// There is no reliable way to make sure all queue items are consumed by the Flush, there always might be some items stored in some buffers/temp variables.
			// If we run Gitea in a cluster, we can even not guarantee all items are consumed in a deterministic instance.
			// Luckily, the "Flush" trick is only used in tests, so far so good.
			if cnt, _ := q.baseQueue.Len(q.ctxRun); cnt == 0 && len(wg.popItemChan) == 0 {
				emptyCounter++
			}
			if emptyCounter >= 2 {
				return
			}
		}
	}
}

func (q *WorkerPoolQueue[T]) isCtxRunCanceled() bool {
	select {
	case <-q.ctxRun.Done():
		return true
	default:
		return false
	}
}

var skipFlushChan = make(chan flushType) // an empty flush chan, used to skip reading other flush requests

// doRun is the main loop of the queue. All related "doXxx" functions are executed in its context.
func (q *WorkerPoolQueue[T]) doRun() {
	log.Debug("Queue %q starts running", q.GetName())
	defer log.Debug("Queue %q stops running", q.GetName())

	wg := &workerGroup[T]{q: q}
	wg.doPrepareWorkerContext()
	wg.popItemChan, wg.popItemErr = popItemByChan(q.ctxRun, q.baseQueue.PopItem)

	defer func() {
		q.ctxRunCancel()

		// drain all data on the fly
		// since the queue is shutting down, we can only call the handler, but not dispatch to workers because the context is canceled
		// it can't call doWorkerHandle either, because there is no chance to push unhandled items back to the queue
		close(q.batchChan)
		for batch := range q.batchChan {
			q.safeHandler(batch...)
		}
		if len(wg.batchBuffer) > 0 {
			q.safeHandler(wg.batchBuffer...)
		}
		for data := range wg.popItemChan {
			if v, ok := q.unmarshal(data); ok {
				q.safeHandler(v)
			}
		}
	}()

	var batchDispatchC <-chan time.Time = infiniteTimerC
	for {
		select {
		case data, dataOk := <-wg.popItemChan:
			if !dataOk {
				return
			}
			if v, jsonOk := q.unmarshal(data); !jsonOk {
				testRecorder.Record("pop:corrupted:%s", data) // in rare cases the levelqueue(leveldb) might be corrupted
				continue
			} else {
				wg.batchBuffer = append(wg.batchBuffer, v)
			}
			if len(wg.batchBuffer) >= q.batchLength {
				q.doDispatchBatchToWorker(wg, q.flushChan)
			} else if batchDispatchC == infiniteTimerC {
				batchDispatchC = time.After(batchDebounceDuration)
			} // else: batchDispatchC is already a debounce timer, it will be triggered soon
		case <-batchDispatchC:
			batchDispatchC = infiniteTimerC
			q.doDispatchBatchToWorker(wg, q.flushChan)
		case flush := <-q.flushChan:
			// before flushing, it needs to try to dispatch the batch to worker first, in case there is no worker running
			// after the flushing, there is at least one worker running, so "doFlush" could wait for workers to finish
			// since we are already in a "flush" operation, so the dispatching function shouldn't read the flush chan.
			q.doDispatchBatchToWorker(wg, skipFlushChan)
			q.doFlush(wg, flush)
		case err := <-wg.popItemErr:
			if !q.isCtxRunCanceled() {
				log.Error("Failed to pop item from queue %q (doRun): %v", q.GetName(), err)
			}
			return
		case <-q.ctxRun.Done():
			log.Debug("Queue %q is shutting down", q.GetName())
			return
		}
	}
}
