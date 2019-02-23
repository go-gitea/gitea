// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path"
	"strings"
)

// GetTreeEntryByPath get the tree entries accroding the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
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
	log("relpath: %s", relpath)
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries()
			if err != nil {
				return nil, err
			}
			log("NAME: %s", name)
			for _, v := range entries {
				log("v.NAME: %s", v.name)
				if v.name == name {
					log("FOUND %s = %s",v.name, name)
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

// GetBlobByPath get the blob object accroding the path
func (t *Tree) GetBlobByPath(relpath string) (*Blob, error) {
	entry, err := t.GetTreeEntryByPath(relpath)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir() {
		return entry.Blob(), nil
	}

	return nil, ErrNotExist{"", relpath}
}
