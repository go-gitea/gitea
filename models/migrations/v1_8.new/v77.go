// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddPullRequestRebaseWithMerge(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64              `xorm:"INDEX(s)"`
		Type        int                `xorm:"INDEX(s)"`
		Config      map[string]any     `xorm:"JSON"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	const (
		v16UnitTypeCode            = iota + 1 // 1 code
		v16UnitTypeIssues                     // 2 issues
		v16UnitTypePRs                        // 3 PRs
		v16UnitTypeCommits                    // 4 Commits
		v16UnitTypeReleases                   // 5 Releases
		v16UnitTypeWiki                       // 6 Wiki
		v16UnitTypeSettings                   // 7 Settings
		v16UnitTypeExternalWiki               // 8 ExternalWiki
		v16UnitTypeExternalTracker            // 9 ExternalTracker
	)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	// Updating existing issue units
	units := make([]*RepoUnit, 0, 100)
	if err := sess.Where("`type` = ?", v16UnitTypePRs).Find(&units); err != nil {
		return fmt.Errorf("Query repo units: %w", err)
	}
	for _, unit := range units {
		if unit.Config == nil {
			unit.Config = make(map[string]any)
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
