// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addPullRequestRebaseWithMerge(x *xorm.Engine) error {
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
		// Allow the new merge style if all other merge styles are allowed
		allowMergeRebase := true

		if allowMerge, ok := unit.Config["AllowMerge"]; ok {
			allowMergeRebase = allowMergeRebase && allowMerge.(bool)
		}

		if allowRebase, ok := unit.Config["AllowRebase"]; ok {
			allowMergeRebase = allowMergeRebase && allowRebase.(bool)
		}

		if allowSquash, ok := unit.Config["AllowSquash"]; ok {
			allowMergeRebase = allowMergeRebase && allowSquash.(bool)
		}

		if _, ok := unit.Config["AllowRebaseMerge"]; !ok {
			unit.Config["AllowRebaseMerge"] = allowMergeRebase
		}
		if _, err := sess.ID(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}
	return sess.Commit()
}
