// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint

import (
	"encoding/hex"

	"github.com/minio/sha256-simd"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func RecalculateUserEmptyPWD(x *xorm.Engine) (err error) {
	const (
		algoBcrypt = "bcrypt"
		algoScrypt = "scrypt"
		algoArgon2 = "argon2"
		algoPbkdf2 = "pbkdf2"
	)

	type User struct {
		ID                 int64  `xorm:"pk autoincr"`
		Passwd             string `xorm:"NOT NULL"`
		PasswdHashAlgo     string `xorm:"NOT NULL DEFAULT 'argon2'"`
		MustChangePassword bool   `xorm:"NOT NULL DEFAULT false"`
		LoginType          int
		LoginName          string
		Type               int
		Salt               string `xorm:"VARCHAR(10)"`
	}

	// hashPassword hash password based on algo and salt
	// state 461406070c
	hashPassword := func(passwd, salt, algo string) string {
		var tempPasswd []byte

		switch algo {
		case algoBcrypt:
			tempPasswd, _ = bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
			return string(tempPasswd)
		case algoScrypt:
			tempPasswd, _ = scrypt.Key([]byte(passwd), []byte(salt), 65536, 16, 2, 50)
		case algoArgon2:
			tempPasswd = argon2.IDKey([]byte(passwd), []byte(salt), 2, 65536, 8, 50)
		case algoPbkdf2:
			fallthrough
		default:
			tempPasswd = pbkdf2.Key([]byte(passwd), []byte(salt), 10000, 50, sha256.New)
		}

		return hex.EncodeToString(tempPasswd)
	}

	// ValidatePassword checks if given password matches the one belongs to the user.
	// state 461406070c, changed since it's not necessary to be time constant
	ValidatePassword := func(u *User, passwd string) bool {
		tempHash := hashPassword(passwd, u.Salt, u.PasswdHashAlgo)

		if u.PasswdHashAlgo != algoBcrypt && u.Passwd == tempHash {
			return true
		}
		if u.PasswdHashAlgo == algoBcrypt && bcrypt.CompareHashAndPassword([]byte(u.Passwd), []byte(passwd)) == nil {
			return true
		}
		return false
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100

	for start := 0; ; start += batchSize {
		users := make([]*User, 0, batchSize)
		if err = sess.Limit(batchSize, start).Where(builder.Neq{"passwd": ""}, 0).Find(&users); err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}

		if err = sess.Begin(); err != nil {
			return err
		}

		for _, user := range users {
			if ValidatePassword(user, "") {
				user.Passwd = ""
				user.Salt = ""
				user.PasswdHashAlgo = ""
				if _, err = sess.ID(user.ID).Cols("passwd", "salt", "passwd_hash_algo").Update(user); err != nil {
					return err
				}
			}
		}

		if err = sess.Commit(); err != nil {
			return err
		}
	}

	// delete salt and algo where password is empty
	_, err = sess.Where(builder.Eq{"passwd": ""}.And(builder.Neq{"salt": ""}.Or(builder.Neq{"passwd_hash_algo": ""}))).
		Cols("salt", "passwd_hash_algo").Update(&User{})

	return err
}
