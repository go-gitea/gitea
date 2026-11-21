// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// AddWithdrawReasonToDisclosure adds the withdraw_reason column to the disclosure table
// and renames the "none" severity to "informative".
func AddWithdrawReasonToDisclosure(_ context.Context, x base.EngineMigration) error {
	type Disclosure struct {
		WithdrawReason string `xorm:"TEXT"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Disclosure)); err != nil {
		return err
	}
	_, err := x.Exec(builder.Update(builder.Eq{"severity": "informative"}).From("`disclosure`").Where(builder.Eq{"severity": "none"}))
	return err
}
