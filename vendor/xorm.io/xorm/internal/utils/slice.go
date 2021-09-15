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
	sort.Strings(left)
	sort.Strings(right)
	for i := 0; i < len(left); i++ {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func IndexSlice(s []string, c string) int {
	for i, ss := range s {
		if c == ss {
			return i
		}
	}
	return -1
}
