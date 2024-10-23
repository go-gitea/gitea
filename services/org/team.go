// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"

	"code.gitea.io/gitea/models"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
)

func UpdateTeam(ctx context.Context, team *org_model.Team, teamName, description string, isAdmin, includesAllRepositories, canCreateOrgRepo bool, unitPerms map[unit_model.Type]perm.AccessMode) error {
	var changedCols []string

	newAccessMode := perm.AccessModeRead
	if isAdmin {
		newAccessMode = perm.AccessModeAdmin
	} else {
		// if newAccessMode is less than admin accessmode, then it should be general accessmode,
		// so we should calculate the minial accessmode from units accessmodes.
		newAccessMode = unit_model.MinUnitAccessMode(unitPerms)
	}

	if !team.IsOwnerTeam() {
		team.Name = teamName
		if team.AccessMode != newAccessMode {
			team.AccessMode = newAccessMode
			changedCols = append(changedCols, "authorize")
		}

		if team.IncludesAllRepositories != includesAllRepositories {
			team.IncludesAllRepositories = includesAllRepositories
			changedCols = append(changedCols, "includes_all_repositories")
		}
		units := make([]*org_model.TeamUnit, 0, len(unitPerms))
		for tp, perm := range unitPerms {
			units = append(units, &org_model.TeamUnit{
				OrgID:      team.OrgID,
				TeamID:     team.ID,
				Type:       tp,
				AccessMode: perm,
			})
		}
		team.Units = units
		changedCols = append(changedCols, "units")
		if team.CanCreateOrgRepo != canCreateOrgRepo {
			team.CanCreateOrgRepo = canCreateOrgRepo
			changedCols = append(changedCols, "can_create_org_repo")
		}
	} else {
		team.CanCreateOrgRepo = true
		team.IncludesAllRepositories = true
		changedCols = append(changedCols, "can_create_org_repo", "includes_all_repositories")
	}

	if team.Description != description {
		changedCols = append(changedCols, "description")
		team.Description = description
	}

	return models.UpdateTeam(ctx, team, changedCols...)
}
