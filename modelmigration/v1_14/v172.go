// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"context"

	"codenest.dev/modelmigration/base"
	"codenest.dev/modules/timeutil"
)

func AddSessionTable(_ context.Context, x base.EngineMigration) error {
	type Session struct {
		Key    string `xorm:"pk CHAR(16)"`
		Data   []byte `xorm:"BLOB"`
		Expiry timeutil.TimeStamp
	}
	return x.Sync(new(Session))
}
