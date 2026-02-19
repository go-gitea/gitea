// Copyright 2025 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package application

import (
	"sort"
	"strings"
)

// that's similar with oauth2 scope but more fine-grained

type AppPermMap map[AppPermGroup]AppPermGroupMap

type AppPermGroup string

type AppPermGroupMap map[AppPermItem]AppPermLevel

type AppPermItem string

type AppPermLevel string

const (
	PermLevelNone  AppPermLevel = "none"
	PermLevelRead  AppPermLevel = "read"
	PermLevelWrite AppPermLevel = "write"
)

const (
	PermGroupRepository   AppPermGroup = "repository"
	PermGroupOrganization AppPermGroup = "organization"
	PermGroupAccount      AppPermGroup = "account"
	PermGroupSystem       AppPermGroup = "system"
)

func (g AppPermGroup) defPermItem(item string) AppPermItem {
	return AppPermItem(string(g) + "." + item)
}

var (
	// Repository permissions
	PermItemRepoActions        = PermGroupRepository.defPermItem("actions")
	PermItemRepoAdministration = PermGroupRepository.defPermItem("administration")
	PermItemRepoContents       = PermGroupRepository.defPermItem("contents")
	PermItemRepoIssues         = PermGroupRepository.defPermItem("issues")
	PermItemRepoPullRequests   = PermGroupRepository.defPermItem("pull_requests")
	PermItemRepoReleases       = PermGroupRepository.defPermItem("releases")
	PermItemRepoWebhooks       = PermGroupRepository.defPermItem("webhooks")
	PermItemRepoPackages       = PermGroupRepository.defPermItem("packages")
	PermItemRepoChecks         = PermGroupRepository.defPermItem("checks") // used for commit status also
	PermItemRepoSecrets        = PermGroupRepository.defPermItem("secrets")

	// Organization permissions
	PermItemOrgAdministration = PermGroupOrganization.defPermItem("administration")
	PermItemOrgMembers        = PermGroupOrganization.defPermItem("members")
	PermItemOrgProjects       = PermGroupOrganization.defPermItem("projects")
	PermItemOrgTeams          = PermGroupOrganization.defPermItem("teams")

	// Account permissions
	PermItemAccountEmails        = PermGroupAccount.defPermItem("emails")
	PermItemAccountFollowers     = PermGroupAccount.defPermItem("followers")
	PermItemAccountGpgKeys       = PermGroupAccount.defPermItem("gpg_keys")
	PermItemAccountSshKeys       = PermGroupAccount.defPermItem("ssh_keys")
	PermItemAccountSubscriptions = PermGroupAccount.defPermItem("subscriptions")

	// System permissions
	PermItemSystemAdministration = PermGroupSystem.defPermItem("administration")
	PermItemSystemStats          = PermGroupSystem.defPermItem("stats")
)

func (g AppPermGroup) NewEmptyAppPermGroupMap() AppPermGroupMap {
	switch g {
	case PermGroupRepository:
		return AppPermGroupMap{
			PermItemRepoActions:        PermLevelNone,
			PermItemRepoAdministration: PermLevelNone,
			PermItemRepoContents:       PermLevelNone,
			PermItemRepoIssues:         PermLevelNone,
			PermItemRepoPullRequests:   PermLevelNone,
			PermItemRepoReleases:       PermLevelNone,
			PermItemRepoWebhooks:       PermLevelNone,
			PermItemRepoPackages:       PermLevelNone,
			PermItemRepoChecks:         PermLevelNone,
			PermItemRepoSecrets:        PermLevelNone,
		}
	case PermGroupOrganization:
		return AppPermGroupMap{
			PermItemOrgAdministration: PermLevelNone,
			PermItemOrgMembers:        PermLevelNone,
			PermItemOrgProjects:       PermLevelNone,
			PermItemOrgTeams:          PermLevelNone,
		}
	case PermGroupAccount:
		return AppPermGroupMap{
			PermItemAccountEmails:        PermLevelNone,
			PermItemAccountFollowers:     PermLevelNone,
			PermItemAccountGpgKeys:       PermLevelNone,
			PermItemAccountSshKeys:       PermLevelNone,
			PermItemAccountSubscriptions: PermLevelNone,
		}
	case PermGroupSystem:
		return AppPermGroupMap{
			PermItemSystemAdministration: PermLevelNone,
			PermItemSystemStats:          PermLevelNone,
		}
	default:
		return AppPermGroupMap{}
	}
}

func NewEmptyAppPermMap() AppPermMap {
	return AppPermMap{
		PermGroupRepository:   PermGroupRepository.NewEmptyAppPermGroupMap(),
		PermGroupOrganization: PermGroupOrganization.NewEmptyAppPermGroupMap(),
		PermGroupAccount:      PermGroupAccount.NewEmptyAppPermGroupMap(),
		PermGroupSystem:       PermGroupSystem.NewEmptyAppPermGroupMap(),
	}
}

// perm format: <group>.<item>:<level>,...
type AppPermList string

func (l AppPermList) ToMap() AppPermMap {
	m := NewEmptyAppPermMap()
	// Split the permission list by comma

	l = AppPermList(strings.TrimSpace(string(l)))

	permPairs := strings.Split(string(l), ",")
	for _, pair := range permPairs {
		// Split each pair by colon
		parts := strings.SplitN(pair, ":", 3)
		if len(parts) != 2 {
			continue
		}
		groupItem := parts[0]
		level := parts[1]

		// Split the group and item
		groupItemParts := strings.SplitN(groupItem, ".", 3)
		if len(groupItemParts) != 2 {
			continue
		}
		group := AppPermGroup(groupItemParts[0])
		item := AppPermItem(groupItem)

		if _, ok := m[group]; !ok {
			continue
		}

		if _, ok := m[group][item]; !ok {
			continue
		}

		// Set the permission level
		m[group][item] = AppPermLevel(level)
	}

	return m
}

func (l AppPermList) ValidOrDefault() AppPermList {
	return l.ToMap().ToList()
}

func (m AppPermMap) ToList() AppPermList {
	var pairs []string

	for _, items := range m {
		for item, level := range items {
			if level == PermLevelNone {
				continue
			}

			pair := string(item) + ":" + string(level)
			pairs = append(pairs, pair)
		}
	}

	sort.Strings(pairs)
	list := AppPermList(strings.Join(pairs, ","))
	return list
}

func (g AppPermGroup) LocalTitle() string {
	return "gitea_apps.app_perm_group." + string(g)
}

func (i AppPermItem) LocalDesc() string {
	return "gitea_apps.app_perm_item." + string(i) + ".desc"
}

func (i AppPermItem) LocalTitle() string {
	return "gitea_apps.app_perm_item." + string(i) + ".title"
}

func (i AppPermItem) AllPermLevels() []AppPermLevel {
	return []AppPermLevel{
		PermLevelNone,
		PermLevelRead,
		PermLevelWrite,
	}
}

func (i AppPermItem) FullValue(level AppPermLevel) string {
	return string(i) + ":" + string(level)
}

func (l AppPermLevel) LocalTitle() string {
	return "gitea_apps.app_perm_level." + string(l)
}

type AppPermRequirement struct {
	Item  AppPermItem
	Level AppPermLevel
}
