// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func ConvertCommitStatusStateIntoInt(x *xorm.Engine) error {
	// CommitStatusState holds the state of a CommitStatus
	// It can be "pending", "success", "error" and "failure"
	type CommitStatusState int

	const (
		// CommitStatusError is for when the CommitStatus is Error
		CommitStatusError CommitStatusState = iota + 1
		// CommitStatusFailure is for when the CommitStatus is Failure
		CommitStatusFailure
		// CommitStatusPending is for when the CommitStatus is Pending
		CommitStatusPending
		// CommitStatusSuccess is for when the CommitStatus is Success
		CommitStatusSuccess
	)

	// CommitStatus holds a single Status of a single Commit
	type CommitStatus struct {
		State CommitStatusState `xorm:"INDEX NOT NULL"`
	}

	commitStatusConvertMap := map[string]CommitStatusState{
		"error":   CommitStatusError,
		"failure": CommitStatusFailure,
		"pending": CommitStatusPending,
		"success": CommitStatusSuccess,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(CommitStatus)); err != nil {
		return err
	}

	for origin, target := range commitStatusConvertMap {
		count, err := sess.Where("`state` = ?", origin).Update(&CommitStatus{State: target})
		if err != nil {
			return err
		}
		log.Debug("Updated %d commit status with %s status", count, origin)
	}

	return sess.Commit()
}
