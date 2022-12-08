// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
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
func LockIssue(opts *IssueLockOptions) error {
	return updateIssueLock(opts, true)
}

// UnlockIssue unlocks a previously locked issue.
func UnlockIssue(opts *IssueLockOptions) error {
	return updateIssueLock(opts, false)
}

func updateIssueLock(opts *IssueLockOptions, lock bool) error {
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

	ctx, committer, err := db.TxContext(db.DefaultContext)
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
	if _, err := CreateCommentCtx(ctx, opt); err != nil {
		return err
	}

	return committer.Commit()
}
