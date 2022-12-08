// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
)

// DeleteProjectByID deletes a project from a repository.
func DeleteProjectByID(ctx context.Context, id int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := project_model.DeleteProjectByID(ctx, id); err != nil {
		return err
	}

	return committer.Commit()
}
