// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// AddIssueRedirectTable adds the issue_redirect table to support URL redirects after issue moves.
func AddIssueRedirectTable(_ context.Context, x base.EngineMigration) error {
	type IssueRedirect struct {
		ID             int64 `xorm:"pk autoincr"`
		OriginalRepoID int64 `xorm:"INDEX NOT NULL"`
		OriginalIndex  int64 `xorm:"NOT NULL"`
		IssueID        int64 `xorm:"INDEX NOT NULL"`
	}
	return x.Sync(new(IssueRedirect))
}
