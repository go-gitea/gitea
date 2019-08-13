// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

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

// Value returns integer value for unit type
func (u UnitType) Value() int {
	return int(u)
}

func (u UnitType) String() string {
	switch u {
	case UnitTypeCode:
		return "UnitTypeCode"
	case UnitTypeIssues:
		return "UnitTypeIssues"
	case UnitTypePullRequests:
		return "UnitTypePullRequests"
	case UnitTypeReleases:
		return "UnitTypeReleases"
	case UnitTypeWiki:
		return "UnitTypeWiki"
	case UnitTypeExternalWiki:
		return "UnitTypeExternalWiki"
	case UnitTypeExternalTracker:
		return "UnitTypeExternalTracker"
	}
	return fmt.Sprintf("Unknown UnitType %d", u)
}

// ColorFormat provides a ColorFormatted version of this UnitType
func (u UnitType) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s",
		log.NewColoredIDValue(u),
		u)
}

var (
	// AllRepoUnitTypes contains all the unit types
	AllRepoUnitTypes = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeExternalWiki,
		UnitTypeExternalTracker,
	}

	// DefaultRepoUnits contains the default unit types
	DefaultRepoUnits = []UnitType{
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

// IsLessThan compares order of two units
func (u Unit) IsLessThan(unit Unit) bool {
	if (u.Type == UnitTypeExternalTracker || u.Type == UnitTypeExternalWiki) && unit.Type != UnitTypeExternalTracker && unit.Type != UnitTypeExternalWiki {
		return false
	}
	return u.Idx < unit.Idx
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

// FindUnitTypes give the unit key name and return unit
func FindUnitTypes(nameKeys ...string) (res []UnitType) {
	for _, key := range nameKeys {
		for t, u := range Units {
			if strings.EqualFold(key, u.NameKey) {
				res = append(res, t)
				break
			}
		}
	}
	return
}
