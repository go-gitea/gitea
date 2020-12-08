// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "sort"

// Entries a list of entry
type Entries []TreeEntry

type customSortableEntries struct {
	Comparer func(s1, s2 string) bool
	Entries
}

var sorter = []func(t1, t2 TreeEntry, cmp func(s1, s2 string) bool) bool{
	func(t1, t2 TreeEntry, cmp func(s1, s2 string) bool) bool {
		return (t1.Mode().IsDir() || t1.Mode().IsSubModule()) && !t2.Mode().IsDir() && !t2.Mode().IsSubModule()
	},
	func(t1, t2 TreeEntry, cmp func(s1, s2 string) bool) bool {
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
