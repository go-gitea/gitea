// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

// PaginateSlice cut a slice as per pagination options
// if page = 0 it do not paginate
func PaginateSlice[S ~[]E, E any](list S, page, pageSize int) S {
	if page <= 0 || pageSize <= 0 {
		return list
	}

	page--

	if page*pageSize >= len(list) {
		return list[len(list):]
	}

	list = list[page*pageSize:]

	if len(list) > pageSize {
		return list[:pageSize]
	}

	return list
}
