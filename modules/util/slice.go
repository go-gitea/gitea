// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

// IntersectString returns the intersection of the two string slices
func IntersectString(a, b []string) []string {
	var intersection []string
	for _, v := range a {
		if IsStringInSlice(v, b) && !IsStringInSlice(v, intersection) {
			intersection = append(intersection, v)
		}
	}
	return intersection
}

// DifferenceString returns all elements of slice a which are not present in slice b
func DifferenceString(a, b []string) []string {
	var difference []string
	for _, v := range a {
		if !IsStringInSlice(v, b) && !IsStringInSlice(v, difference) {
			difference = append(difference, v)
		}
	}
	return difference
}
