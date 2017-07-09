// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"html"

	"code.gitea.io/gitea/models"

	"github.com/go-xorm/xorm"
)

func unescapeUserFullNames(x *xorm.Engine) (err error) {
	const batchSize = 100
	for start := 0; ; start += batchSize {
		users := make([]*models.User, 0, batchSize)
		if err := x.Limit(start, batchSize).Find(users); err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}
		for _, user := range users {
			user.FullName = html.UnescapeString(user.FullName)
			if _, err := x.Cols("full_name").Update(user); err != nil {
				return err
			}
		}
	}
}
