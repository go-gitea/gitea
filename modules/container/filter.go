// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import "slices"

// FilterSlice ranges over the slice and calls include() for each element.
// If the second returned value is true, the first returned value will be included in the resulting
// slice (after deduplication).
func FilterSlice[E any, T comparable](s []E, include func(E) (T, bool)) []T {
	filtered := make([]T, 0, len(s)) // slice will be clipped before returning
	seen := make(map[T]bool, len(s))
	for i := range s {
		if v, ok := include(s[i]); ok && !seen[v] {
			filtered = append(filtered, v)
			seen[v] = true
		}
	}
	return slices.Clip(filtered)
}
