// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
	"os"
	"sync"
)

// PipePair represents an os.Pipe() wrapped with a io.Pipe()-like PipeWriter and PipeReader
type PipePair struct {
	rd *os.File
	wr *os.File

	lock  sync.Mutex
	wrErr error
	rdErr error
}

// ReaderWriter returns the Reader and Writer ends of the Pipe
func (p *PipePair) ReaderWriter() (*PipeReader, *PipeWriter) {
	return p.Reader(), p.Writer()
}

// Reader returns the Reader end of the Pipe
func (p *PipePair) Reader() *PipeReader {
	return &PipeReader{p}
}

func (p *PipePair) read(b []byte) (n int, err error) {
	n, err = p.rd.Read(b)
	if err != nil {
		return n, p.readCloseError(err)
	}
	return
}

func (p *PipePair) closeRead(err error) error {
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
func (p *PipePair) readCloseError(err error) error {
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

// Writer returns the Writer end of the Pipe
func (p *PipePair) Writer() *PipeWriter {
	return &PipeWriter{p}
}

func (p *PipePair) write(b []byte) (n int, err error) {
	n, err = p.wr.Write(b)
	if err != nil {
		return n, p.writeCloseError(err)
	}
	return
}

func (p *PipePair) closeWrite(err error) error {
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
func (p *PipePair) writeCloseError(err error) error {
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

// Close closes the pipe pair
func (p *PipePair) Close() error {
	return p.CloseWithError(nil)
}

// CloseWithError closes the pipe pair
func (p *PipePair) CloseWithError(err error) error {
	_ = p.closeRead(err)
	_ = p.closeWrite(err)
	return nil
}

// PipeReader is the read half of a pipe
type PipeReader struct {
	p *PipePair
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
	p *PipePair
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

// Pipe returns a connected pair of Files wrapped with CloserError similar to io.Pipe().
// Reads from r return bytes written to w. It returns the files and an error, if any.
func Pipe() (*PipeReader, *PipeWriter, error) {
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	pipe := &PipePair{rd: rd, wr: wr}

	return pipe.Reader(), pipe.Writer(), nil
}

// NewPipePair returns a connected pair of Files wrapped in a PipePair
func NewPipePair() (*PipePair, error) {
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	return &PipePair{rd: rd, wr: wr}, nil
}

// NewPipePairs will return a slice of n PipePairs or an error
func NewPipePairs(n int) (PipePairs, error) {
	pipePairs := make([]*PipePair, 0, n)
	for i := 0; i < n; i++ {
		pipe, err := NewPipePair()
		if err != nil {
			for _, pipe := range pipePairs {
				_ = pipe.Close()
			}
			return nil, err
		}

		pipePairs = append(pipePairs, pipe)
	}
	return pipePairs, nil
}

type PipePairs []*PipePair

// Close closes the pipe pairs
func (pairs PipePairs) Close() error {
	return pairs.CloseWithError(nil)
}

// CloseWithError closes the pipe pairs
func (pairs PipePairs) CloseWithError(err error) error {
	for _, p := range pairs {
		_ = p.closeRead(err)
		_ = p.closeWrite(err)
	}
	return nil
}
