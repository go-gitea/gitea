// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/gobwas/glob"
)

// ProtectedTag struct
type ProtectedTag struct {
	ID               int64     `xorm:"pk autoincr"`
	RepoID           int64     `xorm:"UNIQUE(s)"`
	NamePattern      string    `xorm:"UNIQUE(s)"`
	NameGlob         glob.Glob `xorm:"-"`
	WhitelistUserIDs []int64   `xorm:"JSON TEXT"`
	WhitelistTeamIDs []int64   `xorm:"JSON TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (pt *ProtectedTag) BeforeInsert() {
	pt.CreatedUnix = timeutil.TimeStampNow()
	pt.UpdatedUnix = timeutil.TimeStampNow()
}

// BeforeUpdate is invoked from XORM before updating this object.
func (pt *ProtectedTag) BeforeUpdate() {
	pt.UpdatedUnix = timeutil.TimeStampNow()
}

// InsertProtectedTag inserts a protected tag to database
func InsertProtectedTag(pt *ProtectedTag) error {
	_, err := x.Insert(pt)
	return err
}

// UpdateProtectedTag updates the protected tag
func UpdateProtectedTag(pt *ProtectedTag) error {
	_, err := x.ID(pt.ID).AllCols().Update(pt)
	return err
}

// DeleteProtectedTag deletes a protected tag by ID
func DeleteProtectedTag(pt *ProtectedTag) error {
	_, err := x.Delete(&ProtectedTag{ID: pt.ID})
	return err
}

// EnsureCompiledPattern returns if the branch is protected
func (pt *ProtectedTag) EnsureCompiledPattern() error {
	if pt.NameGlob != nil {
		return nil
	}

	expr := strings.TrimSpace(pt.NamePattern)

	var err error
	pt.NameGlob, err = glob.Compile(expr)
	return err
}

// IsUserAllowed returns true if the user is allowed to modify the tag
func (pt *ProtectedTag) IsUserAllowed(userID int64) bool {
	if base.Int64sContains(pt.WhitelistUserIDs, userID) {
		return true
	}

	if len(pt.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, pt.WhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// GetProtectedTags gets all protected tags
func (repo *Repository) GetProtectedTags() ([]*ProtectedTag, error) {
	tags := make([]*ProtectedTag, 0)
	return tags, x.Find(&tags, &ProtectedTag{RepoID: repo.ID})
}
