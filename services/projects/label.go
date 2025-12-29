// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
)

func GetProjectLabels(ctx context.Context, project *project_model.Project) ([]*issues_model.Label, error) {
	var labels []*issues_model.Label
	switch project.Type {
	case project_model.TypeOrganization, project_model.TypeIndividual:
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, project.OwnerID, "", db.ListOptionsAll)
		if err != nil {
			return nil, err
		}
		labels = append(labels, orgLabels...)
	case project_model.TypeRepository:
		// Get repository labels
		repoLabels, err := issues_model.GetLabelsByRepoID(ctx, project.RepoID, "", db.ListOptionsAll)
		if err != nil {
			return nil, err
		}
		labels = append(labels, repoLabels...)

		if err := project.LoadRepo(ctx); err != nil {
			return nil, err
		}

		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, project.Repo.OwnerID, "", db.ListOptionsAll)
		if err != nil {
			return nil, err
		}
		labels = append(labels, orgLabels...)
	}
	return labels, nil
}
