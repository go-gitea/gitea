// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func RemoveIssueRef(ctx context.Context, x base.EngineMigration) error {
	exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, "issue", "ref")
	if err != nil || !exists {
		return err
	}

	return base.EngineTransaction(x, func(sess base.Session) error {
		return base.DropTableColumns(sess, "issue", "ref")
	})
}
