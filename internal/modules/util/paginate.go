// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "reflect"

// PaginateSlice cut a slice as per pagination options
// if page = 0 it do not paginate
func PaginateSlice(list any, page, pageSize int) any {
	if page <= 0 || pageSize <= 0 {
		return list
	}
	if reflect.TypeOf(list).Kind() != reflect.Slice {
		return list
	}

	listValue := reflect.ValueOf(list)

	page--

	if page*pageSize >= listValue.Len() {
		return listValue.Slice(listValue.Len(), listValue.Len()).Interface()
	}

	listValue = listValue.Slice(page*pageSize, listValue.Len())

	if listValue.Len() > pageSize {
		return listValue.Slice(0, pageSize).Interface()
	}

	return listValue.Interface()
}
