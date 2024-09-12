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

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

var (
	prefix    string
	SlowTest  = 10 * time.Second
	SlowFlush = 5 * time.Second
)

var WriterCloser = &testLoggerWriterCloser{}

type testLoggerWriterCloser struct {
	sync.RWMutex
	t []testing.TB
}

func (w *testLoggerWriterCloser) pushT(t testing.TB) {
	w.Lock()
	w.t = append(w.t, t)
	w.Unlock()
}

func (w *testLoggerWriterCloser) Write(p []byte) (int, error) {
	// There was a data race problem: the logger system could still try to output logs after the runner is finished.
	// So we must ensure that the "t" in stack is still valid.
	w.RLock()
	defer w.RUnlock()

	var t testing.TB
	if len(w.t) > 0 {
		t = w.t[len(w.t)-1]
	}

	if len(p) > 0 && p[len(p)-1] == '\n' {
		p = p[:len(p)-1]
	}

	if t == nil {
		// if there is no running test, the log message should be outputted to console, to avoid losing important information.
		// the "???" prefix is used to match the "===" and "+++" in PrintCurrentTest
		return fmt.Fprintf(os.Stdout, "??? [TestLogger] %s\n", p)
	}

	t.Log(string(p))
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
			_, _ = fmt.Fprintf(os.Stdout, "Unclosed logger writer in test: %s", t.Name())
			t.Errorf("Unclosed logger writer in test: %s", t.Name())
		}
		w.t = nil
	}
	w.Unlock()
}

// PrintCurrentTest prints the current test to os.Stdout
func PrintCurrentTest(t testing.TB, skip ...int) func() {
	t.Helper()
	start := time.Now()
	actualSkip := 1
	if len(skip) > 0 {
		actualSkip = skip[0] + 1
	}
	_, filename, line, _ := runtime.Caller(actualSkip)

	if log.CanColorStdout {
		_, _ = fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", fmt.Formatter(log.NewColoredValue(t.Name())), strings.TrimPrefix(filename, prefix), line)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", t.Name(), strings.TrimPrefix(filename, prefix), line)
	}
	WriterCloser.pushT(t)
	return func() {
		took := time.Since(start)
		if took > SlowTest {
			if log.CanColorStdout {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s is a slow test (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgYellow)), fmt.Formatter(log.NewColoredValue(took, log.Bold, log.FgYellow)))
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s is a slow test (took %v)\n", t.Name(), took)
			}
		}
		timer := time.AfterFunc(SlowFlush, func() {
			if log.CanColorStdout {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), SlowFlush)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", t.Name(), SlowFlush)
			}
		})
		if err := queue.GetManager().FlushAll(context.Background(), time.Minute); err != nil {
			t.Errorf("Flushing queues failed with error %v", err)
		}
		timer.Stop()
		flushTook := time.Since(start) - took
		if flushTook > SlowFlush {
			if log.CanColorStdout {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), fmt.Formatter(log.NewColoredValue(flushTook, log.Bold, log.FgRed)))
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", t.Name(), flushTook)
			}
		}
		WriterCloser.popT()
	}
}

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...any) {
	if log.CanColorStdout {
		for i := 0; i < len(args); i++ {
			args[i] = log.NewColoredValue(args[i])
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, "\t"+format, args...)
}

// TestLogEventWriter is a logger which will write to the testing log
type TestLogEventWriter struct {
	*log.EventWriterBaseImpl
}

// NewTestLoggerWriter creates a TestLogEventWriter as a log.LoggerProvider
func NewTestLoggerWriter(name string, mode log.WriterMode) log.EventWriter {
	w := &TestLogEventWriter{}
	w.EventWriterBaseImpl = log.NewEventWriterBase(name, "test-log-writer", mode)
	w.OutputWriteCloser = WriterCloser
	return w
}

func init() {
	const relFilePath = "modules/testlogger/testlogger.go"
	_, filename, _, _ := runtime.Caller(0)
	if !strings.HasSuffix(filename, relFilePath) {
		panic("source code file path doesn't match expected: " + relFilePath)
	}
	prefix = strings.TrimSuffix(filename, relFilePath)
}
