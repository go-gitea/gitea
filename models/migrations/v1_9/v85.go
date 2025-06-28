// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"fmt"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

func HashAppToken(x *xorm.Engine) error {
	// AccessToken see models/token.go
	type AccessToken struct {
		ID             int64 `xorm:"pk autoincr"`
		UID            int64 `xorm:"INDEX"`
		Name           string
		Sha1           string
		Token          string `xorm:"-"`
		TokenHash      string // sha256 of token - we will ensure UNIQUE later
		TokenSalt      string
		TokenLastEight string `xorm:"token_last_eight"`

		CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
		HasRecentActivity bool               `xorm:"-"`
		HasUsed           bool               `xorm:"-"`
	}

	// First remove the index
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(AccessToken)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}

	if err := sess.Commit(); err != nil {
		return err
	}

	if err := sess.Begin(); err != nil {
		return err
	}

	// transform all tokens to hashes
	const batchSize = 100
	for start := 0; ; start += batchSize {
		tokens := make([]*AccessToken, 0, batchSize)
		if err := sess.Limit(batchSize, start).Find(&tokens); err != nil {
			return err
		}
		if len(tokens) == 0 {
			break
		}

		for _, token := range tokens {
			// generate salt
			salt, err := util.CryptoRandomString(10)
			if err != nil {
				return err
			}
			token.TokenSalt = salt
			token.TokenHash = base.HashToken(token.Sha1, salt)
			if len(token.Sha1) < 8 {
				log.Warn("Unable to transform token %s with name %s belonging to user ID %d, skipping transformation", token.Sha1, token.Name, token.UID)
				continue
			}
			token.TokenLastEight = token.Sha1[len(token.Sha1)-8:]
			token.Sha1 = "" // ensure to blank out column in case drop column doesn't work

			if _, err := sess.ID(token.ID).Cols("token_hash, token_salt, token_last_eight, sha1").Update(token); err != nil {
				return fmt.Errorf("couldn't add in sha1, token_hash, token_salt and token_last_eight: %w", err)
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

	if err := base.DropTableColumns(sess, "access_token", "sha1"); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}
	return resyncHashAppTokenWithUniqueHash(x)
}

func resyncHashAppTokenWithUniqueHash(x *xorm.Engine) error {
	// AccessToken see models/token.go
	type AccessToken struct {
		TokenHash string `xorm:"UNIQUE"` // sha256 of token - we will ensure UNIQUE later
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := sess.Sync(new(AccessToken)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return sess.Commit()
}
