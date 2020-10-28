// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func addUserPrimaryEmailToUserMails(x *xorm.Engine) error {
	type User struct {
		ID       int64  `xorm:"pk autoincr"`
		Email    string `xorm:"NOT NULL"`
		IsActive bool   `xorm:"INDEX"`
	}
	type EmailAddress struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		IsActivated bool
	}

	if err := x.Sync2(new(User), new(EmailAddress)); err != nil {
		return err
	}

	updateUsers := func(users []*User) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		for _, user := range users {
			email := new(EmailAddress)
			if isExist, err := sess.Where("email=?", user.Email).Get(email); err != nil {
				return err
			} else if isExist {
				if email.UID != user.ID {
					log.Warn("Email (%s) from user with ID %d is taken by user with ID %d", email.Email, user.ID, email.UID)
					log.Warn("Skipping to avoid conflicts, should be manually fixed")
				}
				continue
			}
			email = &EmailAddress{
				Email:       user.Email,
				UID:         user.ID,
				IsActivated: user.IsActive,
			}
			if _, err := sess.Insert(email); err != nil {
				return err
			}
		}

		return sess.Commit()
	}

	var start = 0
	var batchSize = 100
	for {
		var users = make([]*User, 0, batchSize)
		if err := x.Limit(batchSize, start).Find(&users); err != nil {
			return err
		}

		if err := updateUsers(users); err != nil {
			return err
		}

		start += len(users)

		if len(users) < batchSize {
			break
		}
	}

	return nil
}
