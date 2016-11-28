package deadline

import (
	"io"
	"time"
)

type DeadlineReader interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

type DeadlineWriter interface {
	io.Writer
	SetWriteDeadline(t time.Time) error
}

type DeadlineReadWriter interface {
	io.ReadWriter
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type deadlineReader struct {
	DeadlineReader
	timeout time.Duration
}

func (r *deadlineReader) Read(p []byte) (int, error) {
	r.DeadlineReader.SetReadDeadline(time.Now().Add(r.timeout))
	return r.DeadlineReader.Read(p)
}

func NewDeadlineReader(r DeadlineReader, timeout time.Duration) io.Reader {
	return &deadlineReader{DeadlineReader: r, timeout: timeout}
}

type deadlineWriter struct {
	DeadlineWriter
	timeout time.Duration
}

func (r *deadlineWriter) Write(p []byte) (int, error) {
	r.DeadlineWriter.SetWriteDeadline(time.Now().Add(r.timeout))
	return r.DeadlineWriter.Write(p)
}

func NewDeadlineWriter(r DeadlineWriter, timeout time.Duration) io.Writer {
	return &deadlineWriter{DeadlineWriter: r, timeout: timeout}
}
