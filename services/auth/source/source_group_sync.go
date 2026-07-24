// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	org_service "gitea.dev/services/org"
)

type syncType int

const (
	syncAdd syncType = iota
	syncRemove
)

// SyncGroupsToTeams maps authentication source groups to organization and team memberships
func SyncGroupsToTeams(ctx context.Context, user *user_model.User, sourceUserGroups container.Set[string], sourceGroupTeamMapping map[string]map[string][]string, performRemoval bool) error {
	orgCache := make(map[string]*organization.Organization)
	teamCache := make(map[string]*organization.Team)
	return SyncGroupsToTeamsCached(ctx, user, sourceUserGroups, sourceGroupTeamMapping, performRemoval, orgCache, teamCache)
}

// SyncGroupsToTeamsCached maps authentication source groups to organization and team memberships
func SyncGroupsToTeamsCached(ctx context.Context, user *user_model.User, sourceUserGroups container.Set[string], sourceGroupTeamMapping map[string]map[string][]string, performRemoval bool, orgCache map[string]*organization.Organization, teamCache map[string]*organization.Team) error {
	membershipsToAdd, membershipsToRemove := resolveMappedMemberships(sourceUserGroups, sourceGroupTeamMapping)

	if performRemoval {
		if err := syncGroupsToTeamsCached(ctx, user, membershipsToRemove, syncRemove, orgCache, teamCache); err != nil {
			return fmt.Errorf("could not sync[remove] user groups: %w", err)
		}
	}

	if err := syncGroupsToTeamsCached(ctx, user, membershipsToAdd, syncAdd, orgCache, teamCache); err != nil {
		return fmt.Errorf("could not sync[add] user groups: %w", err)
	}

	return nil
}

func resolveMappedMemberships(sourceUserGroups container.Set[string], groupOrgTeamsMapping map[string]map[string][]string) (membershipsToAdd, membershipsToRemove map[string][]string) {
	membershipsToAdd, membershipsToRemove = map[string][]string{}, map[string][]string{}
	for group, orgTeams := range groupOrgTeamsMapping {
		isUserInGroup := sourceUserGroups.Contains(group)
		if isUserInGroup {
			for org, teams := range orgTeams {
				for _, teamName := range teams {
					membershipsToAdd[org] = append(membershipsToAdd[org], strings.ToLower(teamName))
				}
			}
		} else {
			for org, teams := range orgTeams {
				for _, teamName := range teams {
					membershipsToRemove[org] = append(membershipsToRemove[org], strings.ToLower(teamName))
				}
			}
		}
	}

	// If another group grants the same team (to add), don't remove it
	for org, removeTeams := range membershipsToRemove {
		removeTeamSet := container.SetOf(removeTeams...)
		removedCount := removeTeamSet.RemoveFromSlice(membershipsToAdd[org])
		if removedCount > 0 {
			removeTeams = removeTeamSet.Values()
			membershipsToRemove[org] = removeTeams
			if len(removeTeams) == 0 {
				delete(membershipsToRemove, org)
			}
		}
	}

	return membershipsToAdd, membershipsToRemove
}

func syncGroupsToTeamsCached(ctx context.Context, user *user_model.User, orgTeamMap map[string][]string, action syncType, orgCache map[string]*organization.Organization, teamCache map[string]*organization.Team) error {
	for orgName, teamNames := range orgTeamMap {
		var err error
		org, ok := orgCache[orgName]
		if !ok {
			org, err = organization.GetOrgByName(ctx, orgName)
			if err != nil {
				if organization.IsErrOrgNotExist(err) {
					// organization must be created before group sync
					log.Warn("group sync: Could not find organisation %s: %v", orgName, err)
					continue
				}
				return err
			}
			orgCache[orgName] = org
		}
		for _, teamName := range teamNames {
			team, ok := teamCache[orgName+teamName]
			if !ok {
				team, err = org.GetTeam(ctx, teamName)
				if err != nil {
					if organization.IsErrTeamNotExist(err) {
						// team must be created before group sync
						log.Warn("group sync: Could not find team %s: %v", teamName, err)
						continue
					}
					return err
				}
				teamCache[orgName+teamName] = team
			}

			isMember, err := organization.IsTeamMember(ctx, org.ID, team.ID, user.ID)
			if err != nil {
				return err
			}

			if action == syncAdd && !isMember {
				if err := org_service.AddTeamMember(ctx, team, user); err != nil {
					log.Error("group sync: Could not add user to team: %v", err)
					return err
				}
			} else if action == syncRemove && isMember {
				if err := org_service.RemoveTeamMember(ctx, team, user); err != nil {
					if organization.IsErrLastOrgOwner(err) {
						log.Warn("group sync: Skipping removal of last owner in org %s for user %s: %v", org.Name, user.Name, err)
						continue
					}
					log.Error("group sync: Could not remove user from team: %v", err)
					return err
				}
			}
		}
	}
	return nil
}
