// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrBulkItemRetry is returned in BulkProcessor from a worker when
	// a response item needs to be retried.
	ErrBulkItemRetry = errors.New("elastic: uncommitted bulk response items")

	defaultRetryItemStatusCodes = []int{408, 429, 503, 507}
)

// BulkProcessorService allows to easily process bulk requests. It allows setting
// policies when to flush new bulk requests, e.g. based on a number of actions,
// on the size of the actions, and/or to flush periodically. It also allows
// to control the number of concurrent bulk requests allowed to be executed
// in parallel.
//
// BulkProcessorService, by default, commits either every 1000 requests or when the
// (estimated) size of the bulk requests exceeds 5 MB. However, it does not
// commit periodically. BulkProcessorService also does retry by default, using
// an exponential backoff algorithm. It also will automatically re-enqueue items
// returned with a status of 408, 429, 503 or 507. You can change this
// behavior with RetryItemStatusCodes.
//
// The caller is responsible for setting the index and type on every
// bulk request added to BulkProcessorService.
//
// BulkProcessorService takes ideas from the BulkProcessor of the
// Elasticsearch Java API as documented in
// https://www.elastic.co/guide/en/elasticsearch/client/java-api/current/java-docs-bulk-processor.html.
type BulkProcessorService struct {
	c                    *Client
	beforeFn             BulkBeforeFunc
	afterFn              BulkAfterFunc
	name                 string        // name of processor
	numWorkers           int           // # of workers (>= 1)
	bulkActions          int           // # of requests after which to commit
	bulkSize             int           // # of bytes after which to commit
	flushInterval        time.Duration // periodic flush interval
	wantStats            bool          // indicates whether to gather statistics
	backoff              Backoff       // a custom Backoff to use for errors
	retryItemStatusCodes []int         // array of status codes for bulk response line items that may be retried
}

// NewBulkProcessorService creates a new BulkProcessorService.
func NewBulkProcessorService(client *Client) *BulkProcessorService {
	return &BulkProcessorService{
		c:           client,
		numWorkers:  1,
		bulkActions: 1000,
		bulkSize:    5 << 20, // 5 MB
		backoff: NewExponentialBackoff(
			time.Duration(200)*time.Millisecond,
			time.Duration(10000)*time.Millisecond,
		),
		retryItemStatusCodes: defaultRetryItemStatusCodes,
	}
}

// BulkBeforeFunc defines the signature of callbacks that are executed
// before a commit to Elasticsearch.
type BulkBeforeFunc func(executionId int64, requests []BulkableRequest)

// BulkAfterFunc defines the signature of callbacks that are executed
// after a commit to Elasticsearch. The err parameter signals an error.
type BulkAfterFunc func(executionId int64, requests []BulkableRequest, response *BulkResponse, err error)

// Before specifies a function to be executed before bulk requests get committed
// to Elasticsearch.
func (s *BulkProcessorService) Before(fn BulkBeforeFunc) *BulkProcessorService {
	s.beforeFn = fn
	return s
}

// After specifies a function to be executed when bulk requests have been
// committed to Elasticsearch. The After callback executes both when the
// commit was successful as well as on failures.
func (s *BulkProcessorService) After(fn BulkAfterFunc) *BulkProcessorService {
	s.afterFn = fn
	return s
}

// Name is an optional name to identify this bulk processor.
func (s *BulkProcessorService) Name(name string) *BulkProcessorService {
	s.name = name
	return s
}

// Workers is the number of concurrent workers allowed to be
// executed. Defaults to 1 and must be greater or equal to 1.
func (s *BulkProcessorService) Workers(num int) *BulkProcessorService {
	s.numWorkers = num
	return s
}

// BulkActions specifies when to flush based on the number of actions
// currently added. Defaults to 1000 and can be set to -1 to be disabled.
func (s *BulkProcessorService) BulkActions(bulkActions int) *BulkProcessorService {
	s.bulkActions = bulkActions
	return s
}

