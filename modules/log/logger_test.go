// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type dummyWriter struct {
	*EventWriterBaseImpl

	delay time.Duration

	mu   sync.Mutex
	logs []string
}

func (d *dummyWriter) Write(p []byte) (n int, err error) {
	if d.delay > 0 {
		time.Sleep(d.delay)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logs = append(d.logs, string(p))
	return len(p), nil
}

func (d *dummyWriter) Close() error {
	return nil
}

func (d *dummyWriter) GetLogs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	logs := make([]string, len(d.logs))
	copy(logs, d.logs)
	return logs
}

func newDummyWriter(name string, level Level, delay time.Duration) *dummyWriter {
	w := &dummyWriter{
		EventWriterBaseImpl: NewEventWriterBase(name, "dummy", WriterMode{Level: level, Flags: FlagsFromBits(0)}),
	}
	w.delay = delay
	w.Base().OutputWriteCloser = w
	return w
}

func TestLogger(t *testing.T) {
	logger := NewLoggerWithWriters(context.Background(), "test")

	dump := logger.DumpWriters()
	assert.EqualValues(t, 0, len(dump))
	assert.EqualValues(t, NONE, logger.GetLevel())
	assert.False(t, logger.IsEnabled())

	w1 := newDummyWriter("dummy-1", DEBUG, 0)
	logger.AddWriters(w1)
	assert.EqualValues(t, DEBUG, logger.GetLevel())

	w2 := newDummyWriter("dummy-2", WARN, 200*time.Millisecond)
	logger.AddWriters(w2)
	assert.EqualValues(t, DEBUG, logger.GetLevel())

	dump = logger.DumpWriters()
	assert.EqualValues(t, 2, len(dump))

	logger.Trace("trace-level") // this level is not logged
	logger.Debug("debug-level")
	logger.Error("error-level")

	// w2 is slow, so only w1 has logs
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, []string{"debug-level\n", "error-level\n"}, w1.GetLogs())
	assert.Equal(t, []string{}, w2.GetLogs())

	logger.Close()

	// after Close, all logs are flushed
	assert.Equal(t, []string{"debug-level\n", "error-level\n"}, w1.GetLogs())
	assert.Equal(t, []string{"error-level\n"}, w2.GetLogs())
}

func TestLoggerPause(t *testing.T) {
	logger := NewLoggerWithWriters(context.Background(), "test")

	w1 := newDummyWriter("dummy-1", DEBUG, 0)
	logger.AddWriters(w1)

	GetManager().PauseAll()
	time.Sleep(50 * time.Millisecond)

	logger.Info("info-level")
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, []string{}, w1.GetLogs())

	GetManager().ResumeAll()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, []string{"info-level\n"}, w1.GetLogs())

	logger.Close()
}

type testLogString struct {
	Field string
}

func (t testLogString) LogString() string {
	return "log-string"
}

func TestLoggerLogString(t *testing.T) {
	logger := NewLoggerWithWriters(context.Background(), "test")

	w1 := newDummyWriter("dummy-1", DEBUG, 0)
	w1.Mode.Colorize = true
	logger.AddWriters(w1)

	logger.Info("%s %s %#v %v", testLogString{}, &testLogString{}, testLogString{Field: "detail"}, NewColoredValue(testLogString{}, FgRed))
	logger.Close()

	assert.Equal(t, []string{"log-string log-string log.testLogString{Field:\"detail\"} \x1b[31mlog-string\x1b[0m\n"}, w1.GetLogs())
}

func TestLoggerExpressionFilter(t *testing.T) {
	logger := NewLoggerWithWriters(context.Background(), "test")

	w1 := newDummyWriter("dummy-1", DEBUG, 0)
	w1.Mode.Expression = "foo.*"
	logger.AddWriters(w1)

	logger.Info("foo")
	logger.Info("bar")
	logger.Info("foo bar")
	logger.SendLogEvent(&Event{Level: INFO, Filename: "foo.go", MsgSimpleText: "by filename"})
	logger.Close()

	assert.Equal(t, []string{"foo\n", "foo bar\n", "by filename\n"}, w1.GetLogs())
}
