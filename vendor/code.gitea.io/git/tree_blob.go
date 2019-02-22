// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path"
	"strings"
)

// GetTreeEntryByPath get the tree entries accroding the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string, cache LsTreeCache) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID:   t.ID,
			Type: ObjectTree,
			mode: EntryModeTree,
		}, nil
	}

	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := t
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries(cache)
			if err != nil {
				return nil, err
			}
			for _, v := range entries {
				if v.name == name {
					return v, nil
				}
			}
		} else {
			tree, err = tree.SubTree(name, cache)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, ErrNotExist{"", relpath}
}

// GetBlobByPath get the blob object accroding the path
func (t *Tree) GetBlobByPath(relpath string) (*Blob, error) {
	entry, err := t.GetTreeEntryByPath(relpath, nil)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir() {
		return entry.Blob(), nil
	}

	return nil, ErrNotExist{"", relpath}
}
