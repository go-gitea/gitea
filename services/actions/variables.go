// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/util"
	secret_service "gitea.dev/services/secrets"
)

func CreateVariable(ctx context.Context, ownerID, repoID, environmentID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	if err := secret_service.ValidateName(name); err != nil {
		return nil, err
	}

	v, err := actions_model.InsertVariable(ctx, ownerID, repoID, environmentID, name, util.NormalizeStringEOL(data), description)
	if err != nil {
		// Re-check by name rather than parsing driver-specific constraint text, which differs per
		// database and per locale. This also closes the race a pre-flight existence check leaves open.
		if _, lookupErr := GetVariable(ctx, actions_model.FindVariablesOpts{
			OwnerID:       ownerID,
			RepoID:        repoID,
			EnvironmentID: environmentID,
			Name:          name,
		}); lookupErr == nil {
			return nil, util.NewAlreadyExistErrorf("variable %s already exists", strings.ToUpper(name))
		}
		return nil, err
	}

	return v, nil
}

func UpdateVariableNameData(ctx context.Context, variable *actions_model.ActionVariable) (bool, error) {
	if err := secret_service.ValidateName(variable.Name); err != nil {
		return false, err
	}

	variable.Data = util.NormalizeStringEOL(variable.Data)

	return actions_model.UpdateVariableCols(ctx, variable, "name", "data", "description")
}

func DeleteVariableByID(ctx context.Context, variableID int64) error {
	return actions_model.DeleteVariable(ctx, variableID)
}

func DeleteVariableByName(ctx context.Context, ownerID, repoID, environmentID int64, name string) error {
	v, err := GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID:       ownerID,
		RepoID:        repoID,
		EnvironmentID: environmentID,
		Name:          name,
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
