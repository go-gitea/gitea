// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addTemplateToRepo(x *xorm.Engine) error {

	type Repository struct {
		IsTemplate bool  `xorm:"INDEX NOT NULL DEFAULT false"`
		TemplateID int64 `xorm:"INDEX"`
	}

	return x.Sync2(new(Repository))
}
