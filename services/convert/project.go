// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIProject converts a Project to API format
func ToAPIProject(p *project_model.Project) *api.Project {
	apiProject := &api.Project{
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		OwnerID:     p.OwnerID,
		RepoID:      p.RepoID,
		CreatorID:   p.CreatorID,
		IsClosed:    p.IsClosed,
		Created:     p.CreatedUnix.AsTime(),
		Updated:     p.UpdatedUnix.AsTime(),
	}
	if p.IsClosed && p.ClosedDateUnix > 0 {
		apiProject.Closed = p.ClosedDateUnix.AsTimePtr()
	}
	return apiProject
}

// ToAPIProjectList converts a list of Projects to API format
func ToAPIProjectList(projects []*project_model.Project) []*api.Project {
	result := make([]*api.Project, len(projects))
	for i := range projects {
		result[i] = ToAPIProject(projects[i])
	}
	return result
}

// canDoerSeeProject checks if the doer has permission to see a project.
// For repo-level projects, repo read access is sufficient (already
// checked by the API handler). For org/user-level projects, checks
// org visibility and the doer's projects unit permission. Results
// are cached per owner ID in the provided EphemeralCache so a list
// response with the same project on many issues only checks once.
func canDoerSeeProject(ctx context.Context, permCache *cache.EphemeralCache, doer *user_model.User, p *project_model.Project) bool {
	if p == nil {
		return false
	}
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

// ToAPIProjectMeta projects a LoadedProject into the embed shape used
// in issue/PR API responses. Returns nil if the doer cannot see this
// project (visibility filter for org/user-level projects).
func ToAPIProjectMeta(ctx context.Context, permCache *cache.EphemeralCache, doer *user_model.User, lp *issues_model.LoadedProject) *api.ProjectMeta {
	if lp == nil || lp.Project == nil {
		return nil
	}
	if !canDoerSeeProject(ctx, permCache, doer, lp.Project) {
		return nil
	}
	return &api.ProjectMeta{
		ID:       lp.Project.ID,
		Title:    lp.Project.Title,
		ColumnID: lp.ColumnID,
		Column:   lp.ColumnTitle,
	}
}
