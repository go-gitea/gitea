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

type UpdateTeamOptions struct {
	TeamName                string
	Description             string
	IsAdmin                 bool
	IncludesAllRepositories bool
	CanCreateOrgRepo        bool
	UnitPerms               map[unit_model.Type]perm.AccessMode
}

func UpdateTeam(ctx context.Context, team *org_model.Team, opts UpdateTeamOptions) error {
	var changedCols []string

	newAccessMode := perm.AccessModeRead
	if opts.IsAdmin {
		newAccessMode = perm.AccessModeAdmin
	} else {
		// if newAccessMode is less than admin accessmode, then it should be general accessmode,
		// so we should calculate the minial accessmode from units accessmodes.
		newAccessMode = unit_model.MinUnitAccessMode(opts.UnitPerms)
	}

	if !team.IsOwnerTeam() {
		team.Name = opts.TeamName
		if team.AccessMode != newAccessMode {
			team.AccessMode = newAccessMode
			changedCols = append(changedCols, "authorize")
		}

		if team.IncludesAllRepositories != opts.IncludesAllRepositories {
			team.IncludesAllRepositories = opts.IncludesAllRepositories
			changedCols = append(changedCols, "includes_all_repositories")
		}
		units := make([]*org_model.TeamUnit, 0, len(opts.UnitPerms))
		for tp, perm := range opts.UnitPerms {
			units = append(units, &org_model.TeamUnit{
				OrgID:      team.OrgID,
				TeamID:     team.ID,
				Type:       tp,
				AccessMode: perm,
			})
		}
		team.Units = units
		changedCols = append(changedCols, "units")
		if team.CanCreateOrgRepo != opts.CanCreateOrgRepo {
			team.CanCreateOrgRepo = opts.CanCreateOrgRepo
			changedCols = append(changedCols, "can_create_org_repo")
		}
	} else {
		team.CanCreateOrgRepo = true
		team.IncludesAllRepositories = true
		changedCols = append(changedCols, "can_create_org_repo", "includes_all_repositories")
	}

	if team.Description != opts.Description {
		changedCols = append(changedCols, "description")
		team.Description = opts.Description
	}

	return models.UpdateTeam(ctx, team, changedCols...)
}
