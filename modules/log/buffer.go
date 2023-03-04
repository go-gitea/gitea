// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"sync"
)

type bufferWriteCloser struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *bufferWriteCloser) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *bufferWriteCloser) Close() error {
	return nil
}

func (b *bufferWriteCloser) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// BufferLogger implements LoggerProvider and writes messages in a buffer.
type BufferLogger struct {
	WriterLogger
}

// NewBufferLogger create BufferLogger returning as LoggerProvider.
func NewBufferLogger() LoggerProvider {
	log := &BufferLogger{}
	log.NewWriterLogger(&bufferWriteCloser{})
	return log
}

// Init inits connection writer
func (log *BufferLogger) Init(string) error {
	log.NewWriterLogger(log.out)
	return nil
}

// Content returns the content accumulated in the content provider
func (log *BufferLogger) Content() (string, error) {
	return log.out.(*bufferWriteCloser).String(), nil
}

// Flush when log should be flushed
func (log *BufferLogger) Flush() {
}

// ReleaseReopen does nothing
func (log *BufferLogger) ReleaseReopen() error {
	return nil
}

// GetName returns the default name for this implementation
func (log *BufferLogger) GetName() string {
	return "buffer"
}

func init() {
	Register("buffer", NewBufferLogger)
}
