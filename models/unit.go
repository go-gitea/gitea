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
	UnitTypeReleases                            // 4 Releases
	UnitTypeWiki                                // 5 Wiki
	UnitTypeExternalWiki                        // 6 ExternalWiki
	UnitTypeExternalTracker                     // 7 ExternalTracker
)

var (
	// allRepUnitTypes contains all the unit types
	allRepUnitTypes = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeExternalWiki,
		UnitTypeExternalTracker,
	}

	// defaultRepoUnits contains the default unit types
	defaultRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
	}

	// MustRepoUnits contains the units could not be disabled currently
	MustRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeReleases,
	}
)

// Unit is a section of one repository
type Unit struct {
	Type    UnitType
	NameKey string
	URI     string
	DescKey string
	Idx     int
}

// CanDisable returns if this unit could be disabled.
func (u *Unit) CanDisable() bool {
	return true
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

	UnitReleases = Unit{
		UnitTypeReleases,
		"repo.releases",
		"/releases",
		"repo.releases.desc",
		3,
	}

	UnitWiki = Unit{
		UnitTypeWiki,
		"repo.wiki",
		"/wiki",
		"repo.wiki.desc",
		4,
	}

	UnitExternalWiki = Unit{
		UnitTypeExternalWiki,
		"repo.ext_wiki",
		"/wiki",
		"repo.ext_wiki.desc",
		4,
	}

	// Units contains all the units
	Units = map[UnitType]Unit{
		UnitTypeCode:            UnitCode,
		UnitTypeIssues:          UnitIssues,
		UnitTypeExternalTracker: UnitExternalTracker,
		UnitTypePullRequests:    UnitPullRequests,
		UnitTypeReleases:        UnitReleases,
		UnitTypeWiki:            UnitWiki,
		UnitTypeExternalWiki:    UnitExternalWiki,
	}
)
