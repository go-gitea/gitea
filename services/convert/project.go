// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

// ToProject converts a project_model.Project to api.Project
func ToProject(ctx context.Context, p *project_model.Project) *api.Project {
	if p == nil {
		return nil
	}

	project := &api.Project{
		ID:              p.ID,
		Title:           p.Title,
		Description:     p.Description,
		OwnerID:         p.OwnerID,
		RepoID:          p.RepoID,
		CreatorID:       p.CreatorID,
		IsClosed:        p.IsClosed,
		TemplateType:    int(p.TemplateType),
		CardType:        int(p.CardType),
		Type:            int(p.Type),
		NumOpenIssues:   p.NumOpenIssues,
		NumClosedIssues: p.NumClosedIssues,
		NumIssues:       p.NumIssues,
		Created:         p.CreatedUnix.AsTime(),
		Updated:         p.UpdatedUnix.AsTime(),
	}

	if p.ClosedDateUnix > 0 {
		t := p.ClosedDateUnix.AsTime()
		project.ClosedDate = &t
	}

	// Generate project URL
	if p.Type == project_model.TypeRepository && p.RepoID > 0 {
		if err := p.LoadRepo(ctx); err == nil && p.Repo != nil {
			project.URL = project_model.ProjectLinkForRepo(p.Repo, p.ID)
		}
	} else if p.OwnerID > 0 {
		if err := p.LoadOwner(ctx); err == nil && p.Owner != nil {
			project.URL = project_model.ProjectLinkForOrg(p.Owner, p.ID)
		}
	}

	return project
}

// ToProjectColumn converts a project_model.Column to api.ProjectColumn
func ToProjectColumn(ctx context.Context, column *project_model.Column) *api.ProjectColumn {
	if column == nil {
		return nil
	}

	return &api.ProjectColumn{
		ID:        column.ID,
		Title:     column.Title,
		Default:   column.Default,
		Sorting:   int(column.Sorting),
		Color:     column.Color,
		ProjectID: column.ProjectID,
		CreatorID: column.CreatorID,
		NumIssues: column.NumIssues,
		Created:   column.CreatedUnix.AsTime(),
		Updated:   column.UpdatedUnix.AsTime(),
	}
}

// ToProjectList converts a list of project_model.Project to a list of api.Project
func ToProjectList(ctx context.Context, projects []*project_model.Project) []*api.Project {
	result := make([]*api.Project, len(projects))
	for i, p := range projects {
		result[i] = ToProject(ctx, p)
	}
	return result
}

// ToProjectColumnList converts a list of project_model.Column to a list of api.ProjectColumn
func ToProjectColumnList(ctx context.Context, columns []*project_model.Column) []*api.ProjectColumn {
	result := make([]*api.ProjectColumn, len(columns))
	for i, column := range columns {
		result[i] = ToProjectColumn(ctx, column)
	}
	return result
}
