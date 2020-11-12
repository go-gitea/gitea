// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build nogogit

package git

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
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
			stdoutWriter.CloseWithError(err)
		} else {
			stdoutWriter.Close()
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

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// GetBlobContent Gets the content of the blob as raw text
func (b *Blob) GetBlobContent() (string, error) {
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]
	return string(buf), nil
}

// GetBlobLineCount gets line count of lob as raw text
func (b *Blob) GetBlobLineCount() (int, error) {
	reader, err := b.DataAsync()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}
	for {
		c, err := reader.Read(buf)
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, err
		}
	}
}

// GetBlobContentBase64 Reads the content of the blob with a base64 encode and returns the encoded string
func (b *Blob) GetBlobContentBase64() (string, error) {
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	pr, pw := io.Pipe()
	encoder := base64.NewEncoder(base64.StdEncoding, pw)

	go func() {
		_, err := io.Copy(encoder, dataRc)
		_ = encoder.Close()

		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
