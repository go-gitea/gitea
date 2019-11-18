// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func prependRefsHeadsToIssueRefs(x *xorm.Engine) error {
	var query string

	switch {
	case setting.Database.UseMSSQL:
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' + `ref` WHERE `ref` IS NOT NULL AND `ref` <> ''"
	default:
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' || `ref` WHERE `ref` IS NOT NULL AND `ref` <> ''"
	}

	_, err := x.Exec(query)
	return err
}
