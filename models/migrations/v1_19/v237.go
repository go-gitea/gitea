// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"fmt"
	"xorm.io/xorm"
)

// AddCardTypeToProjectTable: add CardType column, setting existing rows to CardTypeTextOnly
func AddCardTypeToProjectTable(x *xorm.Engine) error {
	type Project struct {
		CardType int `xorm:"NOT NULL DEFAULT 1"`
	}

	if err := x.Sync2(new(Project)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
