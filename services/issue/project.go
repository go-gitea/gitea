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

func IssueAssignOrRemoveProject(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, projectID int64, position int) error {
	if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, projectID, 0); err != nil {
		return err
	}

	var newProject *project_model.Project
	var err error
	if projectID > 0 {
		newProject, err = project_model.GetProjectByID(ctx, projectID)
		if err != nil {
			return err
		}
	}

	notify.IssueChangeProjects(ctx, doer, issue, newProject)
	return nil
}
