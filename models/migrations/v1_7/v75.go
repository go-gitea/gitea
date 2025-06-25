// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_7

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func ClearNonusedData(x *xorm.Engine) error {
	condDelete := func(colName string) builder.Cond {
		return builder.NotIn(colName, builder.Select("id").From("`user`"))
	}

	if _, err := x.Exec(builder.Delete(condDelete("uid")).From("team_user")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete("user_id")).From("collaboration")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete("user_id")).From("stopwatch")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete("owner_id")).From("gpg_key")); err != nil {
		return err
	}
	return nil
}
