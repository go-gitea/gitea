// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"time"

	"xorm.io/xorm"
)

func changeGPGKeysColumns(x *xorm.Engine) error {
	// EmailAddress is the list of all email addresses of a user. Can contain the
	// primary email address, but is not obligatory.
	type EmailAddress struct {
		ID          int64  `xorm:"pk autoincr"`
		UID         int64  `xorm:"INDEX NOT NULL"`
		Email       string `xorm:"UNIQUE NOT NULL"`
		IsActivated bool
		IsPrimary   bool `xorm:"-"`
	}

	// GPGKey represents a GPG key.
	type GPGKey struct {
		ID                int64     `xorm:"pk autoincr"`
		OwnerID           int64     `xorm:"INDEX NOT NULL"`
		KeyID             string    `xorm:"INDEX CHAR(16) NOT NULL"`
		PrimaryKeyID      string    `xorm:"CHAR(16)"`
		Content           string    `xorm:"TEXT NOT NULL"`
		Created           time.Time `xorm:"-"`
		CreatedUnix       int64
		Expired           time.Time `xorm:"-"`
		ExpiredUnix       int64
		Added             time.Time `xorm:"-"`
		AddedUnix         int64
		SubsKey           []*GPGKey `xorm:"-"`
		Emails            []*EmailAddress
		CanSign           bool
		CanEncryptComms   bool
		CanEncryptStorage bool
		CanCertify        bool
	}

	if err := x.DropTables(new(GPGKey)); err != nil {
		return err
	}

	return x.Sync(new(GPGKey))
}
