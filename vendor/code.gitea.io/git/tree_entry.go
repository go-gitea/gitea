// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io"
	"sort"
	"strconv"
	"strings"
)

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeBlob
	EntryModeBlob EntryMode = 0x0100644
	// EntryModeExec
	EntryModeExec EntryMode = 0x0100755
	// EntryModeSymlink
	EntryModeSymlink EntryMode = 0x0120000
	// EntryModeCommit
	EntryModeCommit EntryMode = 0x0160000
	// EntryModeTree
	EntryModeTree EntryMode = 0x0040000
)

// TreeEntry the leaf in the git tree
type TreeEntry struct {
	ID   SHA1
	Type ObjectType

	mode EntryMode
	name string

	ptree *Tree

	commited bool

	size  int64
	sized bool
}

// Name returns the name of the entry
func (te *TreeEntry) Name() string {
	return te.name
}

// Mode returns the mode of the entry
func (te *TreeEntry) Mode() EntryMode {
	return te.mode
}

// Size returns the size of the entry
func (te *TreeEntry) Size() int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	stdout, err := NewCommand("cat-file", "-s", te.ID.String()).RunInDir(te.ptree.repo.Path)
	if err != nil {
		return 0
	}

	te.sized = true
	te.size, _ = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
	return te.size
}

// IsSubModule if the entry is a sub module
func (te *TreeEntry) IsSubModule() bool {
	return te.mode == EntryModeCommit
}

// IsDir if the entry is a sub dir
func (te *TreeEntry) IsDir() bool {
	return te.mode == EntryModeTree
}

// IsLink if the entry is a symlink
func (te *TreeEntry) IsLink() bool {
	return te.mode == EntryModeSymlink
}

// Blob retrun the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		repo:      te.ptree.repo,
		TreeEntry: te,
	}
}

// FollowLink returns the entry pointed to by a symlink
func (te *TreeEntry) FollowLink() (*TreeEntry, error) {
	if !te.IsLink() {
		return nil, ErrBadLink{te.Name(), "not a symlink"}
	}

	// read the link
	r, err := te.Blob().Data()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, te.Size())
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

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

// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func (te *TreeEntry) GetSubJumpablePathName() string {
	if te.IsSubModule() || !te.IsDir() {
		return ""
	}
	tree, err := te.ptree.SubTree(te.name)
	if err != nil {
		return te.name
	}
	entries, _ := tree.ListEntries()
	if len(entries) == 1 && entries[0].IsDir() {
		name := entries[0].GetSubJumpablePathName()
		if name != "" {
			return te.name + "/" + name
		}
	}
	return te.name
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
		return cmp(t1.name, t2.name)
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
