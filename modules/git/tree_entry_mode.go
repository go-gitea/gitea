// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strconv"
)

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeNoEntry is possible if the file was added or removed in a commit. In the case of
	// when adding the base commit doesn't have the file in its tree, a mode of 0o000000 is used.
	EntryModeNoEntry EntryMode = 0o000000

	EntryModeBlob    EntryMode = 0o100644
	EntryModeExec    EntryMode = 0o100755
	EntryModeSymlink EntryMode = 0o120000
	EntryModeCommit  EntryMode = 0o160000
	EntryModeTree    EntryMode = 0o040000
)

// String converts an EntryMode to a string
func (e EntryMode) String() string {
	return strconv.FormatInt(int64(e), 8)
}

// IsSubModule if the entry is a submodule
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

func ParseEntryMode(mode string) EntryMode {
	switch mode {
	case "000000":
		return EntryModeNoEntry
	case "100644":
		return EntryModeBlob
	case "100755":
		return EntryModeExec
	case "120000":
		return EntryModeSymlink
	case "160000":
		return EntryModeCommit
	case "040000":
		return EntryModeTree
	default:
		// git uses 040000 for tree object, but some users may get 040755 from non-standard git implementations
		m, _ := strconv.ParseInt(mode, 8, 32)
		modeInt := EntryMode(m)
		switch modeInt & 0o770000 {
		case 0o040000:
			return EntryModeTree
		case 0o160000:
			return EntryModeCommit
		case 0o120000:
			return EntryModeSymlink
		case 0o100000:
			if modeInt&0o777 == 0o755 {
				return EntryModeExec
			}
			return EntryModeBlob
		default:
			return EntryModeNoEntry
		}
	}
}
