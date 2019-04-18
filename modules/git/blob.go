// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

// Blob represents a Git object.
type Blob struct {
	repo *Repository
	*TreeEntry
}

// Data gets content of blob all at once and wrap it as io.Reader.
// This can be very slow and memory consuming for huge content.
func (b *Blob) Data() (io.Reader, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	// Preallocate memory to save ~50% memory usage on big files.
	stdout.Grow(int(b.Size() + 2048))

	if err := b.DataPipeline(stdout, stderr); err != nil {
		return nil, concatenateError(err, stderr.String())
	}
	return stdout, nil
}

// DataPipeline gets content of blob and write the result or error to stdout or stderr
func (b *Blob) DataPipeline(stdout, stderr io.Writer) error {
	return NewCommand("show", b.ID.String()).RunInDirPipeline(b.repo.Path, stdout, stderr)
}

type cmdReadCloser struct {
	cmd    *exec.Cmd
	stdout io.Reader
}

func (c cmdReadCloser) Read(p []byte) (int, error) {
	return c.stdout.Read(p)
}

func (c cmdReadCloser) Close() error {
	io.Copy(ioutil.Discard, c.stdout)
	return c.cmd.Wait()
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	cmd := exec.Command("git", "show", b.ID.String())
	cmd.Dir = b.repo.Path
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	return cmdReadCloser{stdout: stdout, cmd: cmd}, nil
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
		encoder.Close()

		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
