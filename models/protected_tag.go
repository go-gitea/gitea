// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/modules/base"
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
func (pt *ProtectedTag) IsUserAllowed(userID int64) (bool, error) {
	if base.Int64sContains(pt.WhitelistUserIDs, userID) {
		return true, nil
	}

	if len(pt.WhitelistTeamIDs) == 0 {
		return false, nil
	}

	in, err := IsUserInTeams(userID, pt.WhitelistTeamIDs)
	if err != nil {
		return false, err
	}
	return in, nil
}

// GetProtectedTags gets all protected tags
func (repo *Repository) GetProtectedTags() ([]*ProtectedTag, error) {
	tags := make([]*ProtectedTag, 0)
	return tags, x.Find(&tags, &ProtectedTag{RepoID: repo.ID})
}

// IsUserAllowedToControlTag checks if a user can control the specific tag.
// It returns true if the tag name is not protected or the user is allowed to control it.
func IsUserAllowedToControlTag(tags []*ProtectedTag, tagName string, userID int64) (bool, error) {
	isAllowed := true
	for _, tag := range tags {
		err := tag.EnsureCompiledPattern()
		if err != nil {
			return false, err
		}

		if !tag.NameGlob.Match(tagName) {
			continue
		}

		isAllowed, err = tag.IsUserAllowed(userID)
		if err != nil {
			return false, err
		}
		if isAllowed {
			break
		}
	}

	return isAllowed, nil
}
