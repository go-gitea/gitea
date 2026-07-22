// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import "gitea.dev/modules/util"

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
	return &BoundedFileList{maxFiles: maxFiles, maxBytes: maxNameBytes}
}

// Add appends name, returning util.ErrInvalidArgument once the entry count or accumulated byte length
// would exceed the configured cap.
func (b *BoundedFileList) Add(name string) error {
	if len(b.files) >= b.maxFiles || b.nameBytes+len(name) > b.maxBytes {
		return util.NewInvalidArgumentErrorf("package contains too many file entries")
	}
	b.nameBytes += len(name)
	b.files = append(b.files, name)
	return nil
}

// Files returns the accumulated file names.
func (b *BoundedFileList) Files() []string {
	return b.files
}
