// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15 //nolint

import (
	"strings"

	"xorm.io/xorm"
)

func AddPrimaryEmail2EmailAddress(x *xorm.Engine) (err error) {
	type User struct {
		ID       int64  `xorm:"pk autoincr"`
		Email    string `xorm:"NOT NULL"`
		IsActive bool   `xorm:"INDEX"` // Activate primary email
	}

	type EmailAddress1 struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		LowerEmail  string
		IsActivated bool
		IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
	}

	// Add lower_email and is_primary columns
	if err = x.Table("email_address").Sync2(new(EmailAddress1)); err != nil {
		return
	}

	if _, err = x.Exec("UPDATE email_address SET lower_email=LOWER(email), is_primary=?", false); err != nil {
		return
	}

	type EmailAddress struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		LowerEmail  string `xorm:"UNIQUE NOT NULL"`
		IsActivated bool
		IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
	}

	// change lower_email as unique
	if err = x.Sync2(new(EmailAddress)); err != nil {
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
					LowerEmail:  strings.ToLower(user.Email),
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
