// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// DropActionRunJobReusableCallerIndex drops the index on action_run_job.is_reusable_caller.
// Only a small share of jobs are reusable callers and no query needs the index: every use
// of is_reusable_caller = true also filters by repo_id and concurrency_group, and the
// is_reusable_caller = false filters are not selective. SQLite still picks the index for
// `is_reusable_caller = false` and then reads nearly the whole table.
func DropActionRunJobReusableCallerIndex(ctx context.Context, x base.EngineMigration) error {
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "action_run_job")
	if err != nil {
		return err
	}
	idx, ok := indexes["is_reusable_caller"] // GetIndexes strips the IDX_action_run_job_ prefix
	if !ok {
		return nil
	}
	_, err = x.Exec(x.Dialect().DropIndexSQL("action_run_job", idx))
	return err
}
