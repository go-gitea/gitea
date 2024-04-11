// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
)

func CreateRequireAction(ctx context.Context, orgID int64, repoName, workflowName string) (*actions_model.RequireAction, error) {
	v, err := actions_model.AddRequireAction(ctx, orgID, repoName, workflowName)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func DeleteRequireActionByID(ctx context.Context, requireActionID int64) error {
	return actions_model.DeleteRequireAction(ctx, requireActionID)
}
