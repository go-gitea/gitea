// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/usergroup"
	usergroup_service "code.gitea.io/gitea/services/usergroup"
)

// AddTeamUserGroup assigns a user group to a team.
func AddTeamUserGroup(ctx context.Context, team *organization.Team, groupID int64) error {
	isAssigned, err := organization.IsUserGroupInTeam(ctx, team.ID, groupID)
	if err != nil || isAssigned {
		return err
	}

	if err := organization.AddUserGroupToTeam(ctx, team.ID, groupID, team.OrgID); err != nil {
		return err
	}

	// Ensure every member of the group (and its descendants) is recorded in org_user
	// so they are recognised as org members by IsOrganizationMember.
	if err := syncGroupMembersToOrg(ctx, team.OrgID, []int64{groupID}, true); err != nil {
		return fmt.Errorf("syncGroupMembersToOrg: %w", err)
	}

	return recalculateTeamAccessForUserGroups(ctx, team.ID)
}

// RemoveTeamUserGroup removes a user group from a team.
func RemoveTeamUserGroup(ctx context.Context, team *organization.Team, groupID int64) error {
	if err := organization.RemoveUserGroupFromTeam(ctx, team.ID, groupID); err != nil {
		return err
	}

	// Reevaluate org membership for former group members.
	if err := syncGroupMembersToOrg(ctx, team.OrgID, []int64{groupID}, false); err != nil {
		return fmt.Errorf("syncGroupMembersToOrg: %w", err)
	}

	return recalculateTeamAccessForUserGroups(ctx, team.ID)
}

// RecalculateUserGroupTeamAccesses recalculates access for teams assigned to the user group.
func RecalculateUserGroupTeamAccesses(ctx context.Context, groupID int64) error {
	// Expand to ancestors: if a team assigns "Engineering" and the group changed is
	// "Engineering/Backend", the "Engineering" team is also affected (its effective
	// member set changes). We therefore collect teams for the group itself AND all
	// ancestor groups.
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{groupID})
	if err != nil {
		return err
	}

	teamIDs, err := organization.GetTeamIDsByUserGroupIDs(ctx, 0, ancestorIDs)
	if err != nil {
		return err
	}
	if len(teamIDs) == 0 {
		return nil
	}
	return RecalculateTeamAccessesByTeamIDs(ctx, teamIDs)
}

// RecalculateTeamAccessesByTeamIDs recalculates access for specific teams.
func RecalculateTeamAccessesByTeamIDs(ctx context.Context, teamIDs []int64) error {
	for _, teamID := range teamIDs {
		if err := recalculateTeamAccessForUserGroups(ctx, teamID); err != nil {
			return err
		}
	}
	return nil
}

// SyncGroupMemberToOrgs synchronises org_user membership for a single user when
// they are added to or removed from a user group.
// It finds all orgs whose teams reference the group (or any ancestor of the group)
// and calls AddOrgUser / removeInvalidOrgUser accordingly.
func SyncGroupMemberToOrgs(ctx context.Context, groupID, userID int64, add bool) error {
	// Find all ancestor group IDs so we can look up teams that assign any of them.
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{groupID})
	if err != nil {
		return err
	}

	// Collect distinct org IDs from teams that use any ancestor group.
	orgIDs, err := getOrgIDsByAssignedUserGroupIDs(ctx, ancestorIDs)
	if err != nil {
		return err
	}

	user, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	for _, orgID := range orgIDs {
		if add {
			if err := organization.AddOrgUser(ctx, orgID, userID); err != nil {
				return err
			}
		} else {
			if err := removeInvalidOrgUser(ctx, orgID, user); err != nil {
				return err
			}
		}
	}
	return nil
}

// getOrgIDsByAssignedUserGroupIDs returns distinct org IDs of teams that
// directly reference any of the provided groups.
func getOrgIDsByAssignedUserGroupIDs(ctx context.Context, groupIDs []int64) ([]int64, error) {
	var orgIDs []int64
	if len(groupIDs) == 0 {
		return orgIDs, nil
	}

	if err := db.GetEngine(ctx).
		Table("team").
		Join("INNER", "team_user_group", "team_user_group.team_id = team.id").
		In("team_user_group.group_id", groupIDs).
		Distinct("team.org_id").
		Cols("team.org_id").
		Find(&orgIDs); err != nil {
		return nil, err
	}
	return orgIDs, nil
}

