// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/util"
	secret_service "code.gitea.io/gitea/services/secrets"
)

func CreateVariable(ctx context.Context, ownerID, repoID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	if err := secret_service.ValidateName(name); err != nil {
		return nil, err
	}

	v, err := actions_model.InsertVariable(ctx, ownerID, repoID, name, util.ReserveLineBreakForTextarea(data), description)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func UpdateVariableNameData(ctx context.Context, variable *actions_model.ActionVariable) (bool, error) {
	if err := secret_service.ValidateName(variable.Name); err != nil {
		return false, err
	}

	variable.Data = util.ReserveLineBreakForTextarea(variable.Data)

	return actions_model.UpdateVariableCols(ctx, variable, "name", "data", "description")
}

func DeleteVariableByID(ctx context.Context, variableID int64) error {
	return actions_model.DeleteVariable(ctx, variableID)
}

func DeleteVariableByName(ctx context.Context, ownerID, repoID int64, name string) error {
	v, err := GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    name,
	})
	if err != nil {
		return err
	}

	return actions_model.DeleteVariable(ctx, v.ID)
}

func GetVariable(ctx context.Context, opts actions_model.FindVariablesOpts) (*actions_model.ActionVariable, error) {
	vars, err := actions_model.FindVariables(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(vars) != 1 {
		return nil, util.NewNotExistErrorf("variable not found")
	}
	return vars[0], nil
}
