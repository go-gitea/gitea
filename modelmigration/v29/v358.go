// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"codenest.dev/modelmigration/base"
	"codenest.dev/modules/timeutil"

	"xorm.io/xorm"
)

func AddPreviousAuthTokenHash(_ context.Context, x base.EngineMigration) error {
	type AuthToken struct {
		PreviousTokenHash string
		RotatedUnix       timeutil.TimeStamp
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(AuthToken))
	return err
}
