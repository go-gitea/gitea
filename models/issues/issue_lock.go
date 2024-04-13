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
	Doer   *user_model.User
	Issue  *Issue
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

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

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
	if _, err := CreateComment(ctx, opt); err != nil {
		return err
	}

	return committer.Commit()
}
