// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddScheduleIDForActionRun(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		ScheduleID int64
	}
	return x.Sync(new(ActionRun))
}
