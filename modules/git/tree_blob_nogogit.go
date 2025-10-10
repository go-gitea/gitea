// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path"
	"strings"
)

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(ctx context.Context, relpath string) (_ *TreeEntry, err error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ptree:     t,
			ID:        t.ID,
			name:      "",
			entryMode: EntryModeTree,
		}, nil
	}

	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")

	tree := t
	for _, name := range parts[:len(parts)-1] {
		tree, err = tree.SubTree(ctx, name)
		if err != nil {
			return nil, err
		}
	}

	name := parts[len(parts)-1]
	entries, err := tree.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.Name() == name {
			return v, nil
		}
	}
	return nil, ErrNotExist{"", relpath}
}
