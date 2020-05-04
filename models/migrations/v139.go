// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addRepositoryLastIssueIndexColumn(x *xorm.Engine) error {
	type Repository struct {
		LastIssueIndex int64 `xorm:"DEFAULT(0) NOT NULL"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	_, err := x.Exec("UPDATE `repository` SET `last_issue_index` = (SELECT max(`index`) FROM `issue` WHERE `issue`.`repo_id` = `repository`.`id`)")
	return err
}
