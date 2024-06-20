// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"strings"

	"xorm.io/xorm"
)

func ConvertAuthorIDToString(x *xorm.Engine) error {
	// Google OAuth2 provider may give very long user IDs
	sql := strings.Join([]string{
		"ALTER TABLE issue ALTER COLUMN original_author_id TYPE VARCHAR(255) USING original_author_id::VARCHAR;",
		"ALTER TABLE comment ALTER COLUMN original_author_id TYPE VARCHAR(255) USING original_author_id::VARCHAR;",
		"ALTER TABLE release ALTER COLUMN original_author_id TYPE VARCHAR(255) USING original_author_id::VARCHAR;",
		"ALTER TABLE reaction ALTER COLUMN original_author_id TYPE VARCHAR(255) USING original_author_id::VARCHAR;",
		"ALTER TABLE review ALTER COLUMN original_author_id TYPE VARCHAR(255) USING original_author_id::VARCHAR;",
	}, " ")

	_, err := x.Exec(sql)
	return err
}
