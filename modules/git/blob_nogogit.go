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
// The returned reader streams the content and releases the batch when the stream ends or is closed.
// TODO: considering to remove this method or the Blob struct, so that external code could inovke batch.QueryContent directly.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	batch, cancel, err := b.repo.CatFileBatch(b.repo.Ctx)
	if err != nil {
		return nil, err
	}

	infoCh := make(chan *CatFileObject, 1)
	errCh := make(chan error, 1)
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		err := batch.QueryContent(b.ID.String(), func(info *CatFileObject, reader io.Reader) error {
			infoCh <- info
			_, copyErr := io.Copy(pipeWriter, reader)
			return copyErr
		})
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			errCh <- err
			cancel()
			return
		}
		_ = pipeWriter.Close()
		errCh <- nil
		cancel()
	}()

	var info *CatFileObject
	for info == nil {
		select {
		case info = <-infoCh:
		case err = <-errCh:
			if err != nil {
				return nil, err
			}
		}
	}

	b.gotSize = true
	b.size = info.Size
	return pipeReader, nil
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
