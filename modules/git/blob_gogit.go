// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"io"

	"github.com/go-git/go-git/v5/plumbing"
)

// Blob represents a Git object.
type Blob struct {
	ID ObjectID

	gogitEncodedObj plumbing.EncodedObject
	name            string
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	return b.gogitEncodedObj.Reader()
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	return b.gogitEncodedObj.Size()
}
