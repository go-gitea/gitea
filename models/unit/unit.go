// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unit

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Type is Unit's Type
type Type int

// Enumerate all the unit types
const (
	TypeInvalid         Type = iota // 0 invalid
	TypeCode                        // 1 code
	TypeIssues                      // 2 issues
	TypePullRequests                // 3 PRs
	TypeReleases                    // 4 Releases
	TypeWiki                        // 5 Wiki
	TypeExternalWiki                // 6 ExternalWiki
	TypeExternalTracker             // 7 ExternalTracker
	TypeProjects                    // 8 Kanban board
)

// Value returns integer value for unit type
func (u Type) Value() int {
	return int(u)
}

func (u Type) String() string {
	switch u {
	case TypeCode:
		return "TypeCode"
	case TypeIssues:
		return "TypeIssues"
	case TypePullRequests:
		return "TypePullRequests"
	case TypeReleases:
		return "TypeReleases"
	case TypeWiki:
		return "TypeWiki"
	case TypeExternalWiki:
		return "TypeExternalWiki"
	case TypeExternalTracker:
		return "TypeExternalTracker"
	case TypeProjects:
		return "TypeProjects"
	}
	return fmt.Sprintf("Unknown Type %d", u)
}

// ColorFormat provides a ColorFormatted version of this Type
func (u Type) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s",
		log.NewColoredIDValue(u),
		u)
}

var (
	// AllRepoUnitTypes contains all the unit types
	AllRepoUnitTypes = []Type{
		TypeCode,
		TypeIssues,
		TypePullRequests,
		TypeReleases,
		TypeWiki,
		TypeExternalWiki,
		TypeExternalTracker,
		TypeProjects,
	}

	// DefaultRepoUnits contains the default unit types
	DefaultRepoUnits = []Type{
		TypeCode,
		TypeIssues,
		TypePullRequests,
		TypeReleases,
		TypeWiki,
		TypeProjects,
	}

	// NotAllowedDefaultRepoUnits contains units that can't be default
	NotAllowedDefaultRepoUnits = []Type{
		TypeExternalWiki,
		TypeExternalTracker,
	}

	// MustRepoUnits contains the units could not be disabled currently
	MustRepoUnits = []Type{
		TypeCode,
		TypeReleases,
	}

	// DisabledRepoUnits contains the units that have been globally disabled
	DisabledRepoUnits = []Type{}
)

// LoadUnitConfig load units from settings
func LoadUnitConfig() {
	setDefaultRepoUnits := FindUnitTypes(setting.Repository.DefaultRepoUnits...)
	// Default repo units set if setting is not empty
	if len(setDefaultRepoUnits) > 0 {
		// MustRepoUnits required as default
		DefaultRepoUnits = make([]Type, len(MustRepoUnits))
		copy(DefaultRepoUnits, MustRepoUnits)
		for _, defaultU := range setDefaultRepoUnits {
			if !defaultU.CanBeDefault() {
				log.Warn("Not allowed as default unit: %s", defaultU.String())
				continue
			}
			// MustRepoUnits already added
			if defaultU.CanDisable() {
				DefaultRepoUnits = append(DefaultRepoUnits, defaultU)
			}
		}
	}

	DisabledRepoUnits = FindUnitTypes(setting.Repository.DisabledRepoUnits...)
	// Check that must units are not disabled
	for i, disabledU := range DisabledRepoUnits {
		if !disabledU.CanDisable() {
			log.Warn("Not allowed to global disable unit %s", disabledU.String())
			DisabledRepoUnits = append(DisabledRepoUnits[:i], DisabledRepoUnits[i+1:]...)
		}
	}
	// Remove disabled units from default units
	for _, disabledU := range DisabledRepoUnits {
		for i, defaultU := range DefaultRepoUnits {
			if defaultU == disabledU {
				DefaultRepoUnits = append(DefaultRepoUnits[:i], DefaultRepoUnits[i+1:]...)
			}
		}
	}
}

// UnitGlobalDisabled checks if unit type is global disabled
func (u Type) UnitGlobalDisabled() bool {
	for _, ud := range DisabledRepoUnits {
		if u == ud {
			return true
		}
	}
	return false
}

// CanDisable checks if this unit type can be disabled.
func (u *Type) CanDisable() bool {
	for _, mu := range MustRepoUnits {
		if *u == mu {
			return false
		}
	}
	return true
}