// getAncestorAssignedOrgIDs returns distinct org IDs of teams that reference the
// given group or any of its ancestors.
func getAncestorAssignedOrgIDs(ctx context.Context, groupID int64) ([]int64, error) {
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	return getOrgIDsByAssignedUserGroupIDs(ctx, ancestorIDs)
}

// DeleteUserGroupWithSync deletes a user group and cleans up org_user entries for its members.
// It must capture member and org data BEFORE deletion because the model removes those records atomically.
func DeleteUserGroupWithSync(ctx context.Context, groupID int64) error {
	// Capture effective members and affected orgs before the group is removed.
	memberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, []int64{groupID})
	if err != nil {
		return fmt.Errorf("GetEffectiveUserGroupMemberIDs: %w", err)
	}
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{groupID})
	if err != nil {
		return fmt.Errorf("ExpandUserGroupIDsToAncestors: %w", err)
	}
	orgIDs, err := getOrgIDsByAssignedUserGroupIDs(ctx, ancestorIDs)
	if err != nil {
		return fmt.Errorf("getOrgIDsByAssignedUserGroupIDs: %w", err)
	}
	teamIDs, err := organization.GetTeamIDsByUserGroupIDs(ctx, 0, ancestorIDs)
	if err != nil {
		return fmt.Errorf("GetTeamIDsByUserGroupIDs: %w", err)
	}

	if err := usergroup_service.DeleteUserGroup(ctx, groupID); err != nil {
		return err
	}

	if err := RecalculateTeamAccessesByTeamIDs(ctx, teamIDs); err != nil {
		return fmt.Errorf("RecalculateTeamAccessesByTeamIDs: %w", err)
	}

	// Reevaluate org membership for every former group member.
	for _, uid := range memberIDs {
		user, err := user_model.GetUserByID(ctx, uid)
		if err != nil {
			return err
		}
		for _, orgID := range orgIDs {
			if err := removeInvalidOrgUser(ctx, orgID, user); err != nil {
				return err
			}
		}
	}
	return nil
}

