// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"io"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var _ (service.Tree) = &Tree{}

// Tree represents a git tree
type Tree struct {
	Object

	gogitTree *object.Tree

	// parent tree
	ptree      service.Tree
	resolvedID service.Hash
}

func (t *Tree) loadTreeObject() error {
	gogitRepo, err := GetGoGitRepo(t.repo)
	if err != nil {
		return err
	}
	gogitTree, err := gogitRepo.TreeObject(ToPlumbingHash(t.hash))
	if err != nil {
		return err
	}

	t.gogitTree = gogitTree
	return nil
}

//  _
// |_ ._  _|_ ._ o  _   _
// |_ | |  |_ |  | (/_ _>
//

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (service.TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			Object:   t.Object,
			ptree:    t,
			fullName: "",
		}, nil
	}

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
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	entries := make([]service.TreeEntry, len(t.gogitTree.Entries))
	for i, entry := range t.gogitTree.Entries {
		entries[i] = &TreeEntry{
			Object: Object{
				hash: fromPlumbingHash(entry.Hash),
				repo: t.repo,
			},
			gogitTreeEntry: &t.gogitTree.Entries[i],
			ptree:          t,
		}
	}

	return entries, nil
}

// ListEntriesRecursive returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursive() (service.Entries, error) {
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	var entries []service.TreeEntry
	seen := map[plumbing.Hash]bool{}
	walker := object.NewTreeWalker(t.gogitTree, true, seen)
	for {
		fullName, entry, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if seen[entry.Hash] {
			continue
		}

		convertedEntry := &TreeEntry{
			Object: Object{
				hash: fromPlumbingHash(entry.Hash),
				repo: t.repo,
			},
			gogitTreeEntry: &entry,
			ptree:          t,
			fullName:       fullName,
		}
		entries = append(entries, convertedEntry)
	}

	return entries, nil
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
			ptree: p,
		}

		err = g.loadTreeObject()
		if err != nil {
			return nil, err
		}
		p = g
	}
	return g, nil
}

// Parent gets the parent tree of this tree (if available)
func (t *Tree) Parent() (service.Tree, error) {
	return t.ptree, nil
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
