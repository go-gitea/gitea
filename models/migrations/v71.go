// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/sha256"
	"fmt"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/timeutil"

	"golang.org/x/crypto/pbkdf2"
	"xorm.io/xorm"
)

func addScratchHash(x *xorm.Engine) error {
	// TwoFactor see models/twofactor.go
	type TwoFactor struct {
		ID               int64 `xorm:"pk autoincr"`
		UID              int64 `xorm:"UNIQUE"`
		Secret           string
		ScratchToken     string
		ScratchSalt      string
		ScratchHash      string
		LastUsedPasscode string             `xorm:"VARCHAR(10)"`
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix      timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync2(new(TwoFactor)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	// transform all tokens to hashes
	const batchSize = 100
	for start := 0; ; start += batchSize {
		tfas := make([]*TwoFactor, 0, batchSize)
		if err := sess.Limit(batchSize, start).Find(&tfas); err != nil {
			return err
		}
		if len(tfas) == 0 {
			break
		}

		for _, tfa := range tfas {
			// generate salt
			salt, err := generate.GetRandomString(10)
			if err != nil {
				return err
			}
			tfa.ScratchSalt = salt
			tfa.ScratchHash = hashToken(tfa.ScratchToken, salt)

			if _, err := sess.ID(tfa.ID).Cols("scratch_salt, scratch_hash").Update(tfa); err != nil {
				return fmt.Errorf("couldn't add in scratch_hash and scratch_salt: %v", err)
			}

		}
	}

	// Commit and begin new transaction for dropping columns
	if err := sess.Commit(); err != nil {
		return err
	}
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "two_factor", "scratch_token"); err != nil {
		return err
	}
	return sess.Commit()

}

func hashToken(token, salt string) string {
	tempHash := pbkdf2.Key([]byte(token), []byte(salt), 10000, 50, sha256.New)
	return fmt.Sprintf("%x", tempHash)
}
