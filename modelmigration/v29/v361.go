// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func DropCommentCommitID(ctx context.Context, x base.EngineMigration) error {
	exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, "comment", "commit_id")
	if err != nil || !exists {
		return err
	}

	return base.EngineTransaction(x, func(sess base.Session) error {
		return base.DropTableColumns(sess, "comment", "commit_id")
	})
}
