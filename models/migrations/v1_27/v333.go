// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"code.gitea.io/gitea/models/actions"

	"xorm.io/xorm"
)

func AddActionRunJobSummaryTable(ctx context.Context, x *xorm.Engine) error {
	return x.Sync(new(actions.ActionRunJobSummary))
}
