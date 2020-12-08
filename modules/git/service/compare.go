// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "container/list"

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	MergeBase string
	Commits   *list.List
	NumFiles  int
}
