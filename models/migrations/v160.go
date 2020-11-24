// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func updateNumPublicRepos(x *xorm.Engine) error {
	// User represents a user
	type User struct {
		ID             int64
		NumPublicRepos int `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	// Repository represents a Repo
	type Repository struct {
		ID        int64
		OwnerID   int64
		IsPrivate bool
	}

	if err := x.Sync2(&User{}, &Repository{}); err != nil {
		return err
	}
	var batchSize = 100

	users := make([]*User, 0, batchSize)

	sess := x.NewSession()
	defer sess.Close()

	sess.SetExpr("num_public_repos", 0).Update(&User{})

	for start := 0; ; start += batchSize {
		users = users[:0]

		if err := sess.Begin(); err != nil {
			return err
		}

		if err := sess.Select("owner_id AS id, count(id) AS num_public_repos").Table("repository").Where("repository.is_private", false).GroupBy("owner_id").Limit(batchSize, start).Asc("id").Find(&users); err != nil {
			return err
		}

		if len(users) == 0 {
			break
		}

		for _, user := range users {
			if _, err := sess.ID(user.ID).Cols("num_public_repos").Update(user); err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}
	return nil
}
