// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"

	"code.gitea.io/gitea/modules/log"
)

// Blob represents a Git object.
type Blob struct {
	ID ObjectID

	gotSize bool
	size    int64
	name    string
	repo    *Repository
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (_ io.ReadCloser, retErr error) {
	batch, cancel, err := b.repo.CatFileBatch(b.repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		// if there was an error, cancel the batch right away,
		// otherwise let the caller close it
		if retErr != nil {
			cancel()
		}
	}()

	info, contentReader, err := batch.QueryContent(b.ID.String())
	if err != nil {
		return nil, err
	}
	b.gotSize = true
	b.size = info.Size
	return &blobReader{
		rd:     contentReader,
		n:      info.Size,
		cancel: cancel,
	}, nil
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	if b.gotSize {
		return b.size
	}

	batch, cancel, err := b.repo.CatFileBatch(b.repo.Ctx)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}
	defer cancel()
	info, err := batch.QueryInfo(b.ID.String())
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}
	b.gotSize = true
	b.size = info.Size
	return b.size
}

type blobReader struct {
	rd     BufferedReader
	n      int64
	cancel func()
}

func (b *blobReader) Read(p []byte) (n int, err error) {
	if b.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.n {
		p = p[0:b.n]
	}
	n, err = b.rd.Read(p)
	b.n -= int64(n)
	return n, err
}

// Close implements io.Closer
func (b *blobReader) Close() error {
	if b.rd == nil {
		return nil
	}

	defer b.cancel()

	if err := DiscardFull(b.rd, b.n+1); err != nil {
		return err
	}

	b.rd = nil

	return nil
}
