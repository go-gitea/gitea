// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "strconv"

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeBlob
	EntryModeBlob EntryMode = 0100644
	// EntryModeExec
	EntryModeExec EntryMode = 0100755
	// EntryModeSymlink
	EntryModeSymlink EntryMode = 0120000
	// EntryModeCommit
	EntryModeCommit EntryMode = 0160000
	// EntryModeTree
	EntryModeTree EntryMode = 0040000
)

// String converts an EntryMode to a string
func (e EntryMode) String() string {
	return strconv.FormatInt(int64(e), 8)
}

// ToEntryMode converts a string to an EntryMode
func ToEntryMode(value string) EntryMode {
	v, _ := strconv.ParseInt(value, 8, 32)
	return EntryMode(v)
}

// IsSubModule if the entry is a sub module
func (e EntryMode) IsSubModule() bool {
	return e == EntryModeCommit
}

// IsDir if the entry is a sub dir
func (e EntryMode) IsDir() bool {
	return e == EntryModeTree
}

// IsLink if the entry is a symlink
func (e EntryMode) IsLink() bool {
	return e == EntryModeSymlink
}

// IsRegular if the entry is a regular file
func (e EntryMode) IsRegular() bool {
	return e == EntryModeBlob
}

// IsExecutable if the entry is an executable file (not necessarily binary)
func (e EntryMode) IsExecutable() bool {
	return e == EntryModeExec
}
