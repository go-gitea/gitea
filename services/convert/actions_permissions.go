// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	api "gitea.dev/modules/structs"
)

func actionsTokenPermissionFields(p *api.ActionsTokenPermissions) map[unit.Type]*api.ActionsTokenAccessLevel {
	return map[unit.Type]*api.ActionsTokenAccessLevel{
		unit.TypeCode:         &p.Code,
		unit.TypeIssues:       &p.Issues,
		unit.TypePullRequests: &p.PullRequests,
		unit.TypePackages:     &p.Packages,
		unit.TypeActions:      &p.Actions,
		unit.TypeWiki:         &p.Wiki,
		unit.TypeReleases:     &p.Releases,
		unit.TypeProjects:     &p.Projects,
	}
}

// ToActionsTokenPermissions converts token permissions to their API format, nil stays nil
func ToActionsTokenPermissions(p *repo_model.ActionsTokenPermissions) *api.ActionsTokenPermissions {
	if p == nil {
		return nil
	}
	ret := &api.ActionsTokenPermissions{}
	for ut, field := range actionsTokenPermissionFields(ret) {
		*field = api.ActionsTokenAccessLevel(p.UnitAccessModes[ut].ToString())
	}
	return ret
}

// FromAPIActionsTokenPermissions converts API token permissions to the model, nil stays nil
func FromAPIActionsTokenPermissions(p *api.ActionsTokenPermissions) *repo_model.ActionsTokenPermissions {
	if p == nil {
		return nil
	}
	ret := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	for ut, field := range actionsTokenPermissionFields(p) {
		ret.UnitAccessModes[ut] = perm.ParseAccessMode(string(*field), perm.AccessModeRead, perm.AccessModeWrite)
	}
	return &ret
}
