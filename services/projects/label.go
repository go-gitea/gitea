// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
)

func GetProjectLabels(ctx context.Context, project *project_model.Project) ([]*issues_model.Label, error) {
	if project.Type != project_model.TypeRepository {
		labels, err := issues_model.GetLabelsByOrgID(ctx, project.OwnerID, "", db.ListOptionsAll)
		issues_model.SortLabelsForDisplay(labels)
		return labels, err
	}
	labels, err := issues_model.GetLabelsByRepoID(ctx, project.RepoID, "", db.ListOptionsAll)
	if err != nil {
		return nil, err
	}
	issues_model.SortLabelsForDisplay(labels)
	if err := project.LoadRepo(ctx); err != nil {
		return nil, err
	}
	orgLabels, err := issues_model.GetLabelsByOrgID(ctx, project.Repo.OwnerID, "", db.ListOptionsAll)
	if err != nil {
		return nil, err
	}
	issues_model.SortLabelsForDisplay(orgLabels)
	return append(labels, orgLabels...), nil
}
