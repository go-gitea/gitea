// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

// RemoveIDFromList removes the given ID from the slice, if found.
// It does not preserve order, and assumes the ID is unique.
func RemoveIDFromList(list []int64, id int64) ([]int64, bool) {
	n := len(list) - 1
	for i, item := range list {
		if item == id {
			list[i] = list[n]
			return list[:n], true
		}
	}
	return list, false
}
