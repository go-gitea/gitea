// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddProjectIssueSorting(_ context.Context, x base.EngineMigration) error {
	// ProjectIssue saves relation from issue to a project
	type ProjectIssue struct {
		Sorting int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(ProjectIssue))
}
