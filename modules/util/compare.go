// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"sort"
	"strings"
)

// Int64Slice attaches the methods of Interface to []int64, sorting in increasing order.
type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// IsSliceInt64Eq returns if the two slice has the same elements but different sequences.
func IsSliceInt64Eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Sort(Int64Slice(a))
	sort.Sort(Int64Slice(b))
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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

// IsInt64InSlice sequential searches if int64 exists in slice.
func IsInt64InSlice(target int64, slice []int64) bool {
	for i := 0; i < len(slice); i++ {
		if slice[i] == target {
			return true
		}
	}
	return false
}

// IsEqualSlice returns true if slices are equal.
func IsEqualSlice(target, source []string) bool {
	if len(target) != len(source) {
		return false
	}

	if (target == nil) != (source == nil) {
		return false
	}

	sort.Strings(target)
	sort.Strings(source)

	for i, v := range target {
		if v != source[i] {
			return false
		}
	}

	return true
}
