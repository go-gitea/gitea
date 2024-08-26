// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"
)

// CommentMetaData stores metadata for a comment, these data will not be changed once inserted into database
type CommentMetaData struct {
	ProjectColumnID    int64  `json:"project_column_id"`
	ProjectColumnTitle string `json:"project_column_title"`
	ProjectTitle       string `json:"project_title"`
}

func AddCommentMetaDataColumn(x *xorm.Engine) error {
	type Comment struct {
		CommentMetaData *CommentMetaData `xorm:"JSON TEXT"` // put all non-index metadata in a single field
	}

	return x.Sync(new(Comment))
}
