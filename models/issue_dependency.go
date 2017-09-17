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
	CreatedUnix  int64     `xorm:"NOT NULL"`
	Updated      time.Time `xorm:"-"`
	UpdatedUnix  int64     `xorm:"NOT NULL"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (iw *IssueDependency) BeforeInsert() {
	var (
		t = time.Now()
		u = t.Unix()
	)
	iw.Created = t
	iw.CreatedUnix = u
	iw.Updated = t
	iw.UpdatedUnix = u
}

// BeforeUpdate is invoked from XORM before updating an object of this type.
func (iw *IssueDependency) BeforeUpdate() {
	var (
		t = time.Now()
		u = t.Unix()
	)
	iw.Updated = t
	iw.UpdatedUnix = u
}

// CreateIssueDependency creates a new dependency for an issue
func CreateIssueDependency(user *User, issue, dep *Issue) (err error, exists bool) {
	sess := x.NewSession()

	// TODO: Move this to the appropriate place
	err = x.Sync(new(IssueDependency))
	if err != nil {
		return err, exists
	}

	// Check if it aleready exists
	exists, err = issueDepExists(x, issue.ID, dep.ID)
	if err != nil {
		return err, exists
	}

	// If it not exists, create it, otherwise show an error message
	if !exists {
		newId := new(IssueDependency)
		newId.UserID = user.ID
		newId.IssueID = issue.ID
		newId.DependencyID = dep.ID

		if _, err := x.Insert(newId); err != nil {
			return err, exists
		}

		// Add comment referencing the new dependency
		_, err = createIssueDependencyComment(sess, user, issue, dep, true)

		if err != nil {
			return err, exists
		}

		// Create a new comment for the dependent issue
		_, err = createIssueDependencyComment(sess, user, dep, issue, true)

		if err != nil {
			return err, exists
		}
	}
	return nil, exists
}

// RemoveIssueDependency removes a dependency from an issue
func RemoveIssueDependency(user *User, issue *Issue, dep *Issue, depType int64) (err error) {
	sess := x.NewSession()

	// TODO: Same as above
	err = x.Sync(new(IssueDependency))
	if err != nil {
		return err
	}

	// Check if it exists
	exists, err := issueDepExists(x, issue.ID, dep.ID)
	if err != nil {
		return err
	}

	// If it exists, remove it, otherwise show an error message
	if exists {

		if depType == 1 {
			_, err := x.Delete(&IssueDependency{IssueID: issue.ID, DependencyID: dep.ID})
			if err != nil {
				return err
			}
		}

		if depType == 2 {
			_, err := x.Delete(&IssueDependency{IssueID: dep.ID, DependencyID: issue.ID})
			if err != nil {
				return err
			}
		}

		// Add comment referencing the removed dependency
		_, err = createIssueDependencyComment(sess, user, issue, dep, false)

		if err != nil {
			return err
		}

		// Create a new comment for the dependent issue
		_, err = createIssueDependencyComment(sess, user, dep, issue, false)

		if err != nil {
			return err
		}
	}
	return nil
}

// Check if the dependency already exists
func issueDepExists(e Engine, issueID int64, depID int64) (exists bool, err error) {
	var Dependencies = IssueDependency{IssueID: issueID, DependencyID: depID}

	exists, err = e.Get(&Dependencies)

	// Check for dependencies the other way around
	// Otherwise two issues could block each other which would result in none of them could be closed.
	if !exists {
		Dependencies.IssueID = depID
		Dependencies.DependencyID = issueID
		exists, err = e.Get(&Dependencies)
	}

	return
}

// IssueNoDependenciesLeft checks if issue can be closed
func IssueNoDependenciesLeft(issue *Issue) bool {

	var issueDeps []IssueDependency
	err := x.Where("issue_id = ?", issue.ID).Find(&issueDeps)

	for _, issueDep := range issueDeps {
		issueDetails, _ := getIssueByID(x, issueDep.DependencyID)
		if !issueDetails.IsClosed {
			return false
		}
	}

	if err != nil {
		return false
	}

	return true
}
