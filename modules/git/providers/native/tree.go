// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"path"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Tree) = &Tree{}

// Tree represents a git tree
type Tree struct {
	Object

	parent     service.Tree
	resolvedID service.Hash

	entriesParsed bool
	entries       service.Entries

	entriesRecursiveParsed bool
	entriesRecursive       service.Entries
}

//  _
// |_ ._  _|_ ._ o  _   _
// |_ | |  |_ |  | (/_ _>
//

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (service.TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			Object:    t.Object,
			ptree:     t,
			name:      "",
			fullName:  "",
			entryMode: service.EntryModeTree,
		}, nil
	}

	// FIXME: This should probably use git cat-file --batch to be a bit more efficient
	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := service.Tree(t)
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
	return nil, git.ErrNotExist{
		ID:      "",
		RelPath: relpath,
	}
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (service.Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}

	stdout, err := git.NewCommand("ls-tree", t.ID().String()).RunInDirBytes(t.Repository().Path())
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") || strings.Contains(err.Error(), "fatal: not a tree object") {
			return nil, git.ErrNotExist{
				ID: t.ID().String(),
			}
		}
		return nil, err
	}

	t.entries, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesParsed = true
	}

	return t.entries, err
}

// ListEntriesRecursive returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursive() (service.Entries, error) {
	if t.entriesRecursiveParsed {
		return t.entriesRecursive, nil
	}
	stdout, err := git.NewCommand("ls-tree", "-t", "-r", t.ID().String()).RunInDirBytes(t.Repository().Path())
	if err != nil {
		return nil, err
	}

	t.entriesRecursive, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesRecursiveParsed = true
	}

	return t.entriesRecursive, err
}

// SubTree get a sub tree by the sub dir path
func (t *Tree) SubTree(rpath string) (service.Tree, error) {
	if len(rpath) == 0 {
		return t, nil
	}

	paths := strings.Split(rpath, "/")
	var (
		err error
		g   = t
		p   = t
		te  service.TreeEntry
	)
	for _, name := range paths {
		te, err = p.GetTreeEntryByPath(name)
		if err != nil {
			return nil, err
		}

		g := &Tree{
			Object: Object{
				hash: te.ID(),
				repo: t.Repository(),
			},
			parent: p,
		}

		p = g
	}
	return g, nil
}

// Parent gets the parent tree of this tree (if available)
func (t *Tree) Parent() (service.Tree, error) {
	return t.parent, nil
}

//  _
// |_) |  _  |_
// |_) | (_) |_)
//

// GetBlobByPath get the blob object according the path
func (t *Tree) GetBlobByPath(relpath string) (service.Blob, error) {
	return common.TreeGetBlobByPath(t, relpath)
}

//
// |_|  _.  _ |_
// | | (_| _> | |
//

// ResolvedID returns the hash that was resolved to find this tree
func (t *Tree) ResolvedID() service.Hash {
	return t.resolvedID
}
