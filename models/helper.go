// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	repo_model "code.gitea.io/gitea/models/repo"
)

func valuesRepository(m map[int64]*repo_model.Repository) []*repo_model.Repository {
	values := make([]*repo_model.Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
