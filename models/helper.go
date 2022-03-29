// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

func keysInt64(m map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func valuesRepository(m map[int64]*repo_model.Repository) []*repo_model.Repository {
	values := make([]*repo_model.Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func valuesUser(m map[int64]*user_model.User) []*user_model.User {
	values := make([]*user_model.User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
