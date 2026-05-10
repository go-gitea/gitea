// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
)

func setActionTokenScope(ctx context.Context, store DataStore, task *actions_model.ActionTask) error {
	store.GetData()["LoginMethod"] = ActionTokenMethodName

	if err := task.LoadJob(ctx); err != nil {
		return err
	}
	if err := task.Job.LoadRepo(ctx); err != nil {
		return err
	}

	tokenPerms, err := actions_model.ComputeTaskTokenPermissions(ctx, task, task.Job.Repo)
	if err != nil {
		return err
	}

	var scope auth_model.AccessTokenScope
	packageAccess := tokenPerms.UnitAccessModes[unit.TypePackages]
	switch {
	case packageAccess >= perm.AccessModeWrite:
		scope = auth_model.AccessTokenScopeWritePackage
	case packageAccess >= perm.AccessModeRead:
		scope = auth_model.AccessTokenScopeReadPackage
	default:
		return nil
	}

	store.GetData()["IsApiToken"] = true
	store.GetData()["ApiTokenScope"] = scope
	return nil
}

func setActionTokenScopeByTaskID(ctx context.Context, store DataStore, taskID int64) error {
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return err
	}
	return setActionTokenScope(ctx, store, task)
}
