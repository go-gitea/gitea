// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

func AddUserSessionTable(x *xorm.Engine) error {
	type UserSession struct {
		ID             string `xorm:"pk VARCHAR(64)"`
		UserID         int64  `xorm:"INDEX NOT NULL"`
		LoginIP        string `xorm:"VARCHAR(45)"`
		LastIP         string `xorm:"VARCHAR(45)"`
		PrevIP         string `xorm:"VARCHAR(45)"`
		UserAgent      string `xorm:"TEXT"`
		LoginMethod    string `xorm:"VARCHAR(64)"`
		AuthTokenID    string `xorm:"VARCHAR(64)"`
		CreatedUnix    int64  `xorm:"INDEX NOT NULL"`
		LastAccessUnix int64  `xorm:"INDEX NOT NULL"`
		LogoutUnix     int64  `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(UserSession))
	return err
}
