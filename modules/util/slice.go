// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Most of the functions in this file can have better implementations with "golang.org/x/exp/slices".
// However, "golang.org/x/exp" is experimental and unreliable, we shouldn't use it.
// So lets waiting for the "slices" has be promoted to the main repository one day.

package util

import "strings"

// SliceContains returns true if the target exists in the slice.
func SliceContains[T comparable](slice []T, target T) bool {
	return SliceContainsFunc(slice, func(t T) bool { return t == target })
}

// SliceContainsFunc returns true if any element in the slice satisfies the targetFunc.
func SliceContainsFunc[T any](slice []T, targetFunc func(T) bool) bool {
	for _, v := range slice {
		if targetFunc(v) {
			return true
		}
	}
	return false
}

// SliceContainsString sequential searches if string exists in slice.
func SliceContainsString(slice []string, target string, insensitive ...bool) bool {
	if len(insensitive) != 0 && insensitive[0] {
		target = strings.ToLower(target)
		return SliceContainsFunc(slice, func(t string) bool { return strings.ToLower(t) == target })
	}

	return SliceContains(slice, target)
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

// SliceEqual returns true if the two slices are equal.
func SliceEqual[T comparable](s1, s2 []T) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i, v := range s1 {
		if s2[i] != v {
			return false
		}
	}
	return true
}

// SliceRemoveAll removes all the target elements from the slice.
func SliceRemoveAll[T comparable](slice []T, target T) []T {
	return SliceRemoveAllFunc(slice, func(t T) bool { return t == target })
}

// SliceRemoveAllFunc removes all elements which satisfy the targetFunc from the slice.
func SliceRemoveAllFunc[T comparable](slice []T, targetFunc func(T) bool) []T {
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

func SliceDifference[T comparable](first, second []T) []T {
	var difference []T

	for _, item := range first {
		if !SliceContains(second, item) {
			difference = append(difference, item)
		}
	}

	return difference
}
