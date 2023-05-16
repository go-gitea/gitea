// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package testlogger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

var (
	prefix    string
	SlowTest  = 10 * time.Second
	SlowFlush = 5 * time.Second
)

// TestLogger is a logger which will write to the testing log
type TestLogger struct {
	log.WriterLogger
}

var WriterCloser = &testLoggerWriterCloser{}

type testLoggerWriterCloser struct {
	sync.RWMutex
	t []*testing.TB
}

func (w *testLoggerWriterCloser) pushT(t *testing.TB) {
	w.Lock()
	w.t = append(w.t, t)
	w.Unlock()
}

func (w *testLoggerWriterCloser) Write(p []byte) (int, error) {
	// There was a data race problem: the logger system could still try to output logs after the runner is finished.
	// So we must ensure that the "t" in stack is still valid.
	w.RLock()
	defer w.RUnlock()

	var t *testing.TB
	if len(w.t) > 0 {
		t = w.t[len(w.t)-1]
	}

	if len(p) > 0 && p[len(p)-1] == '\n' {
		p = p[:len(p)-1]
	}

	if t == nil || *t == nil {
		// if there is no running test, the log message should be outputted to console, to avoid losing important information.
		// the "???" prefix is used to match the "===" and "+++" in PrintCurrentTest
		return fmt.Fprintf(os.Stdout, "??? [TestLogger] %s\n", p)
	}

	defer func() {
		err := recover()
		if err == nil {
			return
		}
		var errString string
		errErr, ok := err.(error)
		if ok {
			errString = errErr.Error()
		} else {
			errString, ok = err.(string)
		}
		if !ok {
			panic(err)
		}
		if !strings.HasPrefix(errString, "Log in goroutine after ") {
			panic(err)
		}
	}()

	(*t).Log(string(p))
	return len(p), nil
}

func (w *testLoggerWriterCloser) popT() {
	w.Lock()
	if len(w.t) > 0 {
		w.t = w.t[:len(w.t)-1]
	}
	w.Unlock()
}

func (w *testLoggerWriterCloser) Close() error {
	return nil
}

func (w *testLoggerWriterCloser) Reset() {
	w.Lock()
	if len(w.t) > 0 {
		for _, t := range w.t {
			if t == nil {
				continue
			}
			fmt.Fprintf(os.Stdout, "Unclosed logger writer in test: %s", (*t).Name())
			(*t).Errorf("Unclosed logger writer in test: %s", (*t).Name())
		}
		w.t = nil
	}
	w.Unlock()
}

// PrintCurrentTest prints the current test to os.Stdout
func PrintCurrentTest(t testing.TB, skip ...int) func() {
	start := time.Now()
	actualSkip := 1
	if len(skip) > 0 {
		actualSkip = skip[0]
	}
	_, filename, line, _ := runtime.Caller(actualSkip)

	if log.CanColorStdout {
		fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", fmt.Formatter(log.NewColoredValue(t.Name())), strings.TrimPrefix(filename, prefix), line)
	} else {
		fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", t.Name(), strings.TrimPrefix(filename, prefix), line)
	}
	WriterCloser.pushT(&t)
	return func() {
		took := time.Since(start)
		if took > SlowTest {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s is a slow test (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgYellow)), fmt.Formatter(log.NewColoredValue(took, log.Bold, log.FgYellow)))
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s is a slow test (took %v)\n", t.Name(), took)
			}
		}
		timer := time.AfterFunc(SlowFlush, func() {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), SlowFlush)
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", t.Name(), SlowFlush)
			}
		})
		if err := queue.GetManager().FlushAll(context.Background(), time.Minute); err != nil {
			t.Errorf("Flushing queues failed with error %v", err)
		}
		timer.Stop()
		flushTook := time.Since(start) - took
		if flushTook > SlowFlush {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), fmt.Formatter(log.NewColoredValue(flushTook, log.Bold, log.FgRed)))
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", t.Name(), flushTook)
			}
		}
		WriterCloser.popT()
	}
}

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...interface{}) {
	if log.CanColorStdout {
		for i := 0; i < len(args); i++ {
			args[i] = log.NewColoredValue(args[i])
		}
	}
	fmt.Fprintf(os.Stdout, "\t"+format, args...)
}

// NewTestLogger creates a TestLogger as a log.LoggerProvider
func NewTestLogger() log.LoggerProvider {
	logger := &TestLogger{}
	logger.Colorize = log.CanColorStdout
	logger.Level = log.TRACE
	return logger
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *TestLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), log)
	if err != nil {
		return err
	}
	log.NewWriterLogger(WriterCloser)
	return nil
}

// Flush when log should be flushed
func (log *TestLogger) Flush() {
}

// ReleaseReopen does nothing
func (log *TestLogger) ReleaseReopen() error {
	return nil
}

// GetName returns the default name for this implementation
func (log *TestLogger) GetName() string {
	return "test"
}

func init() {
	const relFilePath = "modules/testlogger/testlogger.go"
	_, filename, _, _ := runtime.Caller(0)
	if !strings.HasSuffix(filename, relFilePath) {
		panic("source code file path doesn't match expected: " + relFilePath)
	}
	prefix = strings.TrimSuffix(filename, relFilePath)
}