// BulkSize specifies when to flush based on the size (in bytes) of the actions
// currently added. Defaults to 5 MB and can be set to -1 to be disabled.
func (s *BulkProcessorService) BulkSize(bulkSize int) *BulkProcessorService {
	s.bulkSize = bulkSize
	return s
}

// FlushInterval specifies when to flush at the end of the given interval.
// This is disabled by default. If you want the bulk processor to
// operate completely asynchronously, set both BulkActions and BulkSize to
// -1 and set the FlushInterval to a meaningful interval.
func (s *BulkProcessorService) FlushInterval(interval time.Duration) *BulkProcessorService {
	s.flushInterval = interval
	return s
}

// Stats tells bulk processor to gather stats while running.
// Use Stats to return the stats. This is disabled by default.
func (s *BulkProcessorService) Stats(wantStats bool) *BulkProcessorService {
	s.wantStats = wantStats
	return s
}

// Backoff sets the backoff strategy to use for errors.
func (s *BulkProcessorService) Backoff(backoff Backoff) *BulkProcessorService {
	s.backoff = backoff
	return s
}

// RetryItemStatusCodes sets an array of status codes that indicate that a bulk
// response line item should be retried.
func (s *BulkProcessorService) RetryItemStatusCodes(retryItemStatusCodes ...int) *BulkProcessorService {
	s.retryItemStatusCodes = retryItemStatusCodes
	return s
}

