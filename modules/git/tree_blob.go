// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path"
)

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID:   t.ID,
			Type: ObjectTree,
			mode: EntryModeTree,
		}, nil
	}

	relpath = path.Clean(relpath)
	entries, err := t.ListEntries(relpath)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.name == relpath {
			return v, nil
		}
	}

	return nil, ErrNotExist{"", relpath}
}

// GetBlobByPath get the blob object according the path
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
