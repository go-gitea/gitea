// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Blob represents a Git object.
type Blob struct {
	ID SHA1

	gotSize  bool
	size     int64
	repoPath string
	name     string
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	var err error

	go func() {
		stderr := &strings.Builder{}
		err = NewCommand("cat-file", "--batch").RunInDirFullPipeline(b.repoPath, stdoutWriter, stderr, strings.NewReader(b.ID.String()+"\n"))
		if err != nil {
			err = ConcatenateError(err, stderr.String())
			_ = stdoutWriter.CloseWithError(err)
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	bufReader := bufio.NewReader(stdoutReader)
	_, _, size, err := ReadBatchLine(bufReader)
	if err != nil {
		stdoutReader.Close()
		return nil, err
	}

	return &LimitedReaderCloser{
		R: bufReader,
		C: stdoutReader,
		N: int64(size),
	}, err
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	if b.gotSize {
		return b.size
	}

	size, err := NewCommand("cat-file", "-s", b.ID.String()).RunInDir(b.repoPath)
	if err != nil {
		log("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repoPath, err)
		return 0
	}

	b.size, err = strconv.ParseInt(size[:len(size)-1], 10, 64)
	if err != nil {
		log("error whilst parsing size %s for %s in %s. Error: %v", size, b.ID.String(), b.repoPath, err)
		return 0
	}
	b.gotSize = true

	return b.size
}
