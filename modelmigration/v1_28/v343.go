// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import "gitea.dev/modelmigration/base"

func AddSpentOnUnixToTrackedTime(x base.EngineMigration) error {
	type TrackedTime struct {
		SpentOnUnix int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}
	if err := x.Sync(new(TrackedTime)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE tracked_time SET spent_on_unix = created_unix WHERE spent_on_unix = 0")
	return err
}
