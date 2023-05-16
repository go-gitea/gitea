// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

type LogChecker struct {
	*log.EventWriterBaseImpl

	filterMessages []string
	filtered       []bool

	stopMark string
	stopped  bool

	mu sync.Mutex
}

func (lc *LogChecker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-lc.Queue:
			if !ok {
				return
			}
			lc.checkLogEvent(event)
		}
	}
}

func (lc *LogChecker) checkLogEvent(event *log.EventFormatted) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for i, msg := range lc.filterMessages {
		if strings.Contains(event.Origin.MsgSimpleText, msg) {
			lc.filtered[i] = true
		}
	}
	if strings.Contains(event.Origin.MsgSimpleText, lc.stopMark) {
		lc.stopped = true
	}
}

var checkerIndex int64

func NewLogChecker(namePrefix string) (logChecker *LogChecker, cancel func()) {
	logger := log.GetManager().GetLogger(namePrefix)
	newCheckerIndex := atomic.AddInt64(&checkerIndex, 1)
	writerName := namePrefix + "-" + fmt.Sprint(newCheckerIndex)

	lc := &LogChecker{}
	lc.EventWriterBaseImpl = log.NewEventWriterBase(writerName, "test-log-checker", log.WriterMode{})
	logger.AddWriters(lc)
	return lc, func() { _ = logger.RemoveWriter(writerName) }
}

// Filter will make the `Check` function to check if these logs are outputted.
func (lc *LogChecker) Filter(msgs ...string) *LogChecker {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.filterMessages = make([]string, len(msgs))
	copy(lc.filterMessages, msgs)
	lc.filtered = make([]bool, len(lc.filterMessages))
	return lc
}

func (lc *LogChecker) StopMark(msg string) *LogChecker {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.stopMark = msg
	lc.stopped = false
	return lc
}

// Check returns the filtered slice and whether the stop mark is reached.
func (lc *LogChecker) Check(d time.Duration) (filtered []bool, stopped bool) {
	stop := time.Now().Add(d)

	for {
		lc.mu.Lock()
		stopped = lc.stopped
		lc.mu.Unlock()

		if time.Now().After(stop) || stopped {
			lc.mu.Lock()
			f := make([]bool, len(lc.filtered))
			copy(f, lc.filtered)
			lc.mu.Unlock()
			return f, stopped
		}
		time.Sleep(10 * time.Millisecond)
	}
}
