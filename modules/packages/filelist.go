// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import "gitea.dev/modules/util"

// Default caps for a package's accumulated file list. They sit far above any legitimate package but low
// enough to stop metadata amplification, where a tiny, highly-compressible archive (e.g. many empty tar
// entries) expands into a huge stored/indexed file list. Package parsers that build a per-file list from
// an untrusted archive should accumulate through BoundedFileList so the limit is enforced uniformly
// rather than reimplemented per parser.
const (
	DefaultMaxFileEntries   = 100000
	DefaultMaxFileNameBytes = 16 << 20
)

// BoundedFileList accumulates file names from a package archive while enforcing caps on the number of
// entries and their total name length, returning an error once either cap would be exceeded.
type BoundedFileList struct {
	files     []string
	nameBytes int
	maxFiles  int
	maxBytes  int
}

// NewBoundedFileList creates a BoundedFileList with the given caps; a non-positive cap falls back to the
// corresponding default.
func NewBoundedFileList(maxFiles, maxNameBytes int) *BoundedFileList {
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFileEntries
	}
	if maxNameBytes <= 0 {
		maxNameBytes = DefaultMaxFileNameBytes
	}
	return &BoundedFileList{maxFiles: maxFiles, maxBytes: maxNameBytes}
}

// Add appends name, returning util.ErrInvalidArgument once the entry count or accumulated byte length
// would exceed the configured cap.
func (b *BoundedFileList) Add(name string) error {
	b.nameBytes += len(name)
	if len(b.files) >= b.maxFiles || b.nameBytes > b.maxBytes {
		return util.NewInvalidArgumentErrorf("package contains too many file entries")
	}
	b.files = append(b.files, name)
	return nil
}

// Files returns the accumulated file names.
func (b *BoundedFileList) Files() []string {
	return b.files
}
