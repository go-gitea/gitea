// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func RemoveIssueRef(_ctx context.Context, x base.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()
	return base.DropTableColumns(sess, "issue", "ref")
}
