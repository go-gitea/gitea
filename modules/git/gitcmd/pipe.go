// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"io"
	"os"
)

type PipeBufferReader interface {
	Read(p []byte) (n int, err error)
	Bytes() []byte
}

type PipeBufferWriter interface {
	Write(p []byte) (n int, err error)
	Bytes() []byte
}

type PipeReader interface {
	io.ReadCloser
	internalOnly()
}

type pipeReader struct {
	f *os.File
}

func (r *pipeReader) internalOnly() {}

func (r *pipeReader) Read(p []byte) (n int, err error) {
	return r.f.Read(p)
}

func (r *pipeReader) Close() error {
	return r.f.Close()
}

type PipeWriter interface {
	io.WriteCloser
	internalOnly()
}

type pipeWriter struct {
	f *os.File
}

func (w *pipeWriter) internalOnly() {}

func (w *pipeWriter) Close() error {
	return w.f.Close()
}

func (w *pipeWriter) Write(p []byte) (n int, err error) {
	return w.f.Write(p)
}

func (w *pipeWriter) DrainBeforeClose() error {
	return nil
}

type pipeNull struct {
	err error
}

func (p *pipeNull) internalOnly() {}

func (p *pipeNull) Read([]byte) (n int, err error) {
	return 0, p.err
}

func (p *pipeNull) Write([]byte) (n int, err error) {
	return 0, p.err
}

func (p *pipeNull) Close() error {
	return nil
}
