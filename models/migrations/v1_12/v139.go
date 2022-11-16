// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func PrependRefsHeadsToIssueRefs(x *xorm.Engine) error {
	var query string

	switch {
	case setting.Database.UseMSSQL:
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' + `ref` WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%'"
	case setting.Database.UseMySQL:
		query = "UPDATE `issue` SET `ref` = CONCAT('refs/heads/', `ref`) WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%';"
	default:
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' || `ref` WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%'"
	}

	_, err := x.Exec(query)
	return err
}
