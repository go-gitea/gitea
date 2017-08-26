// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
	"fmt"
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

// CreateOrUpdateIssueDependency sets or updates a dependency for an issue
func CreateOrUpdateIssueDependency(userID, issueID int64, depID int64) error {
	err := x.Sync(new(IssueDependency))
	if err != nil {
		return err
	}

	exists, err := issueDepExists(x, issueID, depID)
	if err != nil {
		return err
	}

	if !exists {
		newId := new(IssueDependency)
		newId.UserID = userID
		newId.IssueID = issueID
		newId.DependencyID = depID

		if _, err := x.Insert(newId); err != nil {
			return err
		}
	} else {
		fmt.Println("Dependency exists")
		// TODO: Should display a message on issue page
	}
	return nil
}

//
func issueDepExists(e Engine, issueID int64, depID int64) (exists bool, err error) {
	var Dependencies = IssueDependency{IssueID: issueID, DependencyID: depID}

	//err = e.Where("issue_id = ?", issueID).Where("dependency_id = ?", depID).Find(&Dependencies)
	exists, err = e.Get(&Dependencies)
	return
}
