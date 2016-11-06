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
	stdout, err := NewCommand("show", b.ID.String()).RunInDirBytes(b.repo.Path)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(stdout), nil
}

func (b *Blob) DataPipeline(stdout, stderr io.Writer) error {
	return NewCommand("show", b.ID.String()).RunInDirPipeline(b.repo.Path, stdout, stderr)
}
