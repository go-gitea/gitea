// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func UpdateOwnerIDOfRepoLevelRunnerTokens(_ctx context.Context, x base.EngineMigration) error {
	// inactive rows can't register, a newer one moved into scope would become the latest token and trigger an auto-reset
	_, err := x.Exec("UPDATE `action_runner_token` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0 AND `is_active` = ?", true)
	return err
}
