// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// UnitType is Unit's Type
type UnitType int

// Enumerate all the unit types
const (
	UnitTypeCode            UnitType = iota + 1 // 1 code
	UnitTypeIssues                              // 2 issues
	UnitTypePullRequests                        // 3 PRs
	UnitTypeCommits                             // 4 Commits
	UnitTypeReleases                            // 5 Releases
	UnitTypeWiki                                // 6 Wiki
	UnitTypeSettings                            // 7 Settings
	UnitTypeExternalWiki                        // 8 ExternalWiki
	UnitTypeExternalTracker                     // 9 ExternalTracker
)

var (
	// allRepUnitTypes contains all the unit types
	allRepUnitTypes = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeCommits,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeSettings,
		UnitTypeExternalWiki,
		UnitTypeExternalTracker,
	}

	// defaultRepoUnits contains the default unit types
	defaultRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeCommits,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeSettings,
	}

	// MustRepoUnits contains the units could be disabled currently
	MustRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeCommits,
		UnitTypeReleases,
		UnitTypeSettings,
	}
)

// Unit is a tab page of one repository
type Unit struct {
	Type    UnitType
	NameKey string
	URI     string
	DescKey string
	Idx     int
}

// CanDisable returns if this unit could be disabled.
func (u *Unit) CanDisable() bool {
	return u.Type != UnitTypeSettings
}

// Enumerate all the units
var (
	UnitCode = Unit{
		UnitTypeCode,
		"repo.code",
		"/",
		"repo.code.desc",
		0,
	}

	UnitIssues = Unit{
		UnitTypeIssues,
		"repo.issues",
		"/issues",
		"repo.issues.desc",
		1,
	}

	UnitExternalTracker = Unit{
		UnitTypeExternalTracker,
		"repo.ext_issues",
		"/issues",
		"repo.ext_issues.desc",
		1,
	}

	UnitPullRequests = Unit{
		UnitTypePullRequests,
		"repo.pulls",
		"/pulls",
		"repo.pulls.desc",
		2,
	}

	UnitCommits = Unit{
		UnitTypeCommits,
		"repo.commits",
		"/commits/master",
		"repo.commits.desc",
		3,
	}

	UnitReleases = Unit{
		UnitTypeReleases,
		"repo.releases",
		"/releases",
		"repo.releases.desc",
		4,
	}

	UnitWiki = Unit{
		UnitTypeWiki,
		"repo.wiki",
		"/wiki",
		"repo.wiki.desc",
		5,
	}

	UnitExternalWiki = Unit{
		UnitTypeExternalWiki,
		"repo.ext_wiki",
		"/wiki",
		"repo.ext_wiki.desc",
		5,
	}

	UnitSettings = Unit{
		UnitTypeSettings,
		"repo.settings",
		"/settings",
		"repo.settings.desc",
		6,
	}

	// Units contains all the units
	Units = map[UnitType]Unit{
		UnitTypeCode:            UnitCode,
		UnitTypeIssues:          UnitIssues,
		UnitTypeExternalTracker: UnitExternalTracker,
		UnitTypePullRequests:    UnitPullRequests,
		UnitTypeCommits:         UnitCommits,
		UnitTypeReleases:        UnitReleases,
		UnitTypeWiki:            UnitWiki,
		UnitTypeExternalWiki:    UnitExternalWiki,
		UnitTypeSettings:        UnitSettings,
	}
)
