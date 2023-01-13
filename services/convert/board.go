// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIProjectBoard(board *project_model.Board) *api.ProjectBoard {
	ctx := db.DefaultContext

	if err := board.LoadProject(ctx); err != nil {
		return &api.ProjectBoard{}
	}
	if err := board.LoadBoardCreator(ctx); err != nil {
		return &api.ProjectBoard{}
	}

	apiProjectBoard := &api.ProjectBoard{
		ID:      board.ID,
		Title:   board.Title,
		Default: board.Default,
		Color:   board.Color,
		Sorting: board.Sorting,
		Created: board.CreatedUnix.AsTime(),
		Updated: board.UpdatedUnix.AsTime(),
	}

	apiProjectBoard.Project = &api.Project{
		ID:          board.Project.ID,
		Title:       board.Project.Title,
		Description: board.Project.Description,
	}

	apiProjectBoard.Creator = &api.User{
		ID:       board.Creator.ID,
		UserName: board.Creator.Name,
		FullName: board.Creator.FullName,
	}

	return apiProjectBoard
}

func ToAPIProjectBoardList(boards project_model.BoardList) ([]*api.ProjectBoard, error) {
	if err := boards.LoadAttributes(db.DefaultContext); err != nil {
		return nil, err
	}
	result := make([]*api.ProjectBoard, len(boards))
	for i := range boards {
		result[i] = ToAPIProjectBoard(boards[i])
	}
	return result, nil
}