// CanBeDefault checks if the unit type can be a default repo unit
func (u *Type) CanBeDefault() bool {
	for _, nadU := range NotAllowedDefaultRepoUnits {
		if *u == nadU {
			return false
		}
	}
	return true
}

// Unit is a section of one repository
type Unit struct {
	Type          Type
	NameKey       string
	URI           string
	DescKey       string
	Idx           int
	MaxAccessMode perm.AccessMode // The max access mode of the unit. i.e. Read means this unit can only be read.
}

// CanDisable returns if this unit could be disabled.
func (u *Unit) CanDisable() bool {
	return u.Type.CanDisable()
}

// IsLessThan compares order of two units
func (u Unit) IsLessThan(unit Unit) bool {
	if (u.Type == TypeExternalTracker || u.Type == TypeExternalWiki) && unit.Type != TypeExternalTracker && unit.Type != TypeExternalWiki {
		return false
	}
	return u.Idx < unit.Idx
}

// MaxPerm returns the max perms of this unit
func (u Unit) MaxPerm() perm.AccessMode {
	if u.Type == TypeExternalTracker || u.Type == TypeExternalWiki {
		return perm.AccessModeRead
	}
	return perm.AccessModeAdmin
}

// Enumerate all the units
var (
	UnitCode = Unit{
		TypeCode,
		"repo.code",
		"/",
		"repo.code.desc",
		0,
		perm.AccessModeOwner,
	}

	UnitIssues = Unit{
		TypeIssues,
		"repo.issues",
		"/issues",
		"repo.issues.desc",
		1,
		perm.AccessModeOwner,
	}

	UnitExternalTracker = Unit{
		TypeExternalTracker,
		"repo.ext_issues",
		"/issues",
		"repo.ext_issues.desc",
		1,
		perm.AccessModeRead,
	}

	UnitPullRequests = Unit{
		TypePullRequests,
		"repo.pulls",
		"/pulls",
		"repo.pulls.desc",
		2,
		perm.AccessModeOwner,
	}

	UnitReleases = Unit{
		TypeReleases,
		"repo.releases",
		"/releases",
		"repo.releases.desc",
		3,
		perm.AccessModeOwner,
	}

	UnitWiki = Unit{
		TypeWiki,
		"repo.wiki",
		"/wiki",
		"repo.wiki.desc",
		4,
		perm.AccessModeOwner,
	}

	UnitExternalWiki = Unit{
		TypeExternalWiki,
		"repo.ext_wiki",
		"/wiki",
		"repo.ext_wiki.desc",
		4,
		perm.AccessModeRead,
	}

	UnitProjects = Unit{
		TypeProjects,
		"repo.projects",
		"/projects",
		"repo.projects.desc",
		5,
		perm.AccessModeOwner,
	}

	// Units contains all the units
	Units = map[Type]Unit{
		TypeCode:            UnitCode,
		TypeIssues:          UnitIssues,
		TypeExternalTracker: UnitExternalTracker,
		TypePullRequests:    UnitPullRequests,
		TypeReleases:        UnitReleases,
		TypeWiki:            UnitWiki,
		TypeExternalWiki:    UnitExternalWiki,
		TypeProjects:        UnitProjects,
	}
)

// FindUnitTypes give the unit key names and return unit
func FindUnitTypes(nameKeys ...string) (res []Type) {
	for _, key := range nameKeys {
		var found bool
		for t, u := range Units {
			if strings.EqualFold(key, u.NameKey) {
				res = append(res, t)
				found = true
				break
			}
		}
		if !found {
			res = append(res, TypeInvalid)
		}
	}
	return
}

// TypeFromKey give the unit key name and return unit
func TypeFromKey(nameKey string) Type {
	for t, u := range Units {
		if strings.EqualFold(nameKey, u.NameKey) {
			return t
		}
	}
	return TypeInvalid
}

// AllUnitKeyNames returns all unit key names
func AllUnitKeyNames() []string {
	res := make([]string, 0, len(Units))
	for _, u := range Units {
		res = append(res, u.NameKey)
	}
	return res
}

// MinUnitAccessMode returns the minial permission of the permission map
func MinUnitAccessMode(unitsMap map[Type]perm.AccessMode) perm.AccessMode {
	res := perm.AccessModeNone
	for _, mode := range unitsMap {
		// get the minial permission great than AccessModeNone except all are AccessModeNone
		if mode > perm.AccessModeNone && (res == perm.AccessModeNone || mode < res) {
			res = mode
		}
	}
	return res
}
