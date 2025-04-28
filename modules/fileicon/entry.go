// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import "code.gitea.io/gitea/modules/git"

type EntryInfo struct {
	FullName      string
	EntryMode     git.EntryMode
	SymlinkToMode git.EntryMode
	IsOpen        bool
}

func EntryInfoFromGitTreeEntry(gitEntry *git.TreeEntry) *EntryInfo {
	ret := &EntryInfo{FullName: gitEntry.Name(), EntryMode: gitEntry.Mode()}
	if gitEntry.IsLink() {
		if te, err := gitEntry.FollowLink(); err == nil && te.IsDir() {
			ret.SymlinkToMode = te.Mode()
		}
	}
	return ret
}

func EntryInfoFolder() *EntryInfo {
	return &EntryInfo{EntryMode: git.EntryModeTree}
}

func EntryInfoFolderOpen() *EntryInfo {
	return &EntryInfo{EntryMode: git.EntryModeTree, IsOpen: true}
}
