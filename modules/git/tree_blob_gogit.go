// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"path"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID: t.ID,
			// Type: ObjectTree,
			ptree: t,
			gogitTreeEntry: &object.TreeEntry{
				Name: "",
				Mode: filemode.Dir,
				Hash: plumbing.Hash(t.ID.RawValue()),
			},
		}, nil
	}

	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := t
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries()
			if err != nil {
				if err == plumbing.ErrObjectNotFound {
					return nil, ErrNotExist{
						RelPath: relpath,
					}
				}
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
				if err == plumbing.ErrObjectNotFound {
					return nil, ErrNotExist{
						RelPath: relpath,
					}
				}
				return nil, err
			}
		}
	}
	return nil, ErrNotExist{"", relpath}
}
