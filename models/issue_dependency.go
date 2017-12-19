// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"time"
)

// IssueDependency represents an issue dependency
type IssueDependency struct {
	ID           int64     `xorm:"pk autoincr"`
	UserID       int64     `xorm:"NOT NULL"`
	IssueID      int64     `xorm:"UNIQUE(issue_dependency) NOT NULL"`
	DependencyID int64     `xorm:"UNIQUE(issue_dependency) NOT NULL"`
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
func CreateIssueDependency(user *User, issue, dep *Issue) (exists, circular bool, err error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return false, false, err
	}
	defer sess.Close()

	// Check if it aleready exists
	exists, circular, err = issueDepExists(sess, issue.ID, dep.ID)
	if err != nil {
		return
	}

	// If it not exists, create it, otherwise show an error message
	if !exists && !circular {
		newIssueDependency := &IssueDependency{
			UserID:       user.ID,
			IssueID:      issue.ID,
			DependencyID: dep.ID,
		}

		if _, err := sess.Insert(newIssueDependency); err != nil {
			return exists, circular, err
		}

		// Add comment referencing the new dependency
		if _, err = createIssueDependencyComment(sess, user, issue, dep, true); err != nil {
			return
		}

		// Create a new comment for the dependent issue
		if _, err = createIssueDependencyComment(sess, user, dep, issue, true); err != nil {
			return
		}
	}
	return exists, circular, sess.Commit()
}

// RemoveIssueDependency removes a dependency from an issue
func RemoveIssueDependency(user *User, issue *Issue, dep *Issue, depType DependencyType) (err error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	// Check if it exists
	var exists bool
	switch depType {
	case DependencyTypeBlockedBy:
		exists, _, err = issueDepExists(sess, issue.ID, dep.ID)
	case DependencyTypeBlocking:
		exists, _, err = issueDepExists(sess, dep.ID, issue.ID)
	default:
		return
	}
	if err != nil {
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

		if _, err := sess.Delete(&issueDepToDelete); err != nil {
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
	return sess.Commit()
}

// Check if the dependency already exists
func issueDepExists(e Engine, issueID int64, depID int64) (exists, circular bool, err error) {

	// Check if the dependency exists
	exists, err = e.Where("(issue_id = ? AND dependency_id = ?)", issueID, depID).Exist(&IssueDependency{})

	// If not, check for circular dependencies
	if !exists {
		circular, err = e.Where("issue_id = ? AND dependency_id = ?", depID, issueID).Exist(&IssueDependency{})
	}

	return
}

// IssueNoDependenciesLeft checks if issue can be closed
func IssueNoDependenciesLeft(issue *Issue) (bool, error) {

	exists, err := x.
		Table("issue_dependency").
		Select("issue.*").
		Join("INNER", "issue", "issue.id = issue_dependency.dependency_id").
		Where("issue_dependency.issue_id = ?", issue.ID).
		And("issue.is_closed = ?", "0").
		Exist(&Issue{})

	return !exists, err
}

// IsDependenciesEnabled returns if dependecies are enabled and returns the default setting if not set.
func (repo *Repository) IsDependenciesEnabled() bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(UnitTypeIssues); err != nil {
		return setting.Service.DefaultEnableDependencies
	}
	return u.IssuesConfig().EnableDependencies
}
