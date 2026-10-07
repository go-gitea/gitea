// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
)

// GetProjectLabels returns the labels that can be applied to the project's items
func GetProjectLabels(ctx context.Context, project *project_model.Project) ([]*issues_model.Label, error) {
	if project.Type != project_model.TypeRepository {
		return issues_model.GetLabelsByOrgID(ctx, project.OwnerID, "", db.ListOptionsAll)
	}
	labels, err := issues_model.GetLabelsByRepoID(ctx, project.RepoID, "", db.ListOptionsAll)
	if err != nil {
		return nil, err
	}
	if err := project.LoadRepo(ctx); err != nil {
		return nil, err
	}
	orgLabels, err := issues_model.GetLabelsByOrgID(ctx, project.Repo.OwnerID, "", db.ListOptionsAll)
	if err != nil {
		return nil, err
	}
	return append(labels, orgLabels...), nil
}
