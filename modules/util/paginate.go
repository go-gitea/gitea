// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "reflect"

// PaginateSlice cut a slice as per pagination options
// if page = 0 it do not paginate
func PaginateSlice(list interface{}, page, pageSize int) interface{} {
	if page <= 0 || pageSize <= 0 {
		return list
	}
	if reflect.TypeOf(list).Kind() != reflect.Slice {
		return list
	}

	listValue := reflect.ValueOf(list)

	first := (page - 1) * pageSize
	length := listValue.Len()

	if first >= length {
		return listValue.Slice(length, length).Interface()
	}

	if first+pageSize >= length {
		return listValue.Slice(first, length).Interface()
	}

	return listValue.Slice(first, first+pageSize).Interface()
}
