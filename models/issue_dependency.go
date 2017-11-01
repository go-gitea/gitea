// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
)

// IssueDependency is connection request for receiving issue notification.
type IssueDependency struct {
	ID           int64     `xorm:"pk autoincr"`
	UserID       int64     `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID      int64     `xorm:"UNIQUE(watch) NOT NULL"`
	DependencyID int64     `xorm:"UNIQUE(watch) NOT NULL"`
	Created      time.Time `xorm:"-"`
	CreatedUnix  int64     `xorm:"INDEX created"`
	Updated      time.Time `xorm:"-"`
	UpdatedUnix  int64     `xorm:"updated"`
}

// DependencyType Defines Dependency Type Constants
type DependencyType int

// Define Dependency Types
const (
	DependencyTypeBlockedBy DependencyType = iota
	DependencyTypeBlocking
)

// CreateIssueDependency creates a new dependency for an issue
func CreateIssueDependency(user *User, issue, dep *Issue) (exists bool, err error) {
	sess := x.NewSession()

	// Check if it aleready exists
	exists, err = issueDepExists(x, issue.ID, dep.ID)
	if err != nil {
		return exists, err
	}

	// If it not exists, create it, otherwise show an error message
	if !exists {
		newIssueDependency := &IssueDependency{
			UserID:       user.ID,
			IssueID:      issue.ID,
			DependencyID: dep.ID,
		}

		if _, err := x.Insert(newIssueDependency); err != nil {
			return exists, err
		}

		// Add comment referencing the new dependency
		if _, err = createIssueDependencyComment(sess, user, issue, dep, true); err != nil {
			return exists, err
		}

		// Create a new comment for the dependent issue
		if _, err = createIssueDependencyComment(sess, user, dep, issue, true); err != nil {
			return exists, err
		}
	}
	return exists, nil
}

// RemoveIssueDependency removes a dependency from an issue
func RemoveIssueDependency(user *User, issue *Issue, dep *Issue, depType DependencyType) (err error) {
	sess := x.NewSession()

	// Check if it exists
	var exists bool
	if exists, err = issueDepExists(x, issue.ID, dep.ID); err != nil {
		return err
	}

	// If it exists, remove it, otherwise show an error message
	if exists {

		var issueDepToDelete IssueDependency

		switch depType {
		case DependencyTypeBlockedBy:
			issueDepToDelete = IssueDependency{IssueID: issue.ID, DependencyID: dep.ID}
		case DependencyTypeBlocking:
			issueDepToDelete = IssueDependency{IssueID: dep.ID, DependencyID: issue.ID}
		default:
			return
		}

		if _, err := x.Delete(&issueDepToDelete); err != nil {
			return err
		}

		// Add comment referencing the removed dependency
		if _, err = createIssueDependencyComment(sess, user, issue, dep, false); err != nil {
			return err
		}

		// Create a new comment for the dependent issue
		if _, err = createIssueDependencyComment(sess, user, dep, issue, false); err != nil {
			return err
		}
	}
	return nil
}

// Check if the dependency already exists
func issueDepExists(e Engine, issueID int64, depID int64) (exists bool, err error) {

	exists, err = e.Where("(issue_id = ? AND dependency_id = ?) OR (issue_id = ? AND dependency_id = ?)", issueID, depID, depID, issueID).Exist(&IssueDependency{})

	return
}

// IssueDependencyIssue custom type for mysql join
type IssueDependencyIssue struct {
	IssueDependency `xorm:"extends"`
	Issue           `xorm:"extends"`
}

// TableName returns table name for mysql join
func (IssueDependencyIssue) TableName() string {
	return "issue_dependency"
}

// IssueNoDependenciesLeft checks if issue can be closed
func IssueNoDependenciesLeft(issue *Issue) (exists bool, err error) {

	exists, err = x.
		Join("INNER", "issue", "issue.id = issue_dependency.dependency_id").
		Where("issue_dependency.issue_id = ?", issue.ID).
		And("issue.is_closed = ?", "0").
		Exist(&IssueDependencyIssue{})

	return !exists, err
}
