// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRunnerNameToActionTask(_ context.Context, x base.EngineMigration) error {
	type ActionTask struct {
		RunnerName string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionTask)); err != nil {
		return err
	}

	// existing tasks have no recorded runner name, so copy it from the runners that still exist (including soft-deleted ones)
	_, err := x.Exec(`
UPDATE action_task SET runner_name = (
	SELECT action_runner.name FROM action_runner WHERE action_runner.id = action_task.runner_id
)
WHERE runner_name = ''
	AND EXISTS (
		SELECT 1 FROM action_runner WHERE action_runner.id = action_task.runner_id AND action_runner.name IS NOT NULL
	);`)
	return err
}
