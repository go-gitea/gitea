// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"xorm.io/xorm"
)

func UpdateOwnerIDOfRepoLevelRunners(x *xorm.Engine) error {
	if _, err := x.Exec("UPDATE `action_runner` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0"); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `action_runner` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0"); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE `action_runner` SET `owner_id` = 0 WHERE `repo_id` > 0 AND `owner_id` > 0")
	return err
}
