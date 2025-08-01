// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"xorm.io/xorm"
)

func PurgeInvalidDependenciesComments(x *xorm.Engine) error {
	_, err := x.Exec("DELETE FROM comment WHERE dependent_issue_id != 0 AND dependent_issue_id NOT IN (SELECT id FROM issue)")
	return err
}
