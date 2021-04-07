// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"time"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// changeUserTable looks through the database for issue_labels where the label no longer exists and deletes them.
func changeUserTable(x *xorm.Engine) (err error) {
	type User struct {
		ID          int64
		LoginType   int
		LoginSource int64
		LoginName   string
		Name        string
		Email       string
	}

	type ExternalLoginUser struct {
		ExternalID        string                 `xorm:"pk NOT NULL"`
		UserID            int64                  `xorm:"INDEX NOT NULL"`
		LoginSourceID     int64                  `xorm:"pk NOT NULL"`
		RawData           map[string]interface{} `xorm:"TEXT JSON"`
		Provider          string                 `xorm:"index VARCHAR(25)"`
		Email             string
		Name              string
		FirstName         string
		LastName          string
		NickName          string
		Description       string
		AvatarURL         string
		Location          string
		AccessToken       string `xorm:"TEXT"`
		AccessTokenSecret string `xorm:"TEXT"`
		RefreshToken      string `xorm:"TEXT"`
		ExpiresAt         time.Time
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100

	if err = sess.Begin(); err != nil {
		return
	}

	for start := 0; ; start += batchSize {
		users := make([]*User, 0, batchSize)
		if err = sess.Limit(batchSize, start).
			Where(builder.Neq{"login_type": 0}).
			Find(&users); err != nil {
			return
		}
		if len(users) == 0 {
			break
		}

		for _, user := range users {
			if _, err = sess.Insert(&ExternalLoginUser{
				ExternalID:    user.LoginName,
				UserID:        user.ID,
				LoginSourceID: user.LoginSource,
				Email:         user.Email,
				Name:          user.Name,
				NickName:      user.LoginName,
			}); err != nil {
				return
			}
		}
	}

	// drop the columns
	if err = dropTableColumns(sess, "user", "login_type", "login_source", "login_name"); err != nil {
		return
	}
	return sess.Commit()
}
