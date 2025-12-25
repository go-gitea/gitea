// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"io"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-git/go-git/v5/plumbing"
)

// Blob represents a Git object.
type Blob struct {
	ID   ObjectID
	repo *Repository
	name string
}

func (b *Blob) gogitEncodedObj() (plumbing.EncodedObject, error) {
	return b.repo.gogitRepo.Storer.EncodedObject(plumbing.AnyObject, plumbing.Hash(b.ID.RawValue()))
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	obj, err := b.gogitEncodedObj()
	if err != nil {
		return nil, err
	}
	return obj.Reader()
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	obj, err := b.gogitEncodedObj()
	if err != nil {
		log.Error("Error getting gogit encoded object for blob %s(%s): %v", b.name, b.ID.String(), err)
		return 0
	}
	return obj.Size()
}
