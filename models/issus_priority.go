// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"github.com/go-xorm/xorm"
)

const (
	// LowestPriority defines the lowest-level priority
	LowestPriority = iota + 1
	// LowPriority defines the low-level priority
	LowPriority
	// MediumPriority defines the medium-level priority
	MediumPriority
	// HighPriority defines the high-level priority
	HighPriority
	// HighestPriority defines the highest-level priority
	HighestPriority
)

// UpdateIssuePriority update priority for a specific issue
func UpdateIssuePriority(issue *Issue, doer *User) error {
	if err := issue.loadRepo(x); err != nil {
		return err
	}

	if has, err := HasAccess(doer.ID, issue.Repo, AccessModeWrite); err != nil {
		return err
	} else if !has {
		return ErrUserDoesNotHaveAccessToRepo{UserID: doer.ID, RepoName: issue.Repo.Name}
	}

	if issue.Priority > HighestPriority || issue.Priority < LowestPriority {
		return ErrIssueInvalidPriority{ID: issue.ID, RepoID: issue.Repo.ID, DesiredPriority: issue.Priority}
	}

	_, err := AutoTransaction(func(session *xorm.Session) (interface{}, error) {
		return nil, updateIssueCols(session, &Issue{ID: issue.ID, Priority: issue.Priority}, "priority")
	}, x)

	return err
}
