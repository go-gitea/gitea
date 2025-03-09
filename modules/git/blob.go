// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"

	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

// This file contains common functions between the gogit and !gogit variants for git Blobs

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// GetBlobContent Gets the limited content of the blob as raw text
func (b *Blob) GetBlobContent(limit int64) (string, error) {
	if limit <= 0 {
		return "", nil
	}
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()
	buf, err := util.ReadWithLimit(dataRc, int(limit))
	return string(buf), err
}

// GetBlobLineCount gets line count of the blob.
// It will also try to write the content to w if it's not nil, then we could pre-fetch the content without reading it again.
func (b *Blob) GetBlobLineCount(w io.Writer) (int, error) {
	reader, err := b.DataAsync()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	buf := make([]byte, 32*1024)
	count := 1
	lineSep := []byte{'\n'}
	for {
		c, err := reader.Read(buf)
		if w != nil {
			if _, err := w.Write(buf[:c]); err != nil {
				return count, err
			}
		}
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case errors.Is(err, io.EOF):
			return count, nil
		case err != nil:
			return count, err
		}
	}
}

// GetBlobContentBase64 Reads the content of the blob with a base64 encode and returns the encoded string
func (b *Blob) GetBlobContentBase64() (string, error) {
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	pr, pw := io.Pipe()
	encoder := base64.NewEncoder(base64.StdEncoding, pw)

	go func() {
		_, err := io.Copy(encoder, dataRc)
		_ = encoder.Close()

		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	out, err := io.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GuessContentType guesses the content type of the blob.
func (b *Blob) GuessContentType() (typesniffer.SniffedType, error) {
	r, err := b.DataAsync()
	if err != nil {
		return typesniffer.SniffedType{}, err
	}
	defer r.Close()

	return typesniffer.DetectContentTypeFromReader(r)
}
