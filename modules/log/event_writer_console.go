// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"io"
	"os"
)

type WriterConsoleOption struct {
	Stderr bool
}

type eventWriterConsole struct {
	*EventWriterBaseImpl
}

var _ EventWriter = (*eventWriterConsole)(nil)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func NewEventWriterConsole(name string, mode WriterMode) EventWriter {
	w := &eventWriterConsole{EventWriterBaseImpl: NewEventWriterBase(name, "console", mode)}
	opt := mode.WriterOption.(WriterConsoleOption)
	if opt.Stderr {
		w.OutputWriteCloser = nopCloser{os.Stderr}
	} else {
		w.OutputWriteCloser = nopCloser{os.Stdout}
	}
	return w
}

func init() {
	RegisterEventWriter("console", NewEventWriterConsole)
}
