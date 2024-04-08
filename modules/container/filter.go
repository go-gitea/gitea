// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import "slices"

// FilterMapUnique ranges over the slice and calls include() for each element.
// If the second returned value is true, the first returned value will be included in the resulting
// slice (after deduplication).
func FilterMapUnique[T, V comparable](elements []T, include func(T) (V, bool)) []V {
	filtered := make([]V, 0, len(elements)) // slice will be clipped before returning
	seen := make(map[V]bool, len(elements))
	for _, e := range elements {
		if v, ok := include(e); ok && !seen[v] {
			filtered = append(filtered, v)
			seen[v] = true
		}
	}
	return slices.Clip(filtered)
}
