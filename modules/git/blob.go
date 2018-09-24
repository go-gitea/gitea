// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
)

// Blob represents a Git object.
type Blob struct {
	repo *Repository
	*TreeEntry
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	gogitBlob, err := b.repo.gogitRepo.BlobObject(b.gogitTreeEntry.Hash)
	if err != nil {
		return nil, err
	}

	return gogitBlob.Reader()
}
