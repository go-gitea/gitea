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
	DependencyID int64	`xorm:"UNIQUE(watch) NOT NULL"`
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
func CreateOrUpdateIssueDependency(userID, issueID int64, dep int64) error {
	id, exists, err := getIssueWatch(x, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		id = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := x.Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := x.Id(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
			return err
		}
	}
	return nil
}

//
func getIssueDep(e Engine, issueID int64) (Dependencies []*IssueDependency, err error) {
	id = new(IssueDependency)
	err = e.
	Where("issue_id = ?", issueID).
		Find(&Dependencies)
	return
}
