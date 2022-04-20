// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
	"sync"
)

type closerError struct {
	io.Closer
	err  error
	lock sync.Mutex
}

func (c *closerError) CloseWithError(err error) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.err != nil {
		return c.err
	}
	if c.Closer != nil {
		_ = c.Closer.Close()
	}
	if err == nil {
		err = io.ErrClosedPipe
	}
	c.err = err
	return nil
}

func (c *closerError) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.err != nil {
		return c.err
	}
	c.err = c.Closer.Close()
	return c.err
}

// WriteCloserError wraps an io.WriteCloser with an additional CloseWithError function
type WriteCloserError interface {
	io.WriteCloser
	CloseWithError(err error) error
}

type writeCloserError struct {
	io.Writer
	closerError
}

func (c *writeCloserError) Write(p []byte) (n int, err error) {
	c.lock.Lock()
	if c.err != nil {
		return 0, c.err
	}
	c.lock.Unlock() // Unlock here to prevent hanging writes causing a deadlock in close
	n, err = c.Writer.Write(p)
	return
}

func newWriteCloserError(w io.WriteCloser) WriteCloserError {
	return &writeCloserError{
		Writer: w,
		closerError: closerError{
			Closer: w,
		},
	}
}

// ReadCloserError wraps an io.ReadCloser with an additional CloseWithError function
type ReadCloserError interface {
	io.ReadCloser
	CloseWithError(err error) error
}

type readCloserError struct {
	io.Reader
	closerError
}

func (c *readCloserError) Read(p []byte) (n int, err error) {
	c.lock.Lock()
	if c.err != nil {
		return 0, c.err
	}
	c.lock.Unlock() // Unlock here to prevent hanging reads causing a deadlock in close
	n, err = c.Reader.Read(p)
	return
}

func newReadCloserError(r io.ReadCloser) ReadCloserError {
	return &readCloserError{
		Reader: r,
		closerError: closerError{
			Closer: r,
		},
	}
}
