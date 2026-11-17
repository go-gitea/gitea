// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/optional"

	"xorm.io/builder"
)

// DeleteProject deletes the project and reindexes its issues to drop it from their search entries.
func DeleteProject(ctx context.Context, projectID int64) error {
	issueIDs, err := db.FindIDs(ctx, "project_issue", "issue_id", builder.Eq{"project_id": projectID})
	if err != nil {
		return err
	}
	if err := project_model.DeleteProjectByID(ctx, projectID); err != nil {
		return err
	}
	for _, issueID := range issueIDs {
		issue_indexer.UpdateIssueIndexer(ctx, issueID)
	}
	return nil
}

// UpdateProjectOptions represents updatable project fields. Fields with no value are left unchanged.
type UpdateProjectOptions struct {
	Title       optional.Option[string]
	Description optional.Option[string]
	CardType    optional.Option[project_model.CardType]
	IsClosed    optional.Option[bool]
}

// UpdateProject applies the provided options to the project atomically.
func UpdateProject(ctx context.Context, project *project_model.Project, opts UpdateProjectOptions) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		project.Title = opts.Title.ValueOrDefault(project.Title)
		project.Description = opts.Description.ValueOrDefault(project.Description)
		project.CardType = opts.CardType.ValueOrDefault(project.CardType)
		if err := project_model.UpdateProject(ctx, project); err != nil {
			return err
		}
		if opts.IsClosed.Has() && opts.IsClosed.Value() != project.IsClosed {
			if err := project_model.ChangeProjectStatus(ctx, project, opts.IsClosed.Value()); err != nil {
				return err
			}
		}
		return nil
	})
}
