// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func PurgeInvalidDependenciesComments(x *xorm.Engine) error {
	_, err := x.Exec("DELETE FROM comment WHERE dependent_issue_id != 0 AND dependent_issue_id NOT IN (SELECT id FROM issue)")
	return err
}
