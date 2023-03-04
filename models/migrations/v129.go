// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func purgeUnusedDependencies(x *xorm.Engine) error {
	if _, err := x.Exec("DELETE FROM issue_dependency WHERE issue_id NOT IN (SELECT id FROM issue)"); err != nil {
		return err
	}
	_, err := x.Exec("DELETE FROM issue_dependency WHERE dependency_id NOT IN (SELECT id FROM issue)")
	return err
}
