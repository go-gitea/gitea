// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build ignore

package models

import (
	"xorm.io/builder"
)

func AccessibleRepoIDsQuery() *builder.Builder {
	return builder.Select("id").From("repository").Where(accessibleRepositoryCondition(user))
}
