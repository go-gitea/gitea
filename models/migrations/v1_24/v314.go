// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import "gitea.dev/models/db"

func UpdateOwnerIDOfRepoLevelActionsTables(x db.EngineMigration) error {
	if _, err := x.Exec("UPDATE `action_runner` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0"); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `action_variable` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0"); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE `secret` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0")
	return err
}
