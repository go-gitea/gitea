// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func FixMissedRepoIDWhenMigrateAttachments(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE `attachment` SET `repo_id` = (SELECT `repo_id` FROM `issue` WHERE `issue`.`id` = `attachment`.`issue_id`) WHERE `issue_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0);")
	if err != nil {
		return err
	}

	_, err = x.Exec("UPDATE `attachment` SET `repo_id` = (SELECT `repo_id` FROM `release` WHERE `release`.`id` = `attachment`.`release_id`) WHERE `release_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0);")
	return err
}