// Do creates a new BulkProcessor and starts it.
// Consider the BulkProcessor as a running instance that accepts bulk requests
// and commits them to Elasticsearch, spreading the work across one or more
// workers.
//
// You can interoperate with the BulkProcessor returned by Do, e.g. Start and
// Stop (or Close) it.
//
// Context is an optional context that is passed into the bulk request
// service calls. In contrast to other operations, this context is used in
// a long running process. You could use it to pass e.g. loggers, but you
// shouldn't use it for cancellation.
//
// Calling Do several times returns new BulkProcessors. You probably don't
// want to do this. BulkProcessorService implements just a builder pattern.
func (s *BulkProcessorService) Do(ctx context.Context) (*BulkProcessor, error) {

	retryItemStatusCodes := make(map[int]struct{})
	for _, code := range s.retryItemStatusCodes {
		retryItemStatusCodes[code] = struct{}{}
	}

	p := newBulkProcessor(
		s.c,
		s.beforeFn,
		s.afterFn,
		s.name,
		s.numWorkers,
		s.bulkActions,
		s.bulkSize,
		s.flushInterval,
		s.wantStats,
		s.backoff,
		retryItemStatusCodes)

	err := p.Start(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// -- Bulk Processor Statistics --

// BulkProcessorStats contains various statistics of a bulk processor
// while it is running. Use the Stats func to return it while running.
type BulkProcessorStats struct {
	Flushed   int64 // number of times the flush interval has been invoked
	Committed int64 // # of times workers committed bulk requests
	Indexed   int64 // # of requests indexed
	Created   int64 // # of requests that ES reported as creates (201)
	Updated   int64 // # of requests that ES reported as updates
	Deleted   int64 // # of requests that ES reported as deletes
	Succeeded int64 // # of requests that ES reported as successful
	Failed    int64 // # of requests that ES reported as failed

	Workers []*BulkProcessorWorkerStats // stats for each worker
}

// BulkProcessorWorkerStats represents per-worker statistics.
type BulkProcessorWorkerStats struct {
	Queued       int64         // # of requests queued in this worker
	LastDuration time.Duration // duration of last commit
}

// newBulkProcessorStats initializes and returns a BulkProcessorStats struct.
func newBulkProcessorStats(workers int) *BulkProcessorStats {
	stats := &BulkProcessorStats{
		Workers: make([]*BulkProcessorWorkerStats, workers),
	}
	for i := 0; i < workers; i++ {
		stats.Workers[i] = &BulkProcessorWorkerStats{}
	}
	return stats
}

func (st *BulkProcessorStats) dup() *BulkProcessorStats {
	dst := new(BulkProcessorStats)
	dst.Flushed = st.Flushed
	dst.Committed = st.Committed
	dst.Indexed = st.Indexed
	dst.Created = st.Created
	dst.Updated = st.Updated
	dst.Deleted = st.Deleted
	dst.Succeeded = st.Succeeded
	dst.Failed = st.Failed
	for _, src := range st.Workers {
		dst.Workers = append(dst.Workers, src.dup())
	}
	return dst
}

func (st *BulkProcessorWorkerStats) dup() *BulkProcessorWorkerStats {
	dst := new(BulkProcessorWorkerStats)
	dst.Queued = st.Queued
	dst.LastDuration = st.LastDuration
	return dst
}

// -- Bulk Processor --

// BulkProcessor encapsulates a task that accepts bulk requests and
// orchestrates committing them to Elasticsearch via one or more workers.
//
// BulkProcessor is returned by setting up a BulkProcessorService and
// calling the Do method.
type BulkProcessor struct {
	c                    *Client
	beforeFn             BulkBeforeFunc
	afterFn              BulkAfterFunc
	name                 string
	bulkActions          int
	bulkSize             int
	numWorkers           int
	executionId          int64
	requestsC            chan BulkableRequest
	workerWg             sync.WaitGroup
	workers              []*bulkWorker
	flushInterval        time.Duration
	flusherStopC         chan struct{}
	wantStats            bool
	retryItemStatusCodes map[int]struct{}
	backoff              Backoff

	startedMu sync.Mutex // guards the following block
	started   bool

	statsMu sync.Mutex // guards the following block
	stats   *BulkProcessorStats

	stopReconnC chan struct{} // channel to signal stop reconnection attempts
}

func newBulkProcessor(
	client *Client,
	beforeFn BulkBeforeFunc,
	afterFn BulkAfterFunc,
	name string,
	numWorkers int,
	bulkActions int,
	bulkSize int,
	flushInterval time.Duration,
	wantStats bool,
	backoff Backoff,
	retryItemStatusCodes map[int]struct{}) *BulkProcessor {
	return &BulkProcessor{
		c:                    client,
		beforeFn:             beforeFn,
		afterFn:              afterFn,
		name:                 name,
		numWorkers:           numWorkers,
		bulkActions:          bulkActions,
		bulkSize:             bulkSize,
		flushInterval:        flushInterval,
		wantStats:            wantStats,
		retryItemStatusCodes: retryItemStatusCodes,
		backoff:              backoff,
	}
}

// Start starts the bulk processor. If the processor is already started,
// nil is returned.
func (p *BulkProcessor) Start(ctx context.Context) error {
	p.startedMu.Lock()
	defer p.startedMu.Unlock()

	if p.started {
		return nil
	}

	// We must have at least one worker.
	if p.numWorkers < 1 {
		p.numWorkers = 1
	}

	p.requestsC = make(chan BulkableRequest)
	p.executionId = 0
	p.stats = newBulkProcessorStats(p.numWorkers)
	p.stopReconnC = make(chan struct{})

	// Create and start up workers.
	p.workers = make([]*bulkWorker, p.numWorkers)
	for i := 0; i < p.numWorkers; i++ {
		p.workerWg.Add(1)
		p.workers[i] = newBulkWorker(p, i)
		go p.workers[i].work(ctx)
	}

	// Start the ticker for flush (if enabled)
	if int64(p.flushInterval) > 0 {
		p.flusherStopC = make(chan struct{})
		go p.flusher(p.flushInterval)
	}

	p.started = true

	return nil
}

// Stop is an alias for Close.
func (p *BulkProcessor) Stop() error {
	return p.Close()
}

// Close stops the bulk processor previously started with Do.
// If it is already stopped, this is a no-op and nil is returned.
//
// By implementing Close, BulkProcessor implements the io.Closer interface.
func (p *BulkProcessor) Close() error {
	p.startedMu.Lock()
	defer p.startedMu.Unlock()

	// Already stopped? Do nothing.
	if !p.started {
		return nil
	}

	// Tell connection checkers to stop
	if p.stopReconnC != nil {
		close(p.stopReconnC)
		p.stopReconnC = nil
	}

	// Stop flusher (if enabled)
	if p.flusherStopC != nil {
		p.flusherStopC <- struct{}{}
		<-p.flusherStopC
		close(p.flusherStopC)
		p.flusherStopC = nil
	}

	// Stop all workers.
	close(p.requestsC)
	p.workerWg.Wait()

	p.started = false

	return nil
}

// Stats returns the latest bulk processor statistics.
// Collecting stats must be enabled first by calling Stats(true) on
// the service that created this processor.
func (p *BulkProcessor) Stats() BulkProcessorStats {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	return *p.stats.dup()
}

// Add adds a single request to commit by the BulkProcessorService.
//
// The caller is responsible for setting the index and type on the request.
func (p *BulkProcessor) Add(request BulkableRequest) {
	p.requestsC <- request
}

// Flush manually asks all workers to commit their outstanding requests.
// It returns only when all workers acknowledge completion.
func (p *BulkProcessor) Flush() error {
	p.statsMu.Lock()
	p.stats.Flushed++
	p.statsMu.Unlock()

	for _, w := range p.workers {
		w.flushC <- struct{}{}
		<-w.flushAckC // wait for completion
	}
	return nil
}

// flusher is a single goroutine that periodically asks all workers to
// commit their outstanding bulk requests. It is only started if
// FlushInterval is greater than 0.
func (p *BulkProcessor) flusher(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C: // Periodic flush
			p.Flush() // TODO swallow errors here?

		case <-p.flusherStopC:
			p.flusherStopC <- struct{}{}
			return
		}
	}
}

