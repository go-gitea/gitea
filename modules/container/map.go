// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

// MapValues returns a slice of all values in a map.
// TODO: remove this after Golang std lib has similar function but not golang.org/x/exp
func MapValues[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
