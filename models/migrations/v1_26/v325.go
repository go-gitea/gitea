// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddRunnerCapacityAndJobMaxParallel(x *xorm.Engine) error {
	type ActionRunner struct {
		Capacity int `xorm:"NOT NULL DEFAULT 1"`
	}

	type ActionRunJob struct {
		MatrixID    string `xorm:"VARCHAR(255) INDEX"`
		MaxParallel int
	}

	if err := x.Sync(new(ActionRunner)); err != nil {
		return err
	}

	return x.Sync(new(ActionRunJob))
}
