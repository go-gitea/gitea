// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"xorm.io/xorm"
)

func AddTemplateToRepo(x *xorm.Engine) error {
	type Repository struct {
		IsTemplate bool  `xorm:"INDEX NOT NULL DEFAULT false"`
		TemplateID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Repository))
}
