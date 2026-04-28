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
	"code.gitea.io/gitea/modules/util"
)

var (
	prefix        string
	TestTimeout   = 10 * time.Minute
	TestSlowRun   = 10 * time.Second
	TestSlowFlush = 1 * time.Second
)

var WriterCloser = &testLoggerWriterCloser{captures: map[testing.TB]*[]byte{}}

// testLoggerWriterCloser receives log lines and captures them per-test instead
// of forwarding to t.Log. The captured logs are dumped on test failure (see
// PrintCurrentTest); for passing tests they are dropped. This keeps `go test
// -v` output readable: progress markers and init logs still stream through
// os.Stdout, but per-test log noise stays out of the way unless something
// fails.
type testLoggerWriterCloser struct {
	sync.Mutex
	stack    []testing.TB
	captures map[testing.TB]*[]byte
}

func (w *testLoggerWriterCloser) pushT(t testing.TB) {
	w.Lock()
	w.stack = append(w.stack, t)
	if _, ok := w.captures[t]; !ok {
		buf := make([]byte, 0, 1024)
		w.captures[t] = &buf
	}
	w.Unlock()
}

func (w *testLoggerWriterCloser) Write(p []byte) (int, error) {
	// There was a data race problem: the logger system could still try to output logs after the runner is finished.
	// So we must ensure that the "t" in stack is still valid.
	w.Lock()
	defer w.Unlock()

	var t testing.TB
	var buf *[]byte
	if len(w.stack) > 0 {
		t = w.stack[len(w.stack)-1]
		buf = w.captures[t]
	}

	if len(p) > 0 && p[len(p)-1] == '\n' {
		p = p[:len(p)-1]
	}

	if t == nil {
		// if there is no running test, the log message should be outputted to console, to avoid losing important information.
		// the "???" prefix is used to match the "===" and "+++" in PrintCurrentTest
		return fmt.Fprintf(os.Stdout, "??? [TestLogger] %s\n", p)
	}

	if buf != nil {
		*buf = append(*buf, p...)
		*buf = append(*buf, '\n')
	}
	return len(p), nil
}

// popT pops the topmost test off the stack and returns its captured log
// lines. The capture is cleared.
func (w *testLoggerWriterCloser) popT() []byte {
	w.Lock()
	defer w.Unlock()
	if len(w.stack) == 0 {
		return nil
	}
	t := w.stack[len(w.stack)-1]
	w.stack = w.stack[:len(w.stack)-1]
	if buf, ok := w.captures[t]; ok {
		delete(w.captures, t)
		return *buf
	}
	return nil
}

func (w *testLoggerWriterCloser) Close() error {
	return nil
}

func (w *testLoggerWriterCloser) Reset() {
	w.Lock()
	for _, t := range w.stack {
		if t == nil {
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "Unclosed logger writer in test: %s", t.Name())
		t.Errorf("Unclosed logger writer in test: %s", t.Name())
	}
	w.stack = nil
	w.captures = map[testing.TB]*[]byte{}
	w.Unlock()
}

// stdoutPrintf takes a format and args and prints the string to os.Stdout
func stdoutPrintf(format string, args ...any) {
	if !log.CanColorStdout {
		for i := range args {
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

	getRuntimeStackAll := func() string {
		stack := make([]byte, 1024*1024)
		n := runtime.Stack(stack, true)
		return util.UnsafeBytesToString(stack[:n])
	}

	deferHasRun := false
	t.Cleanup(func() {
		if !deferHasRun {
			stdoutPrintf("!!! %s: defer function hasn't been run but Cleanup is called, usually caused by panic\n", t.Name())
		}
	})
	stdoutPrintf("=== %s (%s:%d)\n", log.NewColoredValue(t.Name()), strings.TrimPrefix(filename, prefix), line)

	WriterCloser.pushT(t)
	timeoutChecker := time.AfterFunc(TestTimeout, func() {
		stdoutPrintf("!!! %s ... timeout: %v ... stacktrace:\n%s\n\n", log.NewColoredValue(t.Name(), log.Bold, log.FgRed), TestTimeout, getRuntimeStackAll())
	})
	return func() {
		deferHasRun = true
		flushStart := time.Now()
		slowFlushChecker := time.AfterFunc(TestSlowFlush, func() {
			stdoutPrintf("+++ %s ... still flushing after %v ...\n", log.NewColoredValue(t.Name(), log.Bold, log.FgRed), TestSlowFlush)
		})
		if err := queue.GetManager().FlushAll(t.Context(), -1); err != nil {
			// if panic occurs, then the t.Context() is also cancelled ahead, so here it shows "context canceled" error.
			t.Errorf("Flushing queues failed with error %q, cause %q", err, context.Cause(t.Context()))
		}
		slowFlushChecker.Stop()
		timeoutChecker.Stop()

		runDuration := time.Since(runStart)
		flushDuration := time.Since(flushStart)
		if runDuration > TestSlowRun {
			stdoutPrintf("+++ %s is a slow test (run: %v, flush: %v)\n", log.NewColoredValue(t.Name(), log.Bold, log.FgYellow), runDuration, flushDuration)
		}
		captured := WriterCloser.popT()
		if t.Failed() && len(captured) > 0 {
			stdoutPrintf("!!! %s captured logs:\n%s", log.NewColoredValue(t.Name(), log.Bold, log.FgRed), captured)
		}
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
}

// MainErrorf is used to report an error from TestMain and return a non-zero value to indicate the failure
func MainErrorf(msg string, a ...any) int {
	_, _ = fmt.Fprintf(os.Stderr, msg+"\n", a...)
	return 1
}
