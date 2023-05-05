// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

type LogChecker struct {
	logger          *log.MultiChannelledLogger
	loggerName      string
	eventLoggerName string

	filterMessages []string
	filtered       []bool

	stopMark string
	stopped  bool

	mu sync.Mutex
}

func (lc *LogChecker) LogEvent(event *log.Event) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for i, msg := range lc.filterMessages {
		if strings.Contains(event.GetMsg(), msg) {
			lc.filtered[i] = true
		}
	}
	if strings.Contains(event.GetMsg(), lc.stopMark) {
		lc.stopped = true
	}
	return nil
}

func (lc *LogChecker) Close() {}

func (lc *LogChecker) Flush() {}

func (lc *LogChecker) GetLevel() log.Level {
	return log.TRACE
}

func (lc *LogChecker) GetStacktraceLevel() log.Level {
	return log.NONE
}

func (lc *LogChecker) GetName() string {
	return lc.eventLoggerName
}

func (lc *LogChecker) ReleaseReopen() error {
	return nil
}

var checkerIndex int64

func NewLogChecker(loggerName string) (logChecker *LogChecker, cancel func()) {
	logger := log.GetLogger(loggerName)
	newCheckerIndex := atomic.AddInt64(&checkerIndex, 1)
	lc := &LogChecker{
		logger:          logger,
		loggerName:      loggerName,
		eventLoggerName: "TestLogChecker-" + strconv.FormatInt(newCheckerIndex, 10),
	}
	if err := logger.AddLogger(lc); err != nil {
		panic(err) // it's impossible
	}
	return lc, func() { _, _ = logger.DelLogger(lc.GetName()) }
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
