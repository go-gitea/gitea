// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

func keysInt64(m map[int64]struct{}) []int64 {
	var keys = make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func valuesRepository(m map[int64]*Repository) []*Repository {
	var values = make([]*Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func valuesUser(m map[int64]*User) []*User {
	var values = make([]*User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
