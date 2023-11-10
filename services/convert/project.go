// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIProject(project *project_model.Project) *api.Project {
	ctx := db.DefaultContext

	if err := project.LoadRepo(ctx); err != nil {
		return &api.Project{}
	}
	if err := project.LoadCreator(ctx); err != nil {
		return &api.Project{}
	}

	apiProject := &api.Project{
		Title:       project.Title,
		Description: project.Description,
		BoardType:   uint8(project.BoardType),
		IsClosed:    project.IsClosed,
		Created:     project.CreatedUnix.AsTime(),
		Updated:     project.UpdatedUnix.AsTime(),
		Closed:      project.ClosedDateUnix.AsTime(),
	}

	if project.Repo != nil {
		apiProject.Repo = &api.RepositoryMeta{
			ID:       project.Repo.ID,
			Name:     project.Repo.Name,
			Owner:    project.Repo.OwnerName,
			FullName: project.Repo.FullName(),
		}
	}

	apiProject.Creator = &api.User{
		ID:       project.Creator.ID,
		UserName: project.Creator.Name,
		FullName: project.Creator.FullName,
	}

	return apiProject
}

func ToAPIProjectList(projects project_model.List) ([]*api.Project, error) {
	if err := projects.LoadAttributes(db.DefaultContext); err != nil {
		return nil, err
	}
	result := make([]*api.Project, len(projects))
	for i := range projects {
		result[i] = ToAPIProject(projects[i])
	}
	return result, nil
}
