// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func removeDuplicateUnitTypes(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		RepoID int64
		Type   int
	}

	// Enumerate all the unit types
	const (
		UnitTypeCode            = iota + 1 // 1 code
		UnitTypeIssues                     // 2 issues
		UnitTypePullRequests               // 3 PRs
		UnitTypeReleases                   // 4 Releases
		UnitTypeWiki                       // 5 Wiki
		UnitTypeExternalWiki               // 6 ExternalWiki
		UnitTypeExternalTracker            // 7 ExternalTracker
	)

	var externalIssueRepoUnits []RepoUnit
	err := x.Where("type = ?", UnitTypeExternalTracker).Find(&externalIssueRepoUnits)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}

	var externalWikiRepoUnits []RepoUnit
	err = x.Where("type = ?", UnitTypeExternalWiki).Find(&externalWikiRepoUnits)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	for _, repoUnit := range externalIssueRepoUnits {
		if _, err = sess.Delete(&RepoUnit{
			RepoID: repoUnit.RepoID,
			Type:   UnitTypeIssues,
		}); err != nil {
			return fmt.Errorf("Delete repo unit: %v", err)
		}
	}

	for _, repoUnit := range externalWikiRepoUnits {
		if _, err = sess.Delete(&RepoUnit{
			RepoID: repoUnit.RepoID,
			Type:   UnitTypeWiki,
		}); err != nil {
			return fmt.Errorf("Delete repo unit: %v", err)
		}
	}

	return sess.Commit()
}
