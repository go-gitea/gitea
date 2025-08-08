// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddUserSSHKeypairTable(x *xorm.Engine) error {
	type UserSSHKeypair struct {
		ID                  int64              `xorm:"pk autoincr"`
		OwnerID             int64              `xorm:"INDEX NOT NULL"`
		PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
		PublicKey           string             `xorm:"TEXT NOT NULL"`
		Fingerprint         string             `xorm:"VARCHAR(255) UNIQUE NOT NULL"`
		CreatedUnix         timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(UserSSHKeypair))
}
