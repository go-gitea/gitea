// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"io"
	"strings"
)

// CatFileBatch opens git cat-file --batch in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func CatFileBatch(repoPath string) (*io.PipeWriter, *bufio.Reader, func()) {
	// Next feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := io.Pipe()
	cancel := func() {
		_ = batchStdinReader.Close()
		_ = batchStdinWriter.Close()
		_ = batchStdoutReader.Close()
		_ = batchStdoutWriter.Close()
	}

	go func() {
		stderr := strings.Builder{}
		err := NewCommand("cat-file", "--batch").RunInDirFullPipeline(repoPath, batchStdoutWriter, &stderr, batchStdinReader)
		if err != nil {
			_ = batchStdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
			_ = batchStdinReader.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = batchStdoutWriter.Close()
			_ = batchStdinReader.Close()
		}
	}()

	// For simplicities sake we'll us a buffered reader to read from the cat-file --batch
	batchReader := bufio.NewReader(batchStdoutReader)

	return batchStdinWriter, batchReader, cancel
}
