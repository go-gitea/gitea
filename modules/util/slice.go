// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"slices"
	"strings"
)

// SliceContainsString sequential searches if string exists in slice.
func SliceContainsString(slice []string, target string, insensitive ...bool) bool {
	if len(insensitive) != 0 && insensitive[0] {
		target = strings.ToLower(target)
		return slices.ContainsFunc(slice, func(t string) bool { return strings.ToLower(t) == target })
	}

	return slices.Contains(slice, target)
}

// SliceSortedEqual returns true if the two slices will be equal when they get sorted.
// It doesn't require that the slices have been sorted, and it doesn't sort them either.
func SliceSortedEqual[T comparable](s1, s2 []T) bool {
	if len(s1) != len(s2) {
		return false
	}

	counts := make(map[T]int, len(s1))
	for _, v := range s1 {
		counts[v]++
	}
	for _, v := range s2 {
		counts[v]--
	}

	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

// SliceRemoveAll removes all the target elements from the slice.
func SliceRemoveAll[T comparable](slice []T, target T) []T {
	return slices.DeleteFunc(slice, func(t T) bool { return t == target })
}
