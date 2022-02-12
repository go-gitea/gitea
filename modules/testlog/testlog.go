// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package testlog

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

type LogChecker struct {
	logger             *log.MultiChannelledLogger
	loggerName         string
	eventLoggerName    string
	expectedMessages   []string
	expectedMessageIdx int
	mu                 sync.Mutex
}

func (lc *LogChecker) LogEvent(event *log.Event) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.expectedMessageIdx < len(lc.expectedMessages) {
		if strings.Contains(event.GetMsg(), lc.expectedMessages[lc.expectedMessageIdx]) {
			lc.expectedMessageIdx++
		}
	}
	return nil
}

func (lc *LogChecker) Close() {
}

func (lc *LogChecker) Flush() {
}

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

func NewLogChecker(loggerName string) (*LogChecker, func()) {
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

	return lc, func() {
		_, _ = logger.DelLogger(lc.GetName())
	}
}

// ExpectContains will make the `Check` function to check if these logs are outputted.
// we could refactor this function to accept user-defined functions to do more filter work, ex: Except(Contains("..."), NotContains("..."), ...)
func (lc *LogChecker) ExpectContains(msg string, msgs ...string) {
	lc.expectedMessages = make([]string, 0, len(msg)+1)
	lc.expectedMessages = append(lc.expectedMessages, msg)
	lc.expectedMessages = append(lc.expectedMessages, msgs...)
}

// Check returns nil if the logs are expected. Otherwise it returns the error with reason.
func (lc *LogChecker) Check() error {
	lastProcessedCount := lc.logger.GetProcessedCount()

	// in case the LogEvent is still being processed, we should wait for a while to make sure our EventLogger could receive all events.
	for i := 0; i < 100; i++ {
		lc.mu.Lock()
		if lc.expectedMessageIdx == len(lc.expectedMessages) {
			return nil
		}
		lc.mu.Unlock()

		// we assume that the MultiChannelledLog can process one log event in 10ms.
		// if there is a long time that no more log is processed, then we are sure that there is really no more logs.
		time.Sleep(10 * time.Millisecond)
		currProcessedCount := lc.logger.GetProcessedCount()
		if currProcessedCount == lastProcessedCount {
			break
		}
		lastProcessedCount = currProcessedCount
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()
	return fmt.Errorf("expect to see message: %q, but failed", lc.expectedMessages[lc.expectedMessageIdx])
}
