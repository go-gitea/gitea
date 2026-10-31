// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddFailedAttemptsToTwoFactor(_ context.Context, x base.EngineMigration) error {
	type TwoFactor struct {
		FailedAttempts     int   `xorm:"NOT NULL DEFAULT 0"`
		FailureWindowUntil int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(TwoFactor))
	return err
}
