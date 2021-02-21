// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func removeUserHashAlgo(x *xorm.Engine) (err error) {
	// Make sure the columns exist before dropping them
	type User struct {
		passwdHashAlgo string
	}
	if err := x.Sync2(new(User)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "user", "passwdHashAlgo"); err != nil {
		return err
	}
	return sess.Commit()
}
func updateUserPasswords(x *xorm.Engine) (err error) {
	const (
		algoBcrypt = "bcrypt"
		algoScrypt = "scrypt"
		algoArgon2 = "argon2"
		algoPbkdf2 = "pbkdf2"
	)

	type User struct {
		ID             int64  `xorm:"pk autoincr"`
		Passwd         string `xorm:"NOT NULL"`
		PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 500

	for start := 0; ; start += batchSize {
		users := make([]*User, 0, batchSize)
		if err = sess.Limit(batchSize, start).Where(builder.And(builder.Neq{"passwd": ""}, builder.Neq{"passwd_hash_algo": ""}), 0).Find(&users); err != nil {
			return
		}
		if len(users) == 0 {
			break
		}

		if err = sess.Begin(); err != nil {
			return
		}

		for _, user := range users {
			switch user.PasswdHashAlgo {
			case algoBcrypt:
				user.Passwd = "$bcrypt$" + user.Passwd
			case algoScrypt:
				user.Passwd = "$scrypt$65536$16$2$50" + user.Passwd
			case algoArgon2:
				user.Passwd = "$argon2$2$65536$8$50$" + user.Passwd
			case algoPbkdf2:
				fallthrough
			default:
				user.Passwd = "$pbkdf2$10000$50$" + user.Passwd
			}
			if _, err = sess.ID(user.ID).Cols("passwd").Update(user); err != nil {
				return err
			}
		}

		if err = sess.Commit(); err != nil {
			return
		}
	}

	return sess.Commit()
}
