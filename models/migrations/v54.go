// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addPullRequestOptions(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64                  `xorm:"INDEX(s)"`
		Type        int                    `xorm:"INDEX(s)"`
		Config      map[string]interface{} `xorm:"JSON"`
		CreatedUnix timeutil.TimeStamp     `xorm:"INDEX CREATED"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	//Updating existing issue units
	units := make([]*RepoUnit, 0, 100)
	if err := sess.Where("`type` = ?", V16UnitTypePRs).Find(&units); err != nil {
		return fmt.Errorf("Query repo units: %v", err)
	}
	for _, unit := range units {
		if unit.Config == nil {
			unit.Config = make(map[string]interface{})
		}
		if _, ok := unit.Config["IgnoreWhitespaceConflicts"]; !ok {
			unit.Config["IgnoreWhitespaceConflicts"] = false
		}
		if _, ok := unit.Config["AllowMerge"]; !ok {
			unit.Config["AllowMerge"] = true
		}
		if _, ok := unit.Config["AllowRebase"]; !ok {
			unit.Config["AllowRebase"] = true
		}
		if _, ok := unit.Config["AllowSquash"]; !ok {
			unit.Config["AllowSquash"] = true
		}
		if _, err := sess.ID(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}
	return sess.Commit()
}
