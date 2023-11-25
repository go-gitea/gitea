// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIProjectBoard(ctx context.Context, board *project_model.Board) *api.ProjectBoard {
	apiProjectBoard := api.ProjectBoard{
		ID:      board.ID,
		Title:   board.Title,
		Color:   board.Color,
		Default: board.Default,
		Sorting: board.Sorting,
	}

	return &apiProjectBoard
}

func ToApiProjectBoardList(
	ctx context.Context,
	boards []*project_model.Board,
) ([]*api.ProjectBoard, error) {
	result := make([]*api.ProjectBoard, len(boards))
	for i := range boards {
		result[i] = ToAPIProjectBoard(ctx, boards[i])
	}
	return result, nil
}

func ToAPIProject(ctx context.Context, project *project_model.Project) *api.Project {

	apiProject := &api.Project{
		Title:       project.Title,
		Description: project.Description,
		BoardType:   uint8(project.BoardType),
		IsClosed:    project.IsClosed,
		Created:     project.CreatedUnix.AsTime(),
		Updated:     project.UpdatedUnix.AsTime(),
		Closed:      project.ClosedDateUnix.AsTime(),
	}

	// try to laod the repo
	project.LoadRepo(ctx)
	if project.Repo != nil {
		apiProject.Repo = &api.RepositoryMeta{
			ID:       project.RepoID,
			Name:     project.Repo.Name,
			Owner:    project.Repo.OwnerName,
			FullName: project.Repo.FullName(),
		}
	}

	project.LoadCreator(ctx)
	if project.Creator != nil {
		apiProject.Creator = &api.User{
			ID:       project.Creator.ID,
			UserName: project.Creator.Name,
			FullName: project.Creator.FullName,
		}
	}

	project.LoadOwner(ctx)
	if project.Owner != nil {
		apiProject.Owner = &api.User{
			ID:       project.Owner.ID,
			UserName: project.Owner.Name,
			FullName: project.Owner.FullName,
		}
	}

	return apiProject
}

func ToAPIProjectList(
	ctx context.Context,
	projects []*project_model.Project,
) ([]*api.Project, error) {
	result := make([]*api.Project, len(projects))
	for i := range projects {
		result[i] = ToAPIProject(ctx, projects[i])
	}
	return result, nil
}
