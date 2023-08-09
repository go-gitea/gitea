// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package filebuffer

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
)

var (
	// ErrInvalidMemorySize occurs if the memory size is not in a valid range
	ErrInvalidMemorySize = errors.New("Memory size must be greater 0 and lower math.MaxInt32")
	// ErrWriteAfterRead occurs if Write is called after a read operation
	ErrWriteAfterRead = errors.New("Write is unsupported after a read operation")
)

type readAtSeeker interface {
	io.ReadSeeker
	io.ReaderAt
}

// FileBackedBuffer uses a memory buffer with a fixed size.
// If more data is written a temporary file is used instead.
// It implements io.ReadWriteCloser, io.ReadSeekCloser and io.ReaderAt
type FileBackedBuffer struct {
	maxMemorySize int64
	size          int64
	buffer        bytes.Buffer
	file          *os.File
	reader        readAtSeeker
}

// New creates a file backed buffer with a specific maximum memory size
func New(maxMemorySize int) (*FileBackedBuffer, error) {
	if maxMemorySize < 0 || maxMemorySize > math.MaxInt32 {
		return nil, ErrInvalidMemorySize
	}

	return &FileBackedBuffer{
		maxMemorySize: int64(maxMemorySize),
	}, nil
}

// CreateFromReader creates a file backed buffer and copies the provided reader data into it.
func CreateFromReader(r io.Reader, maxMemorySize int) (*FileBackedBuffer, error) {
	b, err := New(maxMemorySize)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(b, r)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Write implements io.Writer
func (b *FileBackedBuffer) Write(p []byte) (int, error) {
	if b.reader != nil {
		return 0, ErrWriteAfterRead
	}

	var n int
	var err error

	if b.file != nil {
		n, err = b.file.Write(p)
	} else {
		if b.size+int64(len(p)) > b.maxMemorySize {
			b.file, err = os.CreateTemp("", "gitea-buffer-")
			if err != nil {
				return 0, err
			}

			_, err = io.Copy(b.file, &b.buffer)
			if err != nil {
				return 0, err
			}

			return b.Write(p)
		}

		n, err = b.buffer.Write(p)
	}

	if err != nil {
		return n, err
	}
	b.size += int64(n)
	return n, nil
}

// Size returns the byte size of the buffered data
func (b *FileBackedBuffer) Size() int64 {
	return b.size
}

func (b *FileBackedBuffer) switchToReader() error {
	if b.reader != nil {
		return nil
	}

	if b.file != nil {
		if _, err := b.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		b.reader = b.file
	} else {
		b.reader = bytes.NewReader(b.buffer.Bytes())
	}
	return nil
}

// Read implements io.Reader
func (b *FileBackedBuffer) Read(p []byte) (int, error) {
	if err := b.switchToReader(); err != nil {
		return 0, err
	}

	return b.reader.Read(p)
}

// ReadAt implements io.ReaderAt
func (b *FileBackedBuffer) ReadAt(p []byte, off int64) (int, error) {
	if err := b.switchToReader(); err != nil {
		return 0, err
	}

	return b.reader.ReadAt(p, off)
}

// Seek implements io.Seeker
func (b *FileBackedBuffer) Seek(offset int64, whence int) (int64, error) {
	if err := b.switchToReader(); err != nil {
		return 0, err
	}

	return b.reader.Seek(offset, whence)
}

// Close implements io.Closer
func (b *FileBackedBuffer) Close() error {
	if b.file != nil {
		err := b.file.Close()
		os.Remove(b.file.Name())
		return err
	}
	return nil
}
