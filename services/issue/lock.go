// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "gitea.dev/models/issues"
	notify_service "gitea.dev/services/notify"
)

// LockIssue locks an issue and notifies subscribers after a successful change.
func LockIssue(ctx context.Context, opts *issues_model.IssueLockOptions) error {
	return changeIssueLock(ctx, opts, true)
}

// UnlockIssue unlocks an issue and notifies subscribers after a successful change.
func UnlockIssue(ctx context.Context, opts *issues_model.IssueLockOptions) error {
	return changeIssueLock(ctx, opts, false)
}

func changeIssueLock(ctx context.Context, opts *issues_model.IssueLockOptions, locked bool) error {
	if opts.Issue.IsLocked == locked {
		return nil
	}

	var err error
	if locked {
		err = issues_model.LockIssue(ctx, opts)
	} else {
		err = issues_model.UnlockIssue(ctx, opts)
	}
	if err != nil {
		return err
	}

	notify_service.IssueChangeLock(ctx, opts.Doer, opts.Issue, locked)
	return nil
}
