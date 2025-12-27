// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"errors"

	"code.gitea.io/gitea/modules/git/catfile"
)

// ReadBatchLine reads the header line from cat-file --batch while preserving the traditional return signature.
func ReadBatchLine(rd *bufio.Reader) (sha []byte, typ string, size int64, err error) {
	sha, typ, size, err = catfile.ReadBatchLine(rd)
	return sha, typ, size, convertCatfileError(err, sha)
}

// ReadTagObjectID reads a tag object ID hash from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTagObjectID(rd *bufio.Reader, size int64) (string, error) {
	return catfile.ReadTagObjectID(rd, size)
}

// ReadTreeID reads a tree ID from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTreeID(rd *bufio.Reader, size int64) (string, error) {
	return catfile.ReadTreeID(rd, size)
}

// BinToHex converts a binary hash into a hex encoded one.
func BinToHex(objectFormat ObjectFormat, sha, out []byte) []byte {
	return catfile.BinToHex(objectFormat, sha, out)
}

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream.
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd *bufio.Reader, modeBuf, fnameBuf, shaBuf []byte) (mode, fname, sha []byte, n int, err error) {
	mode, fname, sha, n, err = catfile.ParseCatFileTreeLine(objectFormat, rd, modeBuf, fnameBuf, shaBuf)
	return mode, fname, sha, n, convertCatfileError(err, nil)
}

// DiscardFull discards the requested number of bytes from the buffered reader.
func DiscardFull(rd *bufio.Reader, discard int64) error {
	return catfile.DiscardFull(rd, discard)
}

func convertCatfileError(err error, defaultID []byte) error {
	if err == nil {
		return nil
	}
	var notFound catfile.ErrObjectNotFound
	if errors.As(err, &notFound) {
		if notFound.ID == "" && len(defaultID) > 0 {
			notFound.ID = string(defaultID)
		}
		return ErrNotExist{ID: notFound.ID}
	}
	return err
}
