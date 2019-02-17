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

// ConsoleLogger implements LoggerInterface and writes messages to terminal.
type ConsoleLogger struct {
	BaseLogger
}

// NewConsoleLogger create ConsoleLogger returning as LoggerInterface.
func NewConsoleLogger() LoggerInterface {
	cw := &ConsoleLogger{}
	cw.createLogger(&nopWriteCloser{
		w: os.Stdout,
	})
	return cw
}

// Init inits connection writer with json config.
// json config only need key "level".
func (cw *ConsoleLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), cw)
	if err != nil {
		return err
	}
	cw.createLogger(cw.out)
	return nil
}

// LogEvent overrides the base event to add coloring
func (cw *ConsoleLogger) LogEvent(event *Event) error {
	if cw.Level > event.level {
		return nil
	}
	cw.mu.Lock()
	defer cw.mu.Unlock()
	if !cw.Match(event) {
		return nil
	}
	var buf []byte
	if runtime.GOOS != "windows" {
		buf = append(buf, pre...)
		buf = append(buf, colors[event.level]...)
	}
	cw.createMsg(&buf, event)
	if runtime.GOOS != "windows" {
		buf = append(buf, reset...)
	}
	_, err := cw.out.Write(buf)
	return err
}

// Flush when log should be flushed
func (cw *ConsoleLogger) Flush() {
}

func init() {
	Register("console", NewConsoleLogger)
}
