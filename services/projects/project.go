// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/optional"
)

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
