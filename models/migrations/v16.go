// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/markup"

	"xorm.io/xorm"
)

// Enumerate all the unit types
const (
	V16UnitTypeCode            = iota + 1 // 1 code
	V16UnitTypeIssues                     // 2 issues
	V16UnitTypePRs                        // 3 PRs
	V16UnitTypeCommits                    // 4 Commits
	V16UnitTypeReleases                   // 5 Releases
	V16UnitTypeWiki                       // 6 Wiki
	V16UnitTypeSettings                   // 7 Settings
	V16UnitTypeExternalWiki               // 8 ExternalWiki
	V16UnitTypeExternalTracker            // 9 ExternalTracker
)

func addUnitsToTables(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64 `xorm:"INDEX(s)"`
		Type        int   `xorm:"INDEX(s)"`
		Index       int
		Config      map[string]interface{} `xorm:"JSON"`
		CreatedUnix int64                  `xorm:"INDEX CREATED"`
		Created     time.Time              `xorm:"-"`
	}

	// Repo describes a repository
	type Repo struct {
		ID                                                                               int64
		EnableWiki, EnableExternalWiki, EnableIssues, EnableExternalTracker, EnablePulls bool
		ExternalWikiURL, ExternalTrackerURL, ExternalTrackerFormat, ExternalTrackerStyle string
	}

	var repos []Repo
	err := x.Table("repository").Select("*").Find(&repos)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var repoUnit RepoUnit
	if exist, err := sess.IsTableExist(&repoUnit); err != nil {
		return fmt.Errorf("IsExist RepoUnit: %v", err)
	} else if exist {
		return nil
	}

	if err := sess.CreateTable(&repoUnit); err != nil {
		return fmt.Errorf("CreateTable RepoUnit: %v", err)
	}

	if err := sess.CreateUniques(&repoUnit); err != nil {
		return fmt.Errorf("CreateUniques RepoUnit: %v", err)
	}

	if err := sess.CreateIndexes(&repoUnit); err != nil {
		return fmt.Errorf("CreateIndexes RepoUnit: %v", err)
	}

	for _, repo := range repos {
		for i := 1; i <= 9; i++ {
			if (i == V16UnitTypeWiki || i == V16UnitTypeExternalWiki) && !repo.EnableWiki {
				continue
			}
			if i == V16UnitTypeExternalWiki && !repo.EnableExternalWiki {
				continue
			}
			if i == V16UnitTypePRs && !repo.EnablePulls {
				continue
			}
			if (i == V16UnitTypeIssues || i == V16UnitTypeExternalTracker) && !repo.EnableIssues {
				continue
			}
			if i == V16UnitTypeExternalTracker && !repo.EnableExternalTracker {
				continue
			}

			var config = make(map[string]interface{})
			switch i {
			case V16UnitTypeExternalTracker:
				config["ExternalTrackerURL"] = repo.ExternalTrackerURL
				config["ExternalTrackerFormat"] = repo.ExternalTrackerFormat
				if len(repo.ExternalTrackerStyle) == 0 {
					repo.ExternalTrackerStyle = markup.IssueNameStyleNumeric
				}
				config["ExternalTrackerStyle"] = repo.ExternalTrackerStyle
			case V16UnitTypeExternalWiki:
				config["ExternalWikiURL"] = repo.ExternalWikiURL
			}

			if _, err = sess.Insert(&RepoUnit{
				RepoID: repo.ID,
				Type:   i,
				Index:  i,
				Config: config,
			}); err != nil {
				return fmt.Errorf("Insert repo unit: %v", err)
			}
		}
	}

	return sess.Commit()
}
