// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"sort"
	"strings"
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

// FollowLink returns the entry pointed to by a symlink
func (te *TreeEntry) FollowLink() (*TreeEntry, error) {
	if !te.IsLink() {
		return nil, ErrBadLink{te.Name(), "not a symlink"}
	}

	// read the link
	r, err := te.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = r.Close()
		}
	}()
	buf := make([]byte, te.Size())
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	_ = r.Close()
	closed = true

	lnk := string(buf)
	t := te.ptree

	// traverse up directories
	for ; t != nil && strings.HasPrefix(lnk, "../"); lnk = lnk[3:] {
		t = t.ptree
	}

	if t == nil {
		return nil, ErrBadLink{te.Name(), "points outside of repo"}
	}

	target, err := t.GetTreeEntryByPath(lnk)
	if err != nil {
		if IsErrNotExist(err) {
			return nil, ErrBadLink{te.Name(), "broken link"}
		}
		return nil, err
	}
	return target, nil
}

// FollowLinks returns the entry ultimately pointed to by a symlink
func (te *TreeEntry) FollowLinks() (*TreeEntry, error) {
	if !te.IsLink() {
		return nil, ErrBadLink{te.Name(), "not a symlink"}
	}
	entry := te
	for i := 0; i < 999; i++ {
		if entry.IsLink() {
			next, err := entry.FollowLink()
			if err != nil {
				return nil, err
			}
			if next.ID == entry.ID {
				return nil, ErrBadLink{
					entry.Name(),
					"recursive link",
				}
			}
			entry = next
		} else {
			break
		}
	}
	if entry.IsLink() {
		return nil, ErrBadLink{
			te.Name(),
			"too many levels of symbolic links",
		}
	}
	return entry, nil
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
