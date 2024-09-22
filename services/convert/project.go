// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIProject(ctx context.Context, project *project_model.Project) (*api.Project, error) {
	apiProject := &api.Project{
		Title:        project.Title,
		Description:  project.Description,
		TemplateType: uint8(project.TemplateType),
		IsClosed:     project.IsClosed,
		Created:      project.CreatedUnix.AsTime(),
		Updated:      project.UpdatedUnix.AsTime(),
		Closed:       project.ClosedDateUnix.AsTime(),
	}

	if err := project.LoadRepo(ctx); err != nil {
		return nil, err
	}
	if project.Repo != nil {
		apiProject.Repo = &api.RepositoryMeta{
			ID:       project.RepoID,
			Name:     project.Repo.Name,
			Owner:    project.Repo.OwnerName,
			FullName: project.Repo.FullName(),
		}
	}

	if err := project.LoadCreator(ctx); err != nil {
		return nil, err
	}
	if project.Creator != nil {
		apiProject.Creator = &api.User{
			ID:       project.Creator.ID,
			UserName: project.Creator.Name,
			FullName: project.Creator.FullName,
		}
	}

	if err := project.LoadOwner(ctx); err != nil {
		return nil, err
	}
	if project.Owner != nil {
		apiProject.Owner = &api.User{
			ID:       project.Owner.ID,
			UserName: project.Owner.Name,
			FullName: project.Owner.FullName,
		}
	}

	return apiProject, nil
}

func ToAPIProjectList(ctx context.Context, projects []*project_model.Project) ([]*api.Project, error) {
	result := make([]*api.Project, len(projects))
	var err error
	for i := range projects {
		result[i], err = ToAPIProject(ctx, projects[i])
		if err != nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
