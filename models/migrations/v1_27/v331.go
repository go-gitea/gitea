// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

func AddUsageToPublicKey(x *xorm.Engine) error {
	type PublicKey struct {
		Usage int `xorm:"NOT NULL DEFAULT 3"`
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(PublicKey)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE public_key SET `usage` = 3 WHERE `usage` IS NULL OR `usage` = 0")
	return err
}
