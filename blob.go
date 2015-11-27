// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
)

type Blob struct {
	repo *Repository
	*TreeEntry
}

func (b *Blob) Data() (io.Reader, error) {
	stdout, err := NewCommand("show", b.ID.String()).RunInDirBytes(b.repo.Path)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(stdout), nil
}
