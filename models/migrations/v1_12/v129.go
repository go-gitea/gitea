// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"xorm.io/xorm"
)

func PurgeUnusedDependencies(x *xorm.Engine) error {
	if _, err := x.Exec("DELETE FROM issue_dependency WHERE issue_id NOT IN (SELECT id FROM issue)"); err != nil {
		return err
	}
	_, err := x.Exec("DELETE FROM issue_dependency WHERE dependency_id NOT IN (SELECT id FROM issue)")
	return err
}
