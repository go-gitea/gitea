// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"sort"

	"code.gitea.io/gitea/modules/git/service"
)

type tagSorter []service.Tag

func (ts tagSorter) Len() int {
	return len([]service.Tag(ts))
}

func (ts tagSorter) Less(i, j int) bool {
	return []service.Tag(ts)[i].Tagger().When.After([]service.Tag(ts)[j].Tagger().When)
}

func (ts tagSorter) Swap(i, j int) {
	[]service.Tag(ts)[i], []service.Tag(ts)[j] = []service.Tag(ts)[j], []service.Tag(ts)[i]
}

// SortTagsByTime sorts an array of tags
func SortTagsByTime(tags []service.Tag) {
	sorter := tagSorter(tags)
	sort.Sort(sorter)
}
