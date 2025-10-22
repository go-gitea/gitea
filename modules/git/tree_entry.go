// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// Type returns the type of the entry (commit, tree, blob)
func (te *TreeEntry) Type() string {
	switch te.Mode() {
	case EntryModeCommit:
		return "commit"
	case EntryModeTree:
		return "tree"
	default:
		return "blob"
	}
}

type EntryFollowResult struct {
	SymlinkContent string
	TargetFullPath string
	TargetEntry    *TreeEntry
}

func EntryFollowLink(commit *Commit, fullPath string, te *TreeEntry) (*EntryFollowResult, error) {
	if !te.IsLink() {
		return nil, util.ErrorWrap(util.ErrUnprocessableContent, "%q is not a symlink", fullPath)
	}

	// git's filename max length is 4096, hopefully a link won't be longer than multiple of that
	const maxSymlinkSize = 20 * 4096
	if te.Blob().Size() > maxSymlinkSize {
		return nil, util.ErrorWrap(util.ErrUnprocessableContent, "%q content exceeds symlink limit", fullPath)
	}

	link, err := te.Blob().GetBlobContent(maxSymlinkSize)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(link, "/") {
		// It's said that absolute path will be stored as is in Git
		return &EntryFollowResult{SymlinkContent: link}, util.ErrorWrap(util.ErrUnprocessableContent, "%q is an absolute symlink", fullPath)
	}

	targetFullPath := path.Join(path.Dir(fullPath), link)
	targetEntry, err := commit.GetTreeEntryByPath(targetFullPath)
	if err != nil {
		return &EntryFollowResult{SymlinkContent: link}, err
	}
	return &EntryFollowResult{SymlinkContent: link, TargetFullPath: targetFullPath, TargetEntry: targetEntry}, nil
}

func EntryFollowLinks(commit *Commit, firstFullPath string, firstTreeEntry *TreeEntry, optLimit ...int) (res *EntryFollowResult, err error) {
	limit := util.OptionalArg(optLimit, 10)
	treeEntry, fullPath := firstTreeEntry, firstFullPath
	for range limit {
		res, err = EntryFollowLink(commit, fullPath, treeEntry)
		if err != nil {
			return res, err
		}
		treeEntry, fullPath = res.TargetEntry, res.TargetFullPath
		if !treeEntry.IsLink() {
			break
		}
	}
	if treeEntry.IsLink() {
		return res, util.ErrorWrap(util.ErrUnprocessableContent, "%q has too many links", firstFullPath)
	}
	return res, nil
}

// returns the Tree pointed to by this TreeEntry, or nil if this is not a tree
func (te *TreeEntry) Tree() *Tree {
	t, err := te.ptree.repo.getTree(te.ID)
	if err != nil {
		return nil
	}
	t.ptree = te.ptree
	return t
}

// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func (te *TreeEntry) GetSubJumpablePathName() string {
	if te.IsSubModule() || !te.IsDir() {
		return ""
	}
	tree, err := te.ptree.SubTree(te.Name())
	if err != nil {
		return te.Name()
	}
	entries, _ := tree.ListEntries()
	if len(entries) == 1 && entries[0].IsDir() {
		name := entries[0].GetSubJumpablePathName()
		if name != "" {
			return te.Name() + "/" + name
		}
	}
	return te.Name()
}

// Entries a list of entry
type Entries []*TreeEntry

type customSortableEntries struct {
	Comparer func(s1, s2 string) bool
	Entries
}

var sorter = []func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool{
	func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool {
		return (t1.IsDir() || t1.IsSubModule()) && !t2.IsDir() && !t2.IsSubModule()
	},
	func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool {
		return cmp(t1.Name(), t2.Name())
	},
}

func (ctes customSortableEntries) Len() int { return len(ctes.Entries) }

func (ctes customSortableEntries) Swap(i, j int) {
	ctes.Entries[i], ctes.Entries[j] = ctes.Entries[j], ctes.Entries[i]
}

func (ctes customSortableEntries) Less(i, j int) bool {
	t1, t2 := ctes.Entries[i], ctes.Entries[j]
	var k int
	for k = 0; k < len(sorter)-1; k++ {
		s := sorter[k]
		switch {
		case s(t1, t2, ctes.Comparer):
			return true
		case s(t2, t1, ctes.Comparer):
			return false
		}
	}
	return sorter[k](t1, t2, ctes.Comparer)
}

// Sort sort the list of entry
func (tes Entries) Sort() {
	sort.Sort(customSortableEntries{func(s1, s2 string) bool {
		return s1 < s2
	}, tes})
}

// CustomSort customizable string comparing sort entry list
func (tes Entries) CustomSort(cmp func(s1, s2 string) bool) {
	sort.Sort(customSortableEntries{cmp, tes})
}
