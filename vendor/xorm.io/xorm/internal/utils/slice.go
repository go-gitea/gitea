// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import "sort"

// SliceEq return true if two slice have the same elements even if different sort.
func SliceEq(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	sort.Sort(sort.StringSlice(left))
	sort.Sort(sort.StringSlice(right))
	for i := 0; i < len(left); i++ {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