// -- Bulk Worker --

// bulkWorker encapsulates a single worker, running in a goroutine,
// receiving bulk requests and eventually committing them to Elasticsearch.
// It is strongly bound to a BulkProcessor.
type bulkWorker struct {
	p           *BulkProcessor
	i           int
	bulkActions int
	bulkSize    int
	service     *BulkService
	flushC      chan struct{}
	flushAckC   chan struct{}
}

// newBulkWorker creates a new bulkWorker instance.
func newBulkWorker(p *BulkProcessor, i int) *bulkWorker {
	return &bulkWorker{
		p:           p,
		i:           i,
		bulkActions: p.bulkActions,
		bulkSize:    p.bulkSize,
		service:     NewBulkService(p.c),
		flushC:      make(chan struct{}),
		flushAckC:   make(chan struct{}),
	}
}

// work waits for bulk requests and manual flush calls on the respective
// channels and is invoked as a goroutine when the bulk processor is started.
func (w *bulkWorker) work(ctx context.Context) {
	defer func() {
		w.p.workerWg.Done()
		close(w.flushAckC)
		close(w.flushC)
	}()

	var stop bool
	for !stop {
		var err error
		select {
		case req, open := <-w.p.requestsC:
			if open {
				// Received a new request
				if _, err = req.Source(); err == nil {
					w.service.Add(req)
					if w.commitRequired() {
						err = w.commit(ctx)
					}
				}
			} else {
				// Channel closed: Stop.
				stop = true
				if w.service.NumberOfActions() > 0 {
					err = w.commit(ctx)
				}
			}

		case <-w.flushC:
			// Commit outstanding requests
			if w.service.NumberOfActions() > 0 {
				err = w.commit(ctx)
			}
			w.flushAckC <- struct{}{}
		}
		if err != nil {
			w.p.c.errorf("elastic: bulk processor %q was unable to perform work: %v", w.p.name, err)
			if !stop {
				waitForActive := func() {
					// Add back pressure to prevent Add calls from filling up the request queue
					ready := make(chan struct{})
					go w.waitForActiveConnection(ready)
					<-ready
				}
				if _, ok := err.(net.Error); ok {
					waitForActive()
				} else if IsConnErr(err) {
					waitForActive()
				}
			}
		}
	}
}

