// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"io"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

type WriterFileOption struct {
	FileName         string
	MaxSize          int64
	LogRotate        bool
	DailyRotate      bool
	MaxDays          int
	Compress         bool
	CompressionLevel int
}

type eventWriterFile struct {
	*EventWriterBaseImpl
	fileWriter io.WriteCloser
}

var _ EventWriter = (*eventWriterFile)(nil)

func NewEventWriterFile(name string, mode WriterMode) EventWriter {
	w := &eventWriterFile{EventWriterBaseImpl: NewEventWriterBase(name, "file", mode)}
	opt := mode.WriterOption.(WriterFileOption)
	var err error
	w.fileWriter, err = rotatingfilewriter.Open(opt.FileName, &rotatingfilewriter.Options{
		Rotate:           opt.LogRotate,
		MaximumSize:      opt.MaxSize,
		RotateDaily:      opt.DailyRotate,
		KeepDays:         opt.MaxDays,
		Compress:         opt.Compress,
		CompressionLevel: opt.CompressionLevel,
	})
	if err != nil {
		// if the log file can't be opened, what should it do? panic/exit? ignore logs? fallback to stderr?
		// it seems that "fallback to stderr" is slightly better than others ....
		FallbackErrorf("unable to open log file %q: %v", opt.FileName, err)
		w.fileWriter = util.NopCloser{Writer: LoggerToWriter(FallbackErrorf)}
	}
	w.OutputWriteCloser = w.fileWriter
	return w
}

func init() {
	RegisterEventWriter("file", NewEventWriterFile)
}
