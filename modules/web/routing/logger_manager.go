// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/process"
)

// Event indicates when the printer is triggered
type Event int

const (
	// StartEvent at the beginning of a request
	StartEvent Event = iota

	// StillExecutingEvent the request is still executing
	StillExecutingEvent

	// EndEvent the request has ended (either completed or failed)
	EndEvent
)

// Printer is used to output the log for a request
type Printer func(trigger Event, record *requestRecord)

type requestRecordsManager struct {
	print Printer

	lock sync.Mutex

	requestRecords map[uint64]*requestRecord
	count          uint64
}

func (manager *requestRecordsManager) startSlowQueryDetector(threshold time.Duration) {
	go graceful.GetManager().RunWithShutdownContext(func(ctx context.Context) {
		ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: SlowQueryDetector", process.SystemProcessType, true)
		defer finished()
		// This go-routine checks all active requests every second.
		// If a request has been running for a long time (eg: /user/events), we also print a log with "still-executing" message
		// After the "still-executing" log is printed, the record will be removed from the map to prevent from duplicated logs in future

		// We do not care about accurate duration here. It just does the check periodically, 0.5s or 1.5s are all OK.
		t := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				now := time.Now()

				var slowRequests []*requestRecord

				// find all slow requests with lock
				manager.lock.Lock()
				for index, record := range manager.requestRecords {
					if now.Sub(record.startTime) < threshold {
						continue
					}

					slowRequests = append(slowRequests, record)
					delete(manager.requestRecords, index)
				}
				manager.lock.Unlock()

				// print logs for slow requests
				for _, record := range slowRequests {
					manager.print(StillExecutingEvent, record)
				}
			}
		}
	})
}

func (manager *requestRecordsManager) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		record := &requestRecord{
			startTime:      time.Now(),
			request:        req,
			responseWriter: w,
		}

		// generate a record index an insert into the map
		manager.lock.Lock()
		record.index = manager.count
		manager.count++
		manager.requestRecords[record.index] = record
		manager.lock.Unlock()

		defer func() {
			// just in case there is a panic. now the panics are all recovered in middleware.go
			localPanicErr := recover()
			if localPanicErr != nil {
				record.lock.Lock()
				record.panicError = localPanicErr
				record.lock.Unlock()
			}

			// remove from the record map
			manager.lock.Lock()
			delete(manager.requestRecords, record.index)
			manager.lock.Unlock()

			// log the end of the request
			manager.print(EndEvent, record)

			if localPanicErr != nil {
				// the panic wasn't recovered before us, so we should pass it up, and let the framework handle the panic error
				panic(localPanicErr)
			}
		}()

		req = req.WithContext(context.WithValue(req.Context(), contextKey, record))
		manager.print(StartEvent, record)
		next.ServeHTTP(w, req)
	})
}
