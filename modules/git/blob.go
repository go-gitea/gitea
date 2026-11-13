// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// GetBlobBytes Gets the limited content of the blob
func (b *Blob) GetBlobBytes(ctx context.Context, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, nil
	}
	dataRc, err := b.DataAsync(ctx)
	if err != nil {
		return nil, err
	}
	defer dataRc.Close()
	return util.ReadWithLimit(dataRc, int(limit))
}

// GetBlobContent Gets the limited content of the blob as raw text
func (b *Blob) GetBlobContent(ctx context.Context, limit int64) (string, error) {
	buf, err := b.GetBlobBytes(ctx, limit)
	return string(buf), err
}

// GetBlobLineCount gets line count of the blob.
// It will also try to write the content to w if it's not nil, then we could pre-fetch the content without reading it again.
func (b *Blob) GetBlobLineCount(ctx context.Context, w io.Writer) (size int64, count int, _ error) {
	reader, err := b.DataAsync(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()
	return getBlobLineCount(reader, w)
}

func getBlobLineCount(r io.Reader, w io.Writer) (size int64, count int, _ error) {
	buf := make([]byte, 32*1024)
	size, count = 0, 0
	var lastChar byte
	lineSep := []byte("\n")
	for {
		c, err := r.Read(buf)
		size += int64(c)
		if w != nil {
			if _, err := w.Write(buf[:c]); err != nil {
				return size, count, err
			}
		}
		if c > 0 {
			count += bytes.Count(buf[:c], lineSep)
			lastChar = buf[c-1]
		}
		switch {
		case errors.Is(err, io.EOF):
			if size > 0 && lastChar != '\n' {
				// it should match "git diff" hunk line number. "a\nb" => 2 lines, "a\nb\n" => 2 lines
				count++
			}
			return size, count, nil
		case err != nil:
			return size, count, err
		}
	}
}

// GetBlobContentBase64 Reads the content of the blob with a base64 encoding and returns the encoded string
func (b *Blob) GetBlobContentBase64(ctx context.Context, originContent *strings.Builder) (string, error) {
	dataRc, err := b.DataAsync(ctx)
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	base64buf := &strings.Builder{}
	encoder := base64.NewEncoder(base64.StdEncoding, base64buf)
	buf := make([]byte, 32*1024)
loop:
	for {
		n, err := dataRc.Read(buf)
		if n > 0 {
			if originContent != nil {
				_, _ = originContent.Write(buf[:n])
			}
			if _, err := encoder.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		switch {
		case errors.Is(err, io.EOF):
			break loop
		case err != nil:
			return "", err
		}
	}
	_ = encoder.Close()
	return base64buf.String(), nil
}

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
func (b *Blob) DataAsync(ctx context.Context) (_ io.ReadCloser, retErr error) {
	batch, cancel, err := b.repo.CatFileBatch()
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
func (b *Blob) Size(ctx context.Context) int64 {
	if b.gotSize {
		return b.size
	}

	batch, cancel, err := b.repo.CatFileBatch()
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.LogString(), err)
		return 0
	}
	defer cancel()
	info, err := batch.QueryInfo(b.ID.String())
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.LogString(), err)
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
