// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"time"

	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func DropColumnsFromExternalLoginUserTable(x *xorm.Engine) error {
	type ExternalLoginUser struct {
		RawData           map[string]any `xorm:"TEXT JSON"`
		AccessToken       string         `xorm:"TEXT"`
		AccessTokenSecret string         `xorm:"TEXT"`
		RefreshToken      string         `xorm:"TEXT"`
		ExpiresAt         time.Time
	}
	if err := x.Sync(new(ExternalLoginUser)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := base.DropTableColumns(sess, "external_login_user", "raw_data", "access_token", "access_token_secret", "refresh_token", "expires_at"); err != nil {
		return err
	}

	return sess.Commit()
}
