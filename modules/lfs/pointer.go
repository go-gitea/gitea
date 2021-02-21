// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"io"
	"strconv"
	"strings"
)

const (
	blobSizeCutoff = 1024

	// MetaFileIdentifier is the string appearing at the first line of LFS pointer files.
	// https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
	MetaFileIdentifier = "version https://git-lfs.github.com/spec/v1"

	// MetaFileOidPrefix appears in LFS pointer files on a line before the sha256 hash.
	MetaFileOidPrefix = "oid sha256:"
)

// TryReadPointer tries to read LFS pointer data from the reader
func TryReadPointer(reader io.Reader) *Pointer {
	buf := make([]byte, blobSizeCutoff)
	n, _ := io.ReadFull(reader, buf)
	buf = buf[:n]

	return TryReadPointerFromBuffer(buf)
}

// TryReadPointerFromBuffer will return a pointer if the provided byte slice is a pointer file or nil otherwise.
func TryReadPointerFromBuffer(buf []byte) *Pointer {
	headString := string(buf)
	if !strings.HasPrefix(headString, MetaFileIdentifier) {
		return nil
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return nil
	}

	oid := strings.TrimPrefix(splitLines[1], MetaFileOidPrefix)
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if len(oid) != 64 || err != nil {
		return nil
	}

	return &Pointer{Oid: oid, Size: size}
}
