// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"xorm.io/xorm"
)

// AddCardTypeToProjectTable: add CardType column, setting existing rows to CardTypeTextOnly
func AddCardTypeToProjectTable(x *xorm.Engine) error {
	type Project struct {
		CardType int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(Project))
}
