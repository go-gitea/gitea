// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
)

var pre = "\033["
var reset = "\033[0m"

var colors = []string{
	"1;36m", // Trace      cyan
	"1;34m", // Debug      blue
	"1;32m", // Info       green
	"1;33m", // Warn       yellow
	"1;31m", // Error      red
	"1;35m", // Critical   purple
	"1;31m", // Fatal      red
}

type nopWriteCloser struct {
	w io.WriteCloser
}

func (n *nopWriteCloser) Write(p []byte) (int, error) {
	return n.w.Write(p)
}

func (n *nopWriteCloser) Close() error {
	return nil
}

// ConsoleLogger implements LoggerProvider and writes messages to terminal.
type ConsoleLogger struct {
	BaseLogger
}

// NewConsoleLogger create ConsoleLogger returning as LoggerProvider.
func NewConsoleLogger() LoggerProvider {
	log := &ConsoleLogger{}
	log.createLogger(&nopWriteCloser{
		w: os.Stdout,
	})
	return log
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *ConsoleLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), log)
	if err != nil {
		return err
	}
	log.createLogger(log.out)
	return nil
}

// LogEvent overrides the base event to add coloring
func (log *ConsoleLogger) LogEvent(event *Event) error {
	if log.Level > event.level {
		return nil
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	if !log.Match(event) {
		return nil
	}
	var buf []byte
	if runtime.GOOS != "windows" {
		buf = append(buf, pre...)
		buf = append(buf, colors[event.level]...)
	}
	log.createMsg(&buf, event)
	if runtime.GOOS != "windows" {
		buf = append(buf, reset...)
	}
	_, err := log.out.Write(buf)
	return err
}

// Flush when log should be flushed
func (log *ConsoleLogger) Flush() {
}

// GetName returns the default name for this implementation
func (log *ConsoleLogger) GetName() string {
	return "console"
}

func init() {
	Register("console", NewConsoleLogger)
}
