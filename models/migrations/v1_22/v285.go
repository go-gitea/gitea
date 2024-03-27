// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"time"

	"xorm.io/xorm"
)

func AddPreviousDurationToActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		PreviousDuration time.Duration
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, &ActionRun{})
	return err
}
