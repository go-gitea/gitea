// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "strings"

// IsInSlice returns true if the target exists in the slice.
func IsInSlice[T comparable](target T, slice []T) bool {
	return IsInSliceFunc(func(t T) bool { return t == target }, slice)
}

// IsInSliceFunc returns true if any element in the slice satisfies the targetFunc.
func IsInSliceFunc[T comparable](targetFunc func(T) bool, slice []T) bool {
	for _, v := range slice {
		if targetFunc(v) {
			return true
		}
	}
	return false
}

// IsStringInSlice sequential searches if string exists in slice.
func IsStringInSlice(target string, slice []string, insensitive ...bool) bool {
	if len(insensitive) != 0 && insensitive[0] {
		target = strings.ToLower(target)
		return IsInSliceFunc(func(t string) bool { return strings.ToLower(t) == target }, slice)
	}

	return IsInSlice(target, slice)
}

// IsEqualSlice returns true if slices are equal.
// Be careful, two slice has the same elements but different order are considered equal here.
func IsEqualSlice[T comparable](target, source []T) bool {
	if len(target) != len(source) {
		return false
	}

	if (target == nil) != (source == nil) {
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
