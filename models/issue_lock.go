// Copyright 2019 The Gitea Authors. All rights reserved.
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

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := updateIssueCols(sess, opts.Issue, "is_locked"); err != nil {
		return err
	}

	var opt = &CreateCommentOptions{
		Doer:    opts.Doer,
		Issue:   opts.Issue,
		Repo:    opts.Issue.Repo,
		Type:    commentType,
		Content: opts.Reason,
	}
	if _, err := createComment(sess, opt); err != nil {
		return err
	}

	return sess.Commit()
}
