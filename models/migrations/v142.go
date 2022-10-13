// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func setIsArchivedToFalse(x *xorm.Engine) error {
	type Repository struct {
		IsArchived bool `xorm:"INDEX"`
	}
	count, err := x.Where(builder.IsNull{"is_archived"}).Cols("is_archived").Update(&Repository{
		IsArchived: false,
	})
	if err == nil {
		log.Debug("Updated %d repositories with is_archived IS NULL", count)
	}
	return err
}
