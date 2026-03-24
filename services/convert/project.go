// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIProject converts a project to its API representation for embedding in issue/PR responses.
func ToAPIProject(issue *issues_model.Issue, p *project_model.Project) *api.ProjectMeta {
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

	if issue.ProjectBoardID > 0 {
		result.ColumnID = issue.ProjectBoardID
		result.Column = issue.ProjectBoardTitle
	}

	return result
}
