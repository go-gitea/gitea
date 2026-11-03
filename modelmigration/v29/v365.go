// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddIsConfidentialToIssue(_ context.Context, x base.EngineMigration) error {
	type Issue struct {
		ID             int64 `xorm:"pk autoincr"`
		RepoID         int64 `xorm:"INDEX(repo_confidential)"`
		IsConfidential bool  `xorm:"INDEX(repo_confidential) NOT NULL DEFAULT false"`
	}

	return x.Sync(new(Issue))
}
