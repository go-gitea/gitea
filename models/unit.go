// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// UnitType is Unit's Type
type UnitType int

// Enumerate all the unit types
const (
	UnitCode     UnitType = iota // 0 code
	UnitIssues                   // 1 issues
	UnitPRs                      // 2 PRs
	UnitCommits                  // 3 Commits
	UnitReleases                 // 4 Releases
	UnitWiki                     // 5 Wiki
	UnitSettings                 // 6 Settings
)

// Unit is a tab page of one repository
type Unit struct {
	UnitType
	NameKey string
	URI     string
	DescKey string
	Idx     int
}

var (
	// Units contains all the units
	Units = map[UnitType]Unit{
		UnitCode: {
			UnitCode,
			"repo.code",
			"/",
			"repo.code_desc",
			0,
		},
		UnitIssues: {
			UnitIssues,
			"repo.issues",
			"/issues",
			"repo.issues_desc",
			1,
		},
		UnitPRs: {
			UnitPRs,
			"repo.pulls",
			"/pulls",
			"repo.pulls_desc",
			2,
		},
		UnitCommits: {
			UnitCommits,
			"repo.commits",
			"/commits/master",
			"repo.commits_desc",
			3,
		},
		UnitReleases: {
			UnitReleases,
			"repo.releases",
			"/releases",
			"repo.releases_desc",
			4,
		},
		UnitWiki: {
			UnitWiki,
			"repo.wiki",
			"/wiki",
			"repo.wiki_desc",
			5,
		},
		UnitSettings: {
			UnitSettings,
			"repo.settings",
			"/settings",
			"repo.settings_desc",
			6,
		},
	}

	// UnitTypes contains all the unit types
	UnitTypes = []UnitType{
		UnitCode,
		UnitIssues,
		UnitPRs,
		UnitCommits,
		UnitReleases,
		UnitWiki,
		UnitSettings,
	}
)
