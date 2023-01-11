// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Most of the functions in this file can have better implementations with "golang.org/x/exp/slices".
// However, "golang.org/x/exp" is experimental and unreliable, we shouldn't use it.
// So lets waiting for the "slices" has be promoted to the main repository one day.

package util

import "strings"

// SliceContains returns true if the target exists in the slice.
func SliceContains[T comparable](target T, slice []T) bool {
	return SliceContainsFunc(func(t T) bool { return t == target }, slice)
}

// SliceContainsFunc returns true if any element in the slice satisfies the targetFunc.
func SliceContainsFunc[T any](targetFunc func(T) bool, slice []T) bool {
	for _, v := range slice {
		if targetFunc(v) {
			return true
		}
	}
	return false
}

// SliceContainsString sequential searches if string exists in slice.
func SliceContainsString(target string, slice []string, insensitive ...bool) bool {
	if len(insensitive) != 0 && insensitive[0] {
		target = strings.ToLower(target)
		return SliceContainsFunc(func(t string) bool { return strings.ToLower(t) == target }, slice)
	}

	return SliceContains(target, slice)
}

// IsEqualSlice returns true if slices are equal.
// Be careful, two slice which have the same elements but different order are considered equal here.
func IsEqualSlice[T comparable](target, source []T) bool {
	if len(target) != len(source) {
		return false
	}

	counts := make(map[T]int, len(target))
	for _, v := range target {
		counts[v]++
	}
	for _, v := range source {
		counts[v]--
	}

	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

// RemoveFromSlice removes all the target elements from the slice.
func RemoveFromSlice[T comparable](target T, slice []T) []T {
	return RemoveFromSliceFunc(func(t T) bool { return t == target }, slice)
}

// RemoveFromSliceFunc removes all elements which satisfy the targetFunc from the slice.
func RemoveFromSliceFunc[T comparable](targetFunc func(T) bool, slice []T) []T {
	idx := 0
	for _, v := range slice {
		if targetFunc(v) {
			continue
		}
		slice[idx] = v
		idx++
	}
	return slice[:idx]
}
