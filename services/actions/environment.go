// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "gitea.dev/models/actions"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/util"
	secret_service "gitea.dev/services/secrets"
)

// CreateEnvironment creates a new deployment environment for a repository.
func CreateEnvironment(ctx context.Context, repoID int64, name, protectedBranches string) (*actions_model.ActionEnvironment, error) {
	if name == "" {
		return nil, util.NewInvalidArgumentErrorf("environment name cannot be empty")
	}
	if len(name) > 255 {
		return nil, util.NewInvalidArgumentErrorf("environment name too long")
	}
	env, err := actions_model.InsertEnvironment(ctx, repoID, name, protectedBranches)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, actions_model.ErrEnvironmentAlreadyExists{Name: name}
		}
		return nil, err
	}
	return env, nil
}

// UpdateEnvironment updates an existing environment.
func UpdateEnvironment(ctx context.Context, repoID, envID int64, name, protectedBranches string) (*actions_model.ActionEnvironment, error) {
	env, err := actions_model.GetEnvironmentByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if env.RepoID != repoID {
		return nil, util.ErrNotExist
	}
	if name != "" && name != env.Name {
		if existing, err := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name); err == nil && existing != nil {
			return nil, actions_model.ErrEnvironmentAlreadyExists{Name: name}
		}
		env.Name = name
	}
	env.ProtectedBranches = protectedBranches
	return env, actions_model.UpdateEnvironment(ctx, env)
}

// DeleteEnvironment removes an environment and its scoped secrets/variables.
func DeleteEnvironment(ctx context.Context, repoID, envID int64) error {
	return actions_model.DeleteEnvironment(ctx, repoID, envID)
}

// CreateOrUpdateEnvSecret creates or updates a secret scoped to an environment.
func CreateOrUpdateEnvSecret(ctx context.Context, repoID, envID int64, name, data, description string) (*secret_model.Secret, bool, error) {
	return secret_service.CreateOrUpdateSecret(ctx, 0, repoID, envID, name, data, description)
}

// DeleteEnvSecret removes a secret from an environment.
func DeleteEnvSecret(ctx context.Context, repoID, envID int64, name string) error {
	return secret_service.DeleteSecretByName(ctx, 0, repoID, envID, name)
}

// CreateEnvVariable creates a variable scoped to an environment.
func CreateEnvVariable(ctx context.Context, repoID, envID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	return CreateVariable(ctx, 0, repoID, envID, name, data, description)
}

// UpdateEnvVariable updates a variable scoped to an environment.
func UpdateEnvVariable(ctx context.Context, repoID, envID, varID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	v, err := GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID:        repoID,
		EnvironmentID: envID,
		IDs:           []int64{varID},
	})
	if err != nil {
		return nil, err
	}
	if name != "" {
		v.Name = name
	}
	v.Data = data
	v.Description = description
	if _, err := UpdateVariableNameData(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// DeleteEnvVariable removes a variable from an environment.
func DeleteEnvVariable(ctx context.Context, repoID, envID, varID int64) error {
	v, err := GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID:        repoID,
		EnvironmentID: envID,
		IDs:           []int64{varID},
	})
	if err != nil {
		return err
	}
	return DeleteVariableByID(ctx, v.ID)
}
