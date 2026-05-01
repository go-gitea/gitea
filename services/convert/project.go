// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	project_model "code.gitea.io/gitea/models/project"
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
