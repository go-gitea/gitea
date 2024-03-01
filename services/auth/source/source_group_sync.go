// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
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

func resolveMappedMemberships(sourceUserGroups container.Set[string], sourceGroupTeamMapping map[string]map[string][]string) (map[string][]string, map[string][]string) {
	membershipsToAdd := map[string][]string{}
	membershipsToRemove := map[string][]string{}
	for group, memberships := range sourceGroupTeamMapping {
		isUserInGroup := sourceUserGroups.Contains(group)
		if isUserInGroup {
			for org, teams := range memberships {
				membershipsToAdd[org] = append(membershipsToAdd[org], teams...)
			}
		} else {
			for org, teams := range memberships {
				membershipsToRemove[org] = append(membershipsToRemove[org], teams...)
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
				if err := models.AddTeamMember(ctx, team, user.ID); err != nil {
					log.Error("group sync: Could not add user to team: %v", err)
					return err
				}
			} else if action == syncRemove && isMember {
				if err := models.RemoveTeamMember(ctx, team, user.ID); err != nil {
					log.Error("group sync: Could not remove user from team: %v", err)
					return err
				}
			}
		}
	}
	return nil
}
