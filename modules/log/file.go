// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"compress/gzip"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

// FileLogger implements LoggerProvider.
// It writes messages by lines limit, file size limit, or time frequency.
type FileLogger struct {
	WriterLogger

	rfw *rotatingfilewriter.RotatingFileWriter

	// The opened file
	Filename string `json:"filename"`

	// Rotate at size
	Maxsize int64 `json:"maxsize"`

	// Rotate daily
	Daily   bool `json:"daily"`
	Maxdays int  `json:"maxdays"`

	Rotate bool `json:"rotate"`

	Compress         bool `json:"compress"`
	CompressionLevel int  `json:"compressionLevel"`
}

// NewFileLogger create a FileLogger returning as LoggerProvider.
func NewFileLogger() LoggerProvider {
	log := &FileLogger{
		Filename:         "",
		Maxsize:          1 << 28, // 256 MB
		Daily:            true,
		Maxdays:          7,
		Rotate:           true,
		Compress:         true,
		CompressionLevel: gzip.DefaultCompression,
	}
	log.Level = TRACE

	return log
}

// Init file logger with json config.
// config like:
//
//	{
//	"filename":"log/gogs.log",
//	"maxsize":1<<30,
//	"daily":true,
//	"maxdays":15,
//	"rotate":true
//	}
func (log *FileLogger) Init(config string) error {
	if err := json.Unmarshal([]byte(config), log); err != nil {
		return fmt.Errorf("Unable to parse JSON: %w", err)
	}
	if len(log.Filename) == 0 {
		return errors.New("config must have filename")
	}

	rfw, err := rotatingfilewriter.Open(
		log.Filename,
		&rotatingfilewriter.Options{
			MaximumSize:      log.Maxsize,
			RotateDaily:      log.Daily,
			KeepDays:         log.Maxdays,
			Rotate:           log.Rotate,
			Compress:         log.Compress,
			CompressionLevel: log.CompressionLevel,
		},
	)
	if err != nil {
		return err
	}

	log.rfw = rfw

	log.NewWriterLogger(log.rfw)

	return nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.log.2013-01-01.2
func (log *FileLogger) DoRotate() error {
	return log.rfw.DoRotate()
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (log *FileLogger) Flush() {
	_ = log.rfw.Flush()
}

// ReleaseReopen releases and reopens log files
func (log *FileLogger) ReleaseReopen() error {
	return log.rfw.ReleaseReopen()
}

// GetName returns the default name for this implementation
func (log *FileLogger) GetName() string {
	return "file"
}

func init() {
	Register("file", NewFileLogger)
}
