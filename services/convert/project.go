// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

// ToProject converts a models.Project to api.Project
func ToProject(ctx context.Context, project *project_model.Project) *api.Project {
	return &api.Project{
		ID:             project.ID,
		Title:          project.Title,
		Description:    project.Description,
		TemplateType:   uint8(project.TemplateType),
		CardType:       uint8(project.CardType),
		OwnerID:        project.OwnerID,
		RepoID:         project.RepoID,
		CreatorID:      project.CreatorID,
		IsClosed:       project.IsClosed,
		Type:           uint8(project.Type),
		CreatedUnix:    int64(project.CreatedUnix),
		UpdatedUnix:    int64(project.UpdatedUnix),
		ClosedDateUnix: int64(project.ClosedDateUnix),
	}
}

// ToProjects converts a slice of models.Project to a slice of api.Project
func ToProjects(ctx context.Context, projects []*project_model.Project) []*api.Project {
	result := make([]*api.Project, len(projects))
	for i, project := range projects {
		result[i] = ToProject(ctx, project)
	}
	return result
}
