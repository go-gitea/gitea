// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// IssueLockOptions defines options for locking and/or unlocking an issue/PR
type IssueLockOptions struct {
	Doer  *user_model.User
	Issue *Issue

	// Reason is the doer-provided comment message for the locked issue
	// GitHub doesn't support changing the "reasons" by config file, so GitHub has pre-defined "reason" enum values.
	// Gitea is not like GitHub, it allows site admin to define customized "reasons" in the config file.
	// So the API caller might not know what kind of "reasons" are valid, and the customized reasons are not translatable.
	// To make things clear and simple: doer have the chance to use any reason they like, we do not do validation.
	Reason string
}

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo
func LockIssue(ctx context.Context, opts *IssueLockOptions) error {
	return updateIssueLock(ctx, opts, true)
}

// UnlockIssue unlocks a previously locked issue.
func UnlockIssue(ctx context.Context, opts *IssueLockOptions) error {
	return updateIssueLock(ctx, opts, false)
}

func updateIssueLock(ctx context.Context, opts *IssueLockOptions, lock bool) error {
	if opts.Issue.IsLocked == lock {
		return nil
	}

	opts.Issue.IsLocked = lock
	var commentType CommentType
	if opts.Issue.IsLocked {
		commentType = CommentTypeLock
	} else {
		commentType = CommentTypeUnlock
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := UpdateIssueCols(ctx, opts.Issue, "is_locked"); err != nil {
			return err
		}

		opt := &CreateCommentOptions{
			Doer:    opts.Doer,
			Issue:   opts.Issue,
			Repo:    opts.Issue.Repo,
			Type:    commentType,
			Content: opts.Reason,
		}
		_, err := CreateComment(ctx, opt)
		return err
	})
}
