// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	action_model "code.gitea.io/gitea/models/actions"
)

func CreateRequireAction(ctx context.Context, ownerID, repoID int64, name, data string) (*action_model.RequireAction, error) {

	s, err := action_model.InsertRequireAction(ctx, ownerID, repoID, name, data)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func DeleteRequireActionByID(ctx context.Context, requireActionID int64) error {
	s, err := db.Find[action_model.RequireAction](ctx, action_model.FindRequireActionOptions{
		RequireActionID: requireActionID,
	})
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return action_model.ErrRequireActionNotFound{}
	}

	return deleteRequireAction(ctx, s[0])
}

func deleteRequireAction(ctx context.Context, s *action_model.RequireAction) error {
	if _, err := db.DeleteByID[action_model.RequireAction](ctx, s.ID); err != nil {
		return err
	}
	return nil
}
