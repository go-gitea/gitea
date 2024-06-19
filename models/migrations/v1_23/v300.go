// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func ConvertAuthorIDToNumeric(x *xorm.Engine) error {
	// Google OAuth2 provider may give very long user IDs
	if !setting.Database.Type.IsPostgreSQL() {
		return nil
	}
	sql := strings.Join([]string{
		"ALTER TABLE issue ALTER COLUMN original_author_id TYPE NUMERIC USING original_author_id::NUMERIC;",
		"ALTER TABLE comment ALTER COLUMN original_author_id TYPE NUMERIC USING original_author_id::NUMERIC;",
		"ALTER TABLE release ALTER COLUMN original_author_id TYPE NUMERIC USING original_author_id::NUMERIC;",
		"ALTER TABLE reaction ALTER COLUMN original_author_id TYPE NUMERIC USING original_author_id::NUMERIC;",
		"ALTER TABLE review ALTER COLUMN original_author_id TYPE NUMERIC USING original_author_id::NUMERIC;",
	}, " ")

	_, err := x.Exec(sql)
	return err
}
