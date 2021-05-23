// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addPrimaryEmail2EmailAddress(x *xorm.Engine) (err error) {
	type User struct {
		ID       int64  `xorm:"pk autoincr"`
		Email    string `xorm:"NOT NULL"`
		IsActive bool   `xorm:"INDEX"` // Activate primary email
	}

	type EmailAddress struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		IsActivated bool
		IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
	}

	// Add is_primary column
	if err = x.Sync2(new(EmailAddress)); err != nil {
		return
	}

	if _, err = x.Exec("UPDATE email_address SET is_primary=? WHERE is_primary IS NULL", false); err != nil {
		return
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100

	for start := 0; ; start += batchSize {
		users := make([]*User, 0, batchSize)
		if err = sess.Limit(batchSize, start).Find(&users); err != nil {
			return
		}
		if len(users) == 0 {
			break
		}

		for _, user := range users {
			var exist bool
			exist, err = sess.Where("email=?", user.Email).Table("email_address").Exist()
			if err != nil {
				return
			}
			if !exist {
				if _, err = sess.Insert(&EmailAddress{
					UID:         user.ID,
					Email:       user.Email,
					IsActivated: user.IsActive,
					IsPrimary:   true,
				}); err != nil {
					return
				}
			} else {
				if _, err = sess.Where("email=?", user.Email).Cols("is_primary").Update(&EmailAddress{
					IsPrimary: true,
				}); err != nil {
					return
				}
			}
		}
	}
	return nil
}
