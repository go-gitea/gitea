// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/notify"
)

func AssignOrRemoveProjects(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, newProjectIDs []int64) error {
	oldProjectColumnMap, err := issue.ProjectColumnMap(ctx)
	if err != nil {
		return err
	}
	if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, newProjectIDs); err != nil {
		return err
	}

	var newProjects []*project_model.Project
	if len(newProjectIDs) > 0 {
		for _, projectID := range newProjectIDs {
			newProject, err := project_model.GetProjectByID(ctx, projectID)
			if err != nil {
				return err
			}
			newProjects = append(newProjects, newProject)
		}
	}

	notify.IssueChangeProjects(ctx, doer, issue, oldProjectColumnMap, newProjects)
	return nil
}
