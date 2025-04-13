// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"cmp"
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

// Sorted returns the sorted slice
// Note: The parameter is sorted inline.
func Sorted[S ~[]E, E cmp.Ordered](values S) S {
	slices.Sort(values)
	return values
}

// TODO: Replace with "maps.Values" once available, current it only in golang.org/x/exp/maps but not in standard library
func ValuesOfMap[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// TODO: Replace with "maps.Keys" once available, current it only in golang.org/x/exp/maps but not in standard library
func KeysOfMap[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func SliceNilAsEmpty[T any](a []T) []T {
	if a == nil {
		return []T{}
	}
	return a
}
