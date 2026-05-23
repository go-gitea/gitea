// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
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

// logPrinterFunc is used to output the log for a request
type logPrinterFunc func(trigger Event, record *requestRecord)

type loggerRequestManager struct {
	logPrint   logPrinterFunc
	reqRecords sync.Map // it only contains the active requests which haven't been detected as "slow"
}

func (manager *loggerRequestManager) startSlowQueryDetector(threshold time.Duration) {
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

				// print logs for slow requests
				manager.reqRecords.Range(func(key, value any) bool {
					index, record := key.(uint64), value.(*requestRecord)
					if now.Sub(record.startTime) >= threshold {
						manager.logPrint(StillExecutingEvent, record)
						manager.reqRecords.Delete(index)
					}
					return true
				})
			}
		}
	})
}

func (manager *loggerRequestManager) handleRequestRecord(record *requestRecord) func() {
	manager.reqRecords.Store(record.index, record)
	manager.logPrint(StartEvent, record)

	return func() {
		// just in case there is a panic. now the panics are all recovered in middleware.go
		localPanicErr := recover()
		if localPanicErr != nil {
			record.lock.Lock()
			record.panicError = fmt.Errorf("%v\n%s", localPanicErr, log.Stack(2))
			record.lock.Unlock()
		}

		manager.reqRecords.Delete(record.index)
		manager.logPrint(EndEvent, record)

		if localPanicErr != nil {
			// the panic wasn't recovered before us, so we should pass it up, and let the framework handle the panic error
			panic(localPanicErr)
		}
	}
}
