// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"bytes"
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
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	wr, rd, cancel, err := b.repo.CatFileBatch(b.repo.Ctx)
	if err != nil {
		return nil, err
	}

	_, err = wr.Write([]byte(b.ID.String() + "\n"))
	if err != nil {
		cancel()
		return nil, err
	}
	_, _, size, err := ReadBatchLine(rd)
	if err != nil {
		cancel()
		return nil, err
	}
	b.gotSize = true
	b.size = size

	if size < 4096 {
		bs, err := io.ReadAll(io.LimitReader(rd, size))
		defer cancel()
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		return io.NopCloser(bytes.NewReader(bs)), err
	}

	return &blobReader{
		rd:     rd,
		n:      size,
		cancel: cancel,
	}, nil
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	if b.gotSize {
		return b.size
	}

	wr, rd, cancel, err := b.repo.CatFileBatchCheck(b.repo.Ctx)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}
	defer cancel()
	_, err = wr.Write([]byte(b.ID.String() + "\n"))
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}
	_, _, b.size, err = ReadBatchLine(rd)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}

	b.gotSize = true

	return b.size
}

type blobReader struct {
	rd     *bufio.Reader
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
