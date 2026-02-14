// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"os"

	"code.gitea.io/gitea/modules/util"
)

type WriterConsoleOption struct {
	Stderr bool
}

type eventWriterConsole struct {
	*EventWriterBaseImpl
}

var _ EventWriter = (*eventWriterConsole)(nil)

func NewEventWriterConsole(name string, mode WriterMode) EventWriter {
	w := &eventWriterConsole{EventWriterBaseImpl: NewEventWriterBase(name, "console", mode)}
	opt := mode.WriterOption.(WriterConsoleOption)
	if opt.Stderr {
		w.OutputWriteCloser = util.NopCloser{Writer: os.Stderr}
	} else {
		w.OutputWriteCloser = util.NopCloser{Writer: os.Stdout}
	}
	return w
}

func init() {
	RegisterEventWriter("console", NewEventWriterConsole)
}
