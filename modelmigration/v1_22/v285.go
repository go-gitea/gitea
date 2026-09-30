// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"context"
	"time"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddPreviousDurationToActionRun(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		PreviousDuration time.Duration
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, &ActionRun{})
	return err
}
