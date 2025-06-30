// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// spec: https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
const (
	MetaFileMaxSize = 1024 // spec says the maximum size of a pointer file must be smaller than 1024

	MetaFileIdentifier = "version https://git-lfs.github.com/spec/v1" // the first line of a pointer file

	MetaFileOidPrefix = "oid sha256:" // spec says the only supported hash is sha256 at the moment
)

var (
	// ErrMissingPrefix occurs if the content lacks the LFS prefix
	ErrMissingPrefix = errors.New("content lacks the LFS prefix")

	// ErrInvalidStructure occurs if the content has an invalid structure
	ErrInvalidStructure = errors.New("content has an invalid structure")

	// ErrInvalidOIDFormat occurs if the oid has an invalid format
	ErrInvalidOIDFormat = errors.New("OID has an invalid format")
)

// ReadPointer tries to read LFS pointer data from the reader
func ReadPointer(reader io.Reader) (Pointer, error) {
	buf := make([]byte, MetaFileMaxSize)
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return Pointer{}, err
	}
	buf = buf[:n]

	return ReadPointerFromBuffer(buf)
}

var oidPattern = regexp.MustCompile(`^[a-f\d]{64}$`)

// ReadPointerFromBuffer will return a pointer if the provided byte slice is a pointer file or an error otherwise.
func ReadPointerFromBuffer(buf []byte) (Pointer, error) {
	var p Pointer

	headString := string(buf)
	if !strings.HasPrefix(headString, MetaFileIdentifier) {
		return p, ErrMissingPrefix
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return p, ErrInvalidStructure
	}

	// spec says "key/value pairs MUST be sorted alphabetically in ascending order (version is exception and must be the first)"
	oid := strings.TrimPrefix(splitLines[1], MetaFileOidPrefix)
	if len(oid) != 64 || !oidPattern.MatchString(oid) {
		return p, ErrInvalidOIDFormat
	}
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if err != nil {
		return p, err
	}

	p.Oid = oid
	p.Size = size

	return p, nil
}

// IsValid checks if the pointer has a valid structure.
// It doesn't check if the pointed-to-content exists.
func (p Pointer) IsValid() bool {
	if len(p.Oid) != 64 {
		return false
	}
	if !oidPattern.MatchString(p.Oid) {
		return false
	}
	if p.Size < 0 {
		return false
	}
	return true
}

// StringContent returns the string representation of the pointer
// https://github.com/git-lfs/git-lfs/blob/main/docs/spec.md#the-pointer
func (p Pointer) StringContent() string {
	return fmt.Sprintf("%s\n%s%s\nsize %d\n", MetaFileIdentifier, MetaFileOidPrefix, p.Oid, p.Size)
}

// RelativePath returns the relative storage path of the pointer
func (p Pointer) RelativePath() string {
	if len(p.Oid) < 5 {
		return p.Oid
	}

	return path.Join(p.Oid[0:2], p.Oid[2:4], p.Oid[4:])
}

func (p Pointer) LogString() string {
	if p.Oid == "" && p.Size == 0 {
		return "<LFSPointer empty>"
	}
	return fmt.Sprintf("<LFSPointer %s:%d>", p.Oid, p.Size)
}

// GeneratePointer generates a pointer for arbitrary content
func GeneratePointer(content io.Reader) (Pointer, error) {
	h := sha256.New()
	c, err := io.Copy(h, content)
	if err != nil {
		return Pointer{}, err
	}
	sum := h.Sum(nil)
	return Pointer{Oid: hex.EncodeToString(sum), Size: c}, nil
}
