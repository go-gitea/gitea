// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
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
