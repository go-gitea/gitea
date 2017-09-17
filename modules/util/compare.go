// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "sort"

// Int64Slice attaches the methods of Interface to []int64, sorting in increasing order.
type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// IsSliceInt64Eq returns if the two slice has the same elements but different sequences.
func IsSliceInt64Eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Sort(Int64Slice(a))
	sort.Sort(Int64Slice(b))
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
