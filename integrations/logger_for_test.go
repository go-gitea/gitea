// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/log"
)

var prefix string

type TestLogger struct {
	log.WriterLogger
}

var writerCloser = &testLoggerWriterCloser{}

type testLoggerWriterCloser struct {
	t *testing.TB
}

func (w *testLoggerWriterCloser) Write(p []byte) (int, error) {
	if w.t != nil {
		if len(p) > 0 && p[len(p)-1] == '\n' {
			p = p[:len(p)-1]
		}
		(*w.t).Log(string(p))
		return len(p), nil
	}
	return len(p), nil
}

func (w *testLoggerWriterCloser) Close() error {
	return nil
}

func SetTestForLogger(t *testing.TB) {
	_, filename, line, _ := runtime.Caller(2)

	fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", log.NewColoredValue((*t).Name()), strings.TrimPrefix(filename, prefix), line)
	writerCloser.t = t
}

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
	log.NewWriterLogger(writerCloser)
	return nil
}

// Flush when log should be flushed
func (log *TestLogger) Flush() {
}

// GetName returns the default name for this implementation
func (log *TestLogger) GetName() string {
	return "test"
}

func init() {
	log.Register("test", NewTestLogger)
	_, filename, _, _ := runtime.Caller(0)
	prefix = strings.TrimSuffix(filename, "integrations/logger_for_test.go")
}
