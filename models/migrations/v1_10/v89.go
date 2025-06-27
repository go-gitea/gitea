// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "xorm.io/xorm"

func AddOriginalMigrationInfo(x *xorm.Engine) error {
	// Issue see models/issue.go
	type Issue struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Issue)); err != nil {
		return err
	}

	// Issue see models/issue_comment.go
	type Comment struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Comment)); err != nil {
		return err
	}

	// Issue see models/repo.go
	type Repository struct {
		OriginalURL string
	}

	return x.Sync(new(Repository))
}
