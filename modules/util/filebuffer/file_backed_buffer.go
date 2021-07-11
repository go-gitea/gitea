// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package filebuffer

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"os"
)

var (
	// ErrInvalidMemorySize occurs if the memory size is not in a valid range
	ErrInvalidMemorySize = errors.New("Memory size must be greater 0 and lower math.MaxInt32")
)

// FileBackedBuffer implements io.ReadSeekCloser
type FileBackedBuffer struct {
	size    int64
	buffer  *bytes.Reader
	tmpFile *os.File
}

// CreateFromReader creates a file backed buffer which uses a buffer with a fixed memory size.
// If more data is available a temporary file is used to store the data.
func CreateFromReader(r io.Reader, maxMemorySize int) (*FileBackedBuffer, error) {
	if maxMemorySize < 0 || maxMemorySize == math.MaxInt32 {
		return nil, ErrInvalidMemorySize
	}

	var buf bytes.Buffer
	n, err := io.CopyN(&buf, r, int64(maxMemorySize+1))
	if err == io.EOF {
		return &FileBackedBuffer{
			size:   n,
			buffer: bytes.NewReader(buf.Bytes()),
		}, nil
	}
	file, err := ioutil.TempFile("", "buffer-")
	if err != nil {
		return nil, err
	}

	n, err = io.Copy(file, io.MultiReader(&buf, r))
	if err != nil {
		return nil, err
	}

	return &FileBackedBuffer{
		size:    n,
		tmpFile: file,
	}, nil
}

// Size returns the byte size of the buffered data
func (b *FileBackedBuffer) Size() int64 {
	return b.size
}

// Read implements io.Reader
func (b *FileBackedBuffer) Read(p []byte) (int, error) {
	if b.tmpFile != nil {
		return b.tmpFile.Read(p)
	}
	return b.buffer.Read(p)
}

// Seek implements io.Seeker
func (b *FileBackedBuffer) Seek(offset int64, whence int) (int64, error) {
	if b.tmpFile != nil {
		return b.tmpFile.Seek(offset, whence)
	}
	return b.buffer.Seek(offset, whence)
}

// Close implements io.Closer
func (b *FileBackedBuffer) Close() error {
	if b.tmpFile != nil {
		err := b.tmpFile.Close()
		os.Remove(b.tmpFile.Name())
		return err
	}
	return nil
}
