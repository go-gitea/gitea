// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/util"
)

const arMagic = "!<arch>\n"

type arReader struct {
	io.LimitedReader
	pad int64
}

func newArReader(r io.Reader) (*arReader, error) {
	var magic [len(arMagic)]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil || string(magic[:]) != arMagic {
		return nil, GlobalVars().ErrInvalidArchive
	}
	return &arReader{LimitedReader: io.LimitedReader{R: r}}, nil
}

func (ar *arReader) next() (string, error) {
	if _, err := io.CopyN(io.Discard, ar.R, ar.N+ar.pad); err != nil {
		return "", GlobalVars().ErrInvalidArchive
	}
	var header [60]byte
	if _, err := io.ReadFull(ar.R, header[:]); err != nil {
		return "", util.Iif(err == io.EOF, err, GlobalVars().ErrInvalidArchive)
	}
	size, err := strconv.ParseInt(strings.TrimRight(string(header[48:58]), " "), 10, 64)
	if err != nil || size < 0 || string(header[58:]) != "`\n" {
		return "", GlobalVars().ErrInvalidArchive
	}
	ar.N, ar.pad = size, size%2
	return strings.TrimSuffix(strings.TrimRight(string(header[:16]), " "), "/"), nil // GNU ar terminates names with a slash
}
