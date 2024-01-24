// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path"
	"strings"
)

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ptree:     t,
			ID:        t.ID,
			name:      "",
			fullName:  "",
			entryMode: EntryModeTree,
		}, nil
	}

	// FIXME: This should probably use git cat-file --batch to be a bit more efficient
	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := t
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries()
			if err != nil {
				return nil, err
			}
			for _, v := range entries {
				if v.Name() == name {
					return v, nil
				}
			}
		} else {
			tree, err = tree.SubTree(name)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, ErrNotExist{"", relpath}
}

// GetTreeEntryByPathWithPathAsName returns the Same as GetTreeEntryByPath, but it Replaces the Full Name with the relpath
func (t *Tree) GetTreeEntryByPathWithPathAsName(relpath string) (*TreeEntry, error) {
	a, err := t.GetTreeEntryByPath(relpath)
	if err != nil {
		return nil, err
	}
	if a.fullName == "" {
		a.fullName = relpath
	}
	return a, err
}
