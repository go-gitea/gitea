// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (_ *TreeEntry, err error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID:        t.ID,
			name:      "",
			EntryMode: EntryModeTree,
		}, nil
	}

	output, _, err := gitcmd.NewCommand("ls-tree", "-l").
		AddDynamicArguments(t.ID.String()).
		WithDir(t.repo.Path).
		AddDashesAndList(relpath).RunStdBytes(t.repo.Ctx)
	if err != nil {
		return nil, err
	}
	if string(output) == "" {
		return nil, ErrNotExist{"", relpath}
	}

	return parseTreeEntry(output)
}
