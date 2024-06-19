// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"strings"

	"xorm.io/xorm"
)

func changeOriginalAuthorIDDataTypeToNumeric(x *xorm.Engine) error {
	sql := strings.Join([]string{
		"ALTER TABLE `issue` ALTER COLUMN `original_author_id` TYPE NUMERIC USING `original_author_id`::NUMERIC",
		"ALTER TABLE `comment` ALTER COLUMN `original_author_id` TYPE NUMERIC USING `original_author_id`::NUMERIC",
		"ALTER TABLE `release` ALTER COLUMN `original_author_id` TYPE NUMERIC USING `original_author_id`::NUMERIC",
		"ALTER TABLE `reaction` ALTER COLUMN `original_author_id` TYPE NUMERIC USING `original_author_id`::NUMERIC",
		"ALTER TABLE `review` ALTER COLUMN `original_author_id` TYPE NUMERIC USING `original_author_id`::NUMERIC",
	}, "; ")

	_, err := x.Exec(sql)
	return err
}
