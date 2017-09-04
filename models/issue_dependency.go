// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
)

// IssueDependency is connection request for receiving issue notification.
type IssueDependency struct {
	ID          int64     `xorm:"pk autoincr"`
	UserID      int64     `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID     int64     `xorm:"UNIQUE(watch) NOT NULL"`
	DependencyID int64    `xorm:"UNIQUE(watch) NOT NULL"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"NOT NULL"`
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"NOT NULL"`
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
func CreateIssueDependency(userID, issueID int64, depID int64) (err error, exists bool, depExists bool) {
	sess := x.NewSession()

	// TODO: Move this to the appropriate place
	err = x.Sync(new(IssueDependency))
	if err != nil {
		return err, exists, false
	}

	// Check if it aleready exists
	exists, err = issueDepExists(x, issueID, depID)
	if err != nil {
		return err, exists, false
	}

	// If it not exists, create it, otherwise show an error message
	if !exists {
		// Check if the other issue exists
		var issue = Issue{}
		issueExists, err := x.Id(depID).Get(&issue)

		if issueExists {
			newId := new(IssueDependency)
			newId.UserID = userID
			newId.IssueID = issueID
			newId.DependencyID = depID

			if _, err := x.Insert(newId); err != nil {
				return err, exists, false
			}

			user, err := getUserByID(x, userID)
			if err != nil {
				return err, exists, false
			}

			// Add comment referencing the new dependency

			repo, err := getRepositoryByID(x, issue.RepoID)

			if err != nil {
				return err, exists, false
			}


			_, err = createIssueDependencyComment(sess, user, repo, &issue, depID, true)

			if err != nil {
				return err, exists, false
			}

			// Create a new comment for the dependent issue
			depIssue, err := getIssueByID(x, issueID)
			if err != nil {
				return err, exists, false
			}

			repo, err = getRepositoryByID(x, depIssue.RepoID)
			if err != nil {
				return err, exists, false
			}

			_, err = createIssueDependencyComment(sess, user, repo, depIssue, issueID, true)

			if err != nil {
				return err, exists, false
			}
		}
		return err, exists, true
	}
	return nil, exists, false
}

// Removes a dependency from an issue
func RemoveIssueDependency(userID, issueID int64, depID int64, depType int64) (err error) {
	sess := x.NewSession()

	// TODO: Same as above
	err = x.Sync(new(IssueDependency))
	if err != nil {
		return err
	}

	// Check if it exists
	exists, err := issueDepExists(x, issueID, depID)
	if err != nil {
		return err
	}

	// If it exists, remove it, otherwise show an error message
	if exists {

		if depType == 1{
			_, err := x.Delete(&IssueDependency{IssueID: issueID, DependencyID: depID})
			if err != nil {
				return err
			}
		}

		if depType == 2{
			_, err := x.Delete(&IssueDependency{IssueID: depID, DependencyID: issueID})
			if err != nil {
				return err
			}
		}

		// Add comment referencing the removed dependency
		issue, err := getIssueByID(x, depID)

		if err != nil {
			return err
		}

		user, _ := getUserByID(x, userID)

		repo, _ := getRepositoryByID(x, issue.RepoID)

		_, err = createIssueDependencyComment(sess, user, repo, issue, depID, false)

		if err != nil {
			return err
		}

		// Create a new comment for the dependent issue
		depIssue, err := getIssueByID(x, issueID)

		if err != nil {
			return err
		}

		repo, _ = getRepositoryByID(x, depIssue.RepoID)

		_, err = createIssueDependencyComment(sess, user, repo, depIssue, issueID, false)

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

// check if issue can be closed
func IssueNoDependenciesLeft(issueID int64) bool{

	var issueDeps []IssueDependency
	err := x.Where("issue_id = ?", issueID).Find(&issueDeps)

	for _, issueDep := range issueDeps{
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
