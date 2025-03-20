// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_6 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

func AddScratchHash(x *xorm.Engine) error {
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

	if err := x.Sync(new(TwoFactor)); err != nil {
		return fmt.Errorf("Sync: %w", err)
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
			salt, err := util.CryptoRandomString(10)
			if err != nil {
				return err
			}
			tfa.ScratchSalt = salt
			tfa.ScratchHash = base.HashToken(tfa.ScratchToken, salt)

			if _, err := sess.ID(tfa.ID).Cols("scratch_salt, scratch_hash").Update(tfa); err != nil {
				return fmt.Errorf("couldn't add in scratch_hash and scratch_salt: %w", err)
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

	if err := base.DropTableColumns(sess, "two_factor", "scratch_token"); err != nil {
		return err
	}
	return sess.Commit()
}
