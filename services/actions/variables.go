// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	secret_service "gitea.dev/services/secrets"
)

func CreateVariable(ctx context.Context, scope actions_model.BelongingScope, name, data, description string) (*actions_model.ActionVariable, error) {
	if err := secret_service.ValidateName(name); err != nil {
		return nil, err
	}

	v, err := actions_model.InsertVariable(ctx, scope, name, util.NormalizeStringEOL(data), description)
	if err != nil {
		return nil, err
	}
	audit.RecordScopedByID(ctx, v.OwnerID, v.RepoID, audit.VariableAdd, "variable", v.Name)

	return v, nil
}

func UpdateVariableNameData(ctx context.Context, variable *actions_model.ActionVariable) (bool, error) {
	if err := secret_service.ValidateName(variable.Name); err != nil {
		return false, err
	}

	variable.Data = util.NormalizeStringEOL(variable.Data)

	updated, err := actions_model.UpdateVariableCols(ctx, variable, "name", "data", "description")
	if updated {
		audit.RecordScopedByID(ctx, variable.OwnerID, variable.RepoID, audit.VariableUpdate, "variable", variable.Name)
	}
	return updated, err
}

func DeleteVariable(ctx context.Context, variable *actions_model.ActionVariable) error {
	if err := actions_model.DeleteVariable(ctx, variable.ID); err != nil {
		return err
	}
	audit.RecordScopedByID(ctx, variable.OwnerID, variable.RepoID, audit.VariableRemove, "variable", variable.Name)
	return nil
}

func DeleteVariableByName(ctx context.Context, scope actions_model.BelongingScope, name string) error {
	v, err := GetVariable(ctx, actions_model.FindVariablesOpts{BelongingScope: scope, Name: name})
	if err != nil {
		return err
	}
	return DeleteVariable(ctx, v)
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
