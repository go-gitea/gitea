// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
)

// IsInSlice returns true if the target exists in the slice.
func IsInSlice[T comparable](target T, slice []T) bool {
	return IsInSliceFunc(func(t T) bool { return t == target }, slice)
}

// IsInSliceFunc returns true if any element in the slice satisfies the targetF.
func IsInSliceFunc[T comparable](targetF func(T) bool, slice []T) bool {
	for _, v := range slice {
		if targetF(v) {
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
