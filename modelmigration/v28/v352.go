// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func RemoveIssueRef(_ context.Context, x base.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()
	return base.DropTableColumns(sess, "issue", "ref")
}
