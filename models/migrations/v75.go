// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
)

func clearNonusedData(x *xorm.Engine) error {
	condDelete := builder.NotIn("user_id", builder.Select("id").From("user"))
	if _, err := x.Exec(builder.Delete(condDelete).From("team_user")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete).From("collaboration")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete).From("stop_watch")); err != nil {
		return err
	}

	if _, err := x.Exec(builder.Delete(condDelete).From("gpg_key")); err != nil {
		return err
	}
	return nil
}
