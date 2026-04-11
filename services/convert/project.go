// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIProject converts a project to its API representation for embedding in issue/PR responses.
func ToAPIProject(p *project_model.Project, columnID int64, columnTitle string) *api.ProjectMeta {
	if p == nil {
		return nil
	}

	state := api.StateOpen
	if p.IsClosed {
		state = api.StateClosed
	}

	result := &api.ProjectMeta{
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		State:       state,
		Created:     p.CreatedUnix.AsTime(),
		Updated:     p.UpdatedUnix.AsTimePtr(),
	}
	if p.IsClosed {
		result.Closed = p.ClosedDateUnix.AsTimePtr()
	}

	if columnID > 0 {
		result.ColumnID = columnID
		result.Column = columnTitle
	}

	return result
}

// canDoerSeeProject checks if the doer has permission to see a project.
// For repo-level projects, repo read access is sufficient (already checked by API handler).
// For org/user-level projects, checks org visibility and projects unit permission.
// Results are cached per owner ID in the provided EphemeralCache.
func canDoerSeeProject(ctx context.Context, permCache *cache.EphemeralCache, doer *user_model.User, p *project_model.Project) bool {
	if p.RepoID > 0 {
		return true
	}
	if p.OwnerID == 0 {
		return false
	}
	if doer != nil && doer.IsAdmin {
		return true
	}
	accessMode, _ := cache.GetWithEphemeralCache(ctx, permCache, "org-project-perm", p.OwnerID, func(ctx context.Context, ownerID int64) (perm.AccessMode, error) {
		owner, err := user_model.GetUserByID(ctx, ownerID)
		if err != nil {
			return perm.AccessModeNone, err
		}
		if !organization.HasOrgOrUserVisible(ctx, owner, doer) {
			return perm.AccessModeNone, nil
		}
		return organization.OrgFromUser(owner).UnitPermission(ctx, doer, unit.TypeProjects), nil
	})
	return accessMode >= perm.AccessModeRead
}
