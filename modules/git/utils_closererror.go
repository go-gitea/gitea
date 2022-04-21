// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
)

// WriteCloserError wraps an io.WriteCloser with an additional CloseWithError function
type WriteCloserError interface {
	io.WriteCloser
	CloseWithError(err error) error
}

// ReadCloserError wraps an io.ReadCloser with an additional CloseWithError function
type ReadCloserError interface {
	io.ReadCloser
	CloseWithError(err error) error
}

// CloserError wraps an io.Closer with an additional CloseWithError function
type CloserError interface {
	io.Closer
	CloseWithError(err error) error
}

// ClosedReadWriteCloserError is a pre-closed ReadWriteCloserError
type ClosedReadWriteCloserError struct {
	err error
}

func (c *ClosedReadWriteCloserError) Read(p []byte) (n int, err error) {
	return 0, c.err
}

func (c *ClosedReadWriteCloserError) Write(p []byte) (n int, err error) {
	return 0, c.err
}

func (c *ClosedReadWriteCloserError) Close() error {
	return c.err
}

func (c *ClosedReadWriteCloserError) CloseWithError(error) error {
	return c.err
}
