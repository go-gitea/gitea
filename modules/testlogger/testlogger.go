// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package testlogger

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/util"
)

var (
	prefix        string
	TestTimeout   = 10 * time.Minute
	TestSlowRun   = 10 * time.Second
	TestSlowFlush = 1 * time.Second
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

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...any) {
	if !log.CanColorStdout {
		for i := 0; i < len(args); i++ {
			if c, ok := args[i].(*log.ColoredValue); ok {
				args[i] = c.Value()
			}
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, format, args...)
}

// PrintCurrentTest prints the current test to os.Stdout
func PrintCurrentTest(t testing.TB, skip ...int) func() {
	t.Helper()
	runStart := time.Now()
	actualSkip := util.OptionalArg(skip) + 1
	_, filename, line, _ := runtime.Caller(actualSkip)

	Printf("=== %s (%s:%d)\n", log.NewColoredValue(t.Name()), strings.TrimPrefix(filename, prefix), line)

	WriterCloser.pushT(t)
	timeoutChecker := time.AfterFunc(TestTimeout, func() {
		l := 128 * 1024
		var stack []byte
		for {
			stack = make([]byte, l)
			n := runtime.Stack(stack, true)
			if n <= l {
				stack = stack[:n]
				break
			}
			l = n
		}
		Printf("!!! %s ... timeout: %v ... stacktrace:\n%s\n\n", log.NewColoredValue(t.Name(), log.Bold, log.FgRed), TestTimeout, string(stack))
	})
	return func() {
		flushStart := time.Now()
		slowFlushChecker := time.AfterFunc(TestSlowFlush, func() {
			Printf("+++ %s ... still flushing after %v ...\n", log.NewColoredValue(t.Name(), log.Bold, log.FgRed), TestSlowFlush)
		})
		if err := queue.GetManager().FlushAll(t.Context(), -1); err != nil {
			t.Errorf("Flushing queues failed with error %v", err)
		}
		slowFlushChecker.Stop()
		timeoutChecker.Stop()

		runDuration := time.Since(runStart)
		flushDuration := time.Since(flushStart)
		if runDuration > TestSlowRun {
			Printf("+++ %s is a slow test (run: %v, flush: %v)\n", log.NewColoredValue(t.Name(), log.Bold, log.FgYellow), runDuration, flushDuration)
		}
		WriterCloser.popT()
	}
}

// TestLogEventWriter is a logger which will write to the testing log
type TestLogEventWriter struct {
	*log.EventWriterBaseImpl
}

// newTestLoggerWriter creates a TestLogEventWriter as a log.LoggerProvider
func newTestLoggerWriter(name string, mode log.WriterMode) log.EventWriter {
	w := &TestLogEventWriter{}
	w.EventWriterBaseImpl = log.NewEventWriterBase(name, "test-log-writer", mode)
	w.OutputWriteCloser = WriterCloser
	return w
}

func Init() {
	const relFilePath = "modules/testlogger/testlogger.go"
	_, filename, _, _ := runtime.Caller(0)
	if !strings.HasSuffix(filename, relFilePath) {
		panic("source code file path doesn't match expected: " + relFilePath)
	}
	prefix = strings.TrimSuffix(filename, relFilePath)

	log.RegisterEventWriter("test", newTestLoggerWriter)

	duration, err := time.ParseDuration(os.Getenv("GITEA_TEST_SLOW_RUN"))
	if err == nil && duration > 0 {
		TestSlowRun = duration
	}

	duration, err = time.ParseDuration(os.Getenv("GITEA_TEST_SLOW_FLUSH"))
	if err == nil && duration > 0 {
		TestSlowFlush = duration
	}
}

func Fatalf(format string, args ...any) {
	Printf(format+"\n", args...)
	os.Exit(1)
}