// commit commits the bulk requests in the given service,
// invoking callbacks as specified.
func (w *bulkWorker) commit(ctx context.Context) error {
	var res *BulkResponse

	// commitFunc will commit bulk requests and, on failure, be retried
	// via exponential backoff
	commitFunc := func() error {
		var err error
		// Save requests because they will be reset in service.Do
		reqs := w.service.requests
		res, err = w.service.Do(ctx)
		if err == nil {
			// Overall bulk request was OK.  But each bulk response item also has a status
			if w.p.retryItemStatusCodes != nil && len(w.p.retryItemStatusCodes) > 0 {
				// Check res.Items since some might be soft failures
				if res.Items != nil && res.Errors {
					// res.Items will be 1 to 1 with reqs in same order
					for i, item := range res.Items {
						for _, result := range item {
							if _, found := w.p.retryItemStatusCodes[result.Status]; found {
								w.service.Add(reqs[i])
								if err == nil {
									err = ErrBulkItemRetry
								}
							}
						}
					}
				}
			}
		}
		return err
	}
	// notifyFunc will be called if retry fails
	notifyFunc := func(err error) {
		w.p.c.errorf("elastic: bulk processor %q failed but may retry: %v", w.p.name, err)
	}

	id := atomic.AddInt64(&w.p.executionId, 1)

	// Update # documents in queue before eventual retries
	w.p.statsMu.Lock()
	if w.p.wantStats {
		w.p.stats.Workers[w.i].Queued = int64(len(w.service.requests))
	}
	w.p.statsMu.Unlock()

	// Save requests because they will be reset in commitFunc
	reqs := w.service.requests

	// Invoke before callback
	if w.p.beforeFn != nil {
		w.p.beforeFn(id, reqs)
	}

	// Commit bulk requests
	err := RetryNotify(commitFunc, w.p.backoff, notifyFunc)
	w.updateStats(res)
	if err != nil {
		w.p.c.errorf("elastic: bulk processor %q failed: %v", w.p.name, err)
	}

	// Invoke after callback
	if w.p.afterFn != nil {
		w.p.afterFn(id, reqs, res, err)
	}

	return err
}

func (w *bulkWorker) waitForActiveConnection(ready chan<- struct{}) {
	defer close(ready)

	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	client := w.p.c
	stopReconnC := w.p.stopReconnC
	w.p.c.errorf("elastic: bulk processor %q is waiting for an active connection", w.p.name)

	// loop until a health check finds at least 1 active connection or the reconnection channel is closed
	for {
		select {
		case _, ok := <-stopReconnC:
			if !ok {
				w.p.c.errorf("elastic: bulk processor %q active connection check interrupted", w.p.name)
				return
			}
		case <-t.C:
			client.healthcheck(context.Background(), 3*time.Second, true)
			if client.mustActiveConn() == nil {
				// found an active connection
				// exit and signal done to the WaitGroup
				return
			}
		}
	}
}

func (w *bulkWorker) updateStats(res *BulkResponse) {
	// Update stats
	if res != nil {
		w.p.statsMu.Lock()
		if w.p.wantStats {
			w.p.stats.Committed++
			if res != nil {
				w.p.stats.Indexed += int64(len(res.Indexed()))
				w.p.stats.Created += int64(len(res.Created()))
				w.p.stats.Updated += int64(len(res.Updated()))
				w.p.stats.Deleted += int64(len(res.Deleted()))
				w.p.stats.Succeeded += int64(len(res.Succeeded()))
				w.p.stats.Failed += int64(len(res.Failed()))
			}
			w.p.stats.Workers[w.i].Queued = int64(len(w.service.requests))
			w.p.stats.Workers[w.i].LastDuration = time.Duration(int64(res.Took)) * time.Millisecond
		}
		w.p.statsMu.Unlock()
	}
}

// commitRequired returns true if the service has to commit its
// bulk requests. This can be either because the number of actions
// or the estimated size in bytes is larger than specified in the
// BulkProcessorService.
func (w *bulkWorker) commitRequired() bool {
	if w.bulkActions >= 0 && w.service.NumberOfActions() >= w.bulkActions {
		return true
	}
	if w.bulkSize >= 0 && w.service.EstimatedSizeInBytes() >= int64(w.bulkSize) {
		return true
	}
	return false
}
