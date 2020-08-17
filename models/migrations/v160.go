// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addProjectsIssuesBoardsPriority(x *xorm.Engine) error {
	// ProjectIssue saves relation from issue to a project
	type ProjectIssue struct {
		Priority int `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(ProjectIssue)); err != nil {
		return err
	}

	type ProjectBoard struct {
		Priority int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(ProjectBoard))
}
