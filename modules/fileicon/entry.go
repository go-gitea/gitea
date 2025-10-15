// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

type EntryInfo struct {
	BaseName      string
	EntryMode     git.EntryMode
	SymlinkToMode git.EntryMode
	IsOpen        bool
}

func EntryInfoFromGitTreeEntry(ctx context.Context, commit *git.Commit, fullPath string, gitEntry *git.TreeEntry) *EntryInfo {
	ret := &EntryInfo{BaseName: gitEntry.Name(), EntryMode: gitEntry.Mode()}
	if gitEntry.IsLink() {
		if res, err := git.EntryFollowLink(ctx, commit, fullPath, gitEntry); err == nil && res.TargetEntry.IsDir() {
			ret.SymlinkToMode = res.TargetEntry.Mode()
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
