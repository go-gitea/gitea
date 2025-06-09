// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Type is Unit's Type
type Type int

// Enumerate all the unit types
const (
	TypeInvalid Type = iota // 0 invalid

	TypeCode            // 1 code
	TypeIssues          // 2 issues
	TypePullRequests    // 3 PRs
	TypeReleases        // 4 Releases
	TypeWiki            // 5 Wiki
	TypeExternalWiki    // 6 ExternalWiki
	TypeExternalTracker // 7 ExternalTracker
	TypeProjects        // 8 Projects
	TypePackages        // 9 Packages
	TypeActions         // 10 Actions

	// FIXME: TEAM-UNIT-PERMISSION: the team unit "admin" permission's design is not right, when a new unit is added in the future,
	// admin team won't inherit the correct admin permission for the new unit, need to have a complete fix before adding any new unit.
)

// Value returns integer value for unit type (used by template)
func (u Type) Value() int {
	return int(u)
}

func (u Type) LogString() string {
	unit, ok := Units[u]
	unitName := "unknown"
	if ok {
		unitName = unit.NameKey
	}
	return fmt.Sprintf("<UnitType:%d:%s>", u, unitName)
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
		TypeActions,
	}

	// ForkRepoUnits contains the default unit types for forks
	DefaultForkRepoUnits = []Type{
		TypeCode,
		TypePullRequests,
	}

	// DefaultMirrorRepoUnits contains the default unit types for mirrors
	DefaultMirrorRepoUnits = []Type{
		TypeCode,
		TypeIssues,
		TypeReleases,
		TypeWiki,
		TypeProjects,
		TypePackages,
	}

	// DefaultTemplateRepoUnits contains the default unit types for templates
	DefaultTemplateRepoUnits = []Type{
		TypeCode,
		TypeIssues,
		TypePullRequests,
		TypeReleases,
		TypeWiki,
		TypeProjects,
		TypePackages,
	}

	// NotAllowedDefaultRepoUnits contains units that can't be default
	NotAllowedDefaultRepoUnits = []Type{
		TypeExternalWiki,
		TypeExternalTracker,
	}

	disabledRepoUnitsAtomic atomic.Pointer[[]Type] // the units that have been globally disabled
)

// DisabledRepoUnitsGet returns the globally disabled units, it is a quick patch to fix data-race during testing.
// Because the queue worker might read when a test is mocking the value. FIXME: refactor to a clear solution later.
func DisabledRepoUnitsGet() []Type {
	v := disabledRepoUnitsAtomic.Load()
	if v == nil {
		return nil
	}
	return *v
}

func DisabledRepoUnitsSet(v []Type) {
	disabledRepoUnitsAtomic.Store(&v)
}

// Get valid set of default repository units from settings
func validateDefaultRepoUnits(defaultUnits, settingDefaultUnits []Type) []Type {
	units := defaultUnits

	// Use setting if not empty
	if len(settingDefaultUnits) > 0 {
		units = make([]Type, 0, len(settingDefaultUnits))
		for _, settingUnit := range settingDefaultUnits {
			if !settingUnit.CanBeDefault() {
				log.Warn("Not allowed as default unit: %s", settingUnit.LogString())
				continue
			}
			units = append(units, settingUnit)
		}
	}

	// Remove disabled units
	for _, disabledUnit := range DisabledRepoUnitsGet() {
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
	disabledRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DisabledRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in disabled repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DisabledRepoUnitsSet(disabledRepoUnits)

	setDefaultRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultRepoUnits = validateDefaultRepoUnits(DefaultRepoUnits, setDefaultRepoUnits)
	if len(DefaultRepoUnits) == 0 {
		return errors.New("no default repository units found")
	}
	// default fork repo units
	setDefaultForkRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultForkRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default fork repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultForkRepoUnits = validateDefaultRepoUnits(DefaultForkRepoUnits, setDefaultForkRepoUnits)
	if len(DefaultForkRepoUnits) == 0 {
		return errors.New("no default fork repository units found")
	}
	// default mirror repo units
	setDefaultMirrorRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultMirrorRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default mirror repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultMirrorRepoUnits = validateDefaultRepoUnits(DefaultMirrorRepoUnits, setDefaultMirrorRepoUnits)
	if len(DefaultMirrorRepoUnits) == 0 {
		return errors.New("no default mirror repository units found")
	}
	// default template repo units
	setDefaultTemplateRepoUnits, invalidKeys := FindUnitTypes(setting.Repository.DefaultTemplateRepoUnits...)
	if len(invalidKeys) > 0 {
		log.Warn("Invalid keys in default template repo units: %s", strings.Join(invalidKeys, ", "))
	}
	DefaultTemplateRepoUnits = validateDefaultRepoUnits(DefaultTemplateRepoUnits, setDefaultTemplateRepoUnits)
	if len(DefaultTemplateRepoUnits) == 0 {
		return errors.New("no default template repository units found")
	}
	return nil
}

// UnitGlobalDisabled checks if unit type is global disabled
func (u Type) UnitGlobalDisabled() bool {
	for _, ud := range DisabledRepoUnitsGet() {
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
	Priority      int
	MaxAccessMode perm.AccessMode // The max access mode of the unit. i.e. Read means this unit can only be read.
}

// IsLessThan compares order of two units
func (u Unit) IsLessThan(unit Unit) bool {
	return u.Priority < unit.Priority
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
		101,
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
		102,
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
	m := make(container.Set[Type])
	for _, key := range nameKeys {
		t := TypeFromKey(key)
		if t == TypeInvalid {
			invalidKeys = append(invalidKeys, key)
		} else if m.Add(t) {
			res = append(res, t)
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
