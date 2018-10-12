// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// IssueLockOptions defines options for locking and/or unlocking an issue/PR
type IssueLockOptions struct {
	Doer   *User
	Issue  *Issue
	Reason string
}

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo
func LockIssue(opts *IssueLockOptions) error {
	opts.Issue.IsLocked = true
	return lockOrUnlockIssue(opts, CommentTypeLock)
}

// UnlockIssue unlocks a previously locked issue.
func UnlockIssue(opts *IssueLockOptions) error {
	opts.Issue.IsLocked = false
	return lockOrUnlockIssue(opts, CommentTypeUnlock)
}

func lockOrUnlockIssue(opts *IssueLockOptions, commentType CommentType) error {
	if err := UpdateIssueCols(opts.Issue, "is_locked"); err != nil {
		return err
	}

	_, err := CreateComment(&CreateCommentOptions{
		Doer:    opts.Doer,
		Issue:   opts.Issue,
		Repo:    opts.Issue.Repo,
		Type:    commentType,
		Content: opts.Reason,
	})
	return err
}