// SyncReplaceUserGroupMembers replaces the members of a user group and keeps
// org_user in sync: new members are added to affected orgs, removed members are
// re-evaluated (and removed from orgs if they have no other access path).
func SyncReplaceUserGroupMembers(ctx context.Context, groupID int64, newUserIDs []int64) error {
	// Snapshot old members before replacement.
	oldMemberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, []int64{groupID})
	if err != nil {
		return fmt.Errorf("GetEffectiveUserGroupMemberIDs (old): %w", err)
	}
	orgIDs, err := getAncestorAssignedOrgIDs(ctx, groupID)
	if err != nil {
		return fmt.Errorf("getAncestorAssignedOrgIDs: %w", err)
	}

	if err := usergroup.ReplaceUserGroupMembers(ctx, groupID, newUserIDs); err != nil {
		return err
	}

	if err := RecalculateUserGroupTeamAccesses(ctx, groupID); err != nil {
		return fmt.Errorf("RecalculateUserGroupTeamAccesses: %w", err)
	}

	if len(orgIDs) == 0 {
		return nil // group not assigned to any team, no org_user to update
	}

	newMemberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, []int64{groupID})
	if err != nil {
		return fmt.Errorf("GetEffectiveUserGroupMemberIDs (new): %w", err)
	}

	oldSet := make(map[int64]struct{}, len(oldMemberIDs))
	for _, id := range oldMemberIDs {
		oldSet[id] = struct{}{}
	}
	newSet := make(map[int64]struct{}, len(newMemberIDs))
	for _, id := range newMemberIDs {
		newSet[id] = struct{}{}
	}

	// Add new members to orgs.
	for _, uid := range newMemberIDs {
		if _, wasOld := oldSet[uid]; wasOld {
			continue // already an org member, no change needed
		}
		for _, orgID := range orgIDs {
			if err := organization.AddOrgUser(ctx, orgID, uid); err != nil {
				return err
			}
		}
	}

	// Re-evaluate removed members.
	for _, uid := range oldMemberIDs {
		if _, isNew := newSet[uid]; isNew {
			continue // still a member, keep their org access
		}
		user, err := user_model.GetUserByID(ctx, uid)
		if err != nil {
			return err
		}
		for _, orgID := range orgIDs {
			if err := removeInvalidOrgUser(ctx, orgID, user); err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdateUserGroupWithSync updates a user group and keeps access/org_user in
// sync when the group's parent changes.
func UpdateUserGroupWithSync(ctx context.Context, group *usergroup.UserGroup) error {
	oldGroup, err := usergroup.GetUserGroupByID(ctx, group.ID)
	if err != nil {
		return err
	}

	if oldGroup.ParentID == group.ParentID {
		return usergroup.UpdateUserGroup(ctx, group)
	}

	oldAncestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{group.ID})
	if err != nil {
		return fmt.Errorf("ExpandUserGroupIDsToAncestors (old): %w", err)
	}
	oldTeamIDs, err := organization.GetTeamIDsByUserGroupIDs(ctx, 0, oldAncestorIDs)
	if err != nil {
		return fmt.Errorf("GetTeamIDsByUserGroupIDs (old): %w", err)
	}
	oldOrgIDs, err := getOrgIDsByAssignedUserGroupIDs(ctx, oldAncestorIDs)
	if err != nil {
		return fmt.Errorf("getOrgIDsByAssignedUserGroupIDs (old): %w", err)
	}
	memberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, []int64{group.ID})
	if err != nil {
		return fmt.Errorf("GetEffectiveUserGroupMemberIDs: %w", err)
	}

	if err := usergroup.UpdateUserGroup(ctx, group); err != nil {
		return err
	}

	newAncestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, []int64{group.ID})
	if err != nil {
		return fmt.Errorf("ExpandUserGroupIDsToAncestors (new): %w", err)
	}
	newTeamIDs, err := organization.GetTeamIDsByUserGroupIDs(ctx, 0, newAncestorIDs)
	if err != nil {
		return fmt.Errorf("GetTeamIDsByUserGroupIDs (new): %w", err)
	}
	newOrgIDs, err := getOrgIDsByAssignedUserGroupIDs(ctx, newAncestorIDs)
	if err != nil {
		return fmt.Errorf("getOrgIDsByAssignedUserGroupIDs (new): %w", err)
	}

	affectedTeamIDs := append(append([]int64{}, oldTeamIDs...), newTeamIDs...)
	if err := RecalculateTeamAccessesByTeamIDs(ctx, affectedTeamIDs); err != nil {
		return fmt.Errorf("RecalculateTeamAccessesByTeamIDs: %w", err)
	}

	oldOrgSet := make(map[int64]struct{}, len(oldOrgIDs))
	for _, orgID := range oldOrgIDs {
		oldOrgSet[orgID] = struct{}{}
	}
	newOrgSet := make(map[int64]struct{}, len(newOrgIDs))
	for _, orgID := range newOrgIDs {
		newOrgSet[orgID] = struct{}{}
	}

	for _, orgID := range newOrgIDs {
		if _, existed := oldOrgSet[orgID]; existed {
			continue
		}
		for _, uid := range memberIDs {
			if err := organization.AddOrgUser(ctx, orgID, uid); err != nil {
				return err
			}
		}
	}

	for _, orgID := range oldOrgIDs {
		if _, stillPresent := newOrgSet[orgID]; stillPresent {
			continue
		}
		for _, uid := range memberIDs {
			user, err := user_model.GetUserByID(ctx, uid)
			if err != nil {
				return err
			}
			if err := removeInvalidOrgUser(ctx, orgID, user); err != nil {
				return err
			}
		}
	}

	return nil
}

func recalculateTeamAccessForUserGroups(ctx context.Context, teamID int64) error {
	repos, err := repo_model.GetTeamRepositories(ctx, &repo_model.SearchTeamRepoOptions{TeamID: teamID})
	if err != nil {
		return fmt.Errorf("GetTeamRepositories: %w", err)
	}
	for _, repo := range repos {
		if err := access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
			return fmt.Errorf("recalculateTeamAccesses: %w", err)
		}
	}
	return nil
}

// syncGroupMembersToOrg adds or re-evaluates org_user entries for all effective
// members of the given user groups (including descendants).
// If add=true it ensures org membership; if add=false it removes users who no
// longer have any team membership in the org.
func syncGroupMembersToOrg(ctx context.Context, orgID int64, groupIDs []int64, add bool) error {
	memberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, groupIDs)
	if err != nil {
		return err
	}
	for _, uid := range memberIDs {
		if add {
			if err := organization.AddOrgUser(ctx, orgID, uid); err != nil {
				return err
			}
		} else {
			user, err := user_model.GetUserByID(ctx, uid)
			if err != nil {
				return err
			}
			if err := removeInvalidOrgUser(ctx, orgID, user); err != nil {
				return err
			}
		}
	}
	return nil
}
