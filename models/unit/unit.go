// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	"errors"
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
	TypePackages                    // 9 Packages
	TypeActions                     // 10 Actions
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
	case TypePackages:
		return "TypePackages"
	case TypeActions:
		return "TypeActions"
	}
	return fmt.Sprintf("Unknown Type %d", u)
}

func (u Type) LogString() string {
	return fmt.Sprintf("<UnitType:%d:%s>", u, u.String())
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
		TypePackages,
		TypeActions,
	}

	// DefaultRepoUnits contains the default unit types
	DefaultRepoUnits = []Type{
		TypeCode,
		TypeIssues,
		TypePullRequests,
		TypeReleases,
		TypeWiki,
		TypeProjects,
		TypePackages,
	}

	// ForkRepoUnits contains the default unit types for forks
	DefaultForkRepoUnits = []Type{
		TypeCode,
		TypePullRequests,
	}

	// NotAllowedDefaultRepoUnits contains units that can't be default
	NotAllowedDefaultRepoUnits = []Type{
		TypeExternalWiki,
		TypeExternalTracker,
	}

	// DisabledRepoUnits contains the units that have been globally disabled
	DisabledRepoUnits = []Type{}
)

// Get valid set of default repository units from settings
func validateDefaultRepoUnits(defaultUnits, settingDefaultUnits []Type) []Type {
	units := defaultUnits

	// Use setting if not empty
	if len(settingDefaultUnits) > 0 {
		units = make([]Type, 0, len(settingDefaultUnits))
		for _, settingUnit := range settingDefaultUnits {
			if !settingUnit.CanBeDefault() {
				log.Warn("Not allowed as default unit: %s", settingUnit.String())
				continue
			}
			units = append(units, settingUnit)
		}
	}

	// Remove disabled units
	for _, disabledUnit := range DisabledRepoUnits {
		for i, unit := range units {
			if unit == disabledUnit {
				units = append(units[:i], units[i+1:]...)
			}
		}
	}

	return units
}

// LoadUnitConfig load units from settings
func LoadUnitConfig() error {
	var invalidKeys []string
	DisabledRepoUnits, invalidKeys = FindUnitTypes(setting.Repository.DisabledRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in disabled repo units: %s", strings.Join(invalidKeys, ", "))
	}

	setDefaultRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultRepoUnits = validateDefaultRepoUnits(DefaultRepoUnits, setDefaultRepoUnits)
	if len(DefaultRepoUnits) == 0 {
		return errors.New("no default repository units found")
	}
	setDefaultForkRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultForkRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default fork repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultForkRepoUnits = validateDefaultRepoUnits(DefaultForkRepoUnits, setDefaultForkRepoUnits)
	if len(DefaultForkRepoUnits) == 0 {
		return errors.New("no default fork repository units found")
	}
	return nil
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

	UnitPackages = Unit{
		TypePackages,
		"repo.packages",
		"/packages",
		"packages.desc",
		6,
		perm.AccessModeRead,
	}

	UnitActions = Unit{
		TypeActions,
		"repo.actions",
		"/actions",
		"actions.unit.desc",
		7,
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
		TypePackages:        UnitPackages,
		TypeActions:         UnitActions,
	}
)

// FindUnitTypes give the unit key names and return valid unique units and invalid keys
func FindUnitTypes(nameKeys ...string) (res []Type, invalidKeys []string) {
	m := map[Type]struct{}{}
	for _, key := range nameKeys {
		t := TypeFromKey(key)
		if t == TypeInvalid {
			invalidKeys = append(invalidKeys, key)
		} else if _, ok := m[t]; !ok {
			res = append(res, t)
			m[t] = struct{}{}
		}
	}
	return res, invalidKeys
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
	for t, mode := range unitsMap {
		// Don't allow `TypeExternal{Tracker,Wiki}` to influence this as they can only be set to READ perms.
		if t == TypeExternalTracker || t == TypeExternalWiki {
			continue
		}

		// get the minial permission great than AccessModeNone except all are AccessModeNone
		if mode > perm.AccessModeNone && (res == perm.AccessModeNone || mode < res) {
			res = mode
		}
	}
	return res
}
