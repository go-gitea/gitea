// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
<<<<<<< HEAD
=======
	"xorm.io/xorm/schemas"
>>>>>>> 8e22da2299 (enhance: add colors to reputation labels)
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

// AddColorToBadges adds the display color for manually managed labels.
func AddColorToBadges(_ context.Context, x base.EngineMigration) error {
	type Badge struct {
		Color string
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Badge))
	return err
}
