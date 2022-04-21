// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
	"os"
	"sync"
)

type pipe struct {
	rd *os.File
	wr *os.File

	lock  sync.Mutex
	wrErr error
	rdErr error
}

func (p *pipe) read(b []byte) (n int, err error) {
	n, err = p.rd.Read(b)
	if err != nil {
		return n, p.readCloseError(err)
	}
	return
}

func (p *pipe) closeRead(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.rdErr != nil {
		return nil
	}

	p.rdErr = err
	_ = p.rd.Close()
	return nil
}

// readCloseError returns the error returned on reading a closed read pipe
func (p *pipe) readCloseError(err error) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	rdErr := p.rdErr

	if wrErr := p.wrErr; rdErr == nil && wrErr != nil {
		return wrErr
	}
	if err != io.EOF {
		return err
	}
	return io.ErrClosedPipe
}

func (p *pipe) write(b []byte) (n int, err error) {
	n, err = p.wr.Write(b)
	if err != nil {
		return n, p.writeCloseError(err)
	}
	return
}

func (p *pipe) closeWrite(err error) error {
	if err == nil {
		err = io.EOF
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.wrErr != nil {
		return nil
	}
	p.wrErr = err
	_ = p.wr.Close()
	return nil
}

// writeCloseError returns the error returned on writing to a closed write pipe
func (p *pipe) writeCloseError(err error) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	wrErr := p.wrErr
	if rdErr := p.rdErr; wrErr == nil && rdErr != nil {
		return rdErr
	}
	if err != io.EOF {
		return err
	}
	return io.ErrClosedPipe
}

// PipeReader is the read half of a pipe
type PipeReader struct {
	p *pipe
}

// Read implements the standard Read interface.
// If the write end is closed with an error, that error is
// returned as err; otherwise err is EOF.
func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.p.read(data)
}

// Close closes the reader; subsequent writes to the
// write half of the pipe will return the error ErrClosedPipe.
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes
// to the write half of the pipe will return the error err.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (r *PipeReader) CloseWithError(err error) error {
	return r.p.closeRead(err)
}

// File returns the underlying os.File for this PipeReader
func (r *PipeReader) File() *os.File {
	return r.p.rd
}

// PipeWriter is the write half of a pipe.
type PipeWriter struct {
	p *pipe
}

// Write implements the standard Write interface:
// If the read end is closed with an error, that err is
// returned as err; otherwise err is ErrClosedPipe.
func (w *PipeWriter) Write(data []byte) (n int, err error) {
	return w.p.write(data)
}

// Close closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and EOF.
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and the error err,
// or EOF if err is nil.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (w *PipeWriter) CloseWithError(err error) error {
	return w.p.closeWrite(err)
}

// File returns the underlying os.File for this PipeWriter
func (w *PipeWriter) File() *os.File {
	return w.p.wr
}

func Pipe() (*PipeReader, *PipeWriter, error) {
	p := &pipe{}
	var err error

	p.rd, p.wr, err = os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	return &PipeReader{p}, &PipeWriter{p}, nil
}
