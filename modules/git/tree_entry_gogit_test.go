// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/stretchr/testify/assert"
)

func TestEntryGogit(t *testing.T) {
	cases := map[EntryMode]filemode.FileMode{
		EntryModeBlob:    filemode.Regular,
		EntryModeCommit:  filemode.Submodule,
		EntryModeExec:    filemode.Executable,
		EntryModeSymlink: filemode.Symlink,
		EntryModeTree:    filemode.Dir,
	}
	for emode, fmode := range cases {
		assert.Equal(t, fmode, entryModeToGogitFileMode(emode))
		assert.Equal(t, emode, gogitFileModeToEntryMode(fmode))
	}
}
