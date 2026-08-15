// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"

	actions_model "gitea.dev/models/actions"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/util"
	secret_service "gitea.dev/services/secrets"
)

// CreateEnvironment adds a deployment environment to a repository.
func CreateEnvironment(ctx context.Context, repoID int64, name string, branchPatterns []string) (*actions_model.ActionEnvironment, error) {
	if err := actions_model.ValidateEnvironmentName(name); err != nil {
		return nil, err
	}
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil {
		return nil, err
	}

	env, err := actions_model.InsertEnvironment(ctx, repoID, name, patterns)
	if err != nil {
		// Re-check by name rather than parsing driver-specific constraint text, which differs per
		// database and per locale. This also closes the race a pre-flight existence check leaves open.
		if existing, lookupErr := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name); lookupErr == nil {
			return nil, actions_model.ErrEnvironmentAlreadyExists{Name: existing.Name}
		}
		return nil, err
	}
	return env, nil
}

// UpdateEnvironment replaces the mutable fields of an existing environment.
func UpdateEnvironment(ctx context.Context, env *actions_model.ActionEnvironment, name string, branchPatterns []string) error {
	if err := actions_model.ValidateEnvironmentName(name); err != nil {
		return err
	}
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil {
		return err
	}

	if !strings.EqualFold(name, env.Name) {
		if _, err := actions_model.GetEnvironmentByRepoAndName(ctx, env.RepoID, name); err == nil {
			return actions_model.ErrEnvironmentAlreadyExists{Name: name}
		} else if !errors.Is(err, util.ErrNotExist) {
			return err
		}
	}

	env.Name = name
	env.AllowedBranchPatterns = patterns
	return actions_model.UpdateEnvironment(ctx, env)
}

// CreateOrUpdateEnvironment backs the idempotent PUT endpoint. The bool reports whether it created the environment.
func CreateOrUpdateEnvironment(ctx context.Context, repoID int64, name string, branchPatterns []string) (*actions_model.ActionEnvironment, bool, error) {
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name)
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			return nil, false, err
		}
		created, err := CreateEnvironment(ctx, repoID, name, branchPatterns)
		return created, true, err
	}
	return env, false, UpdateEnvironment(ctx, env, name, branchPatterns)
}

// GetOrCreateEnvironment resolves an environment named by a workflow, creating it on first reference
// the way GitHub does. Callers must not use it for fork pull requests, which would let an outside
// contributor write rows into a repository they cannot otherwise modify.
func GetOrCreateEnvironment(ctx context.Context, repoID int64, name string) (*actions_model.ActionEnvironment, error) {
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name)
	if err == nil {
		return env, nil
	}
	if !errors.Is(err, util.ErrNotExist) {
		return nil, err
	}

	env, err = CreateEnvironment(ctx, repoID, name, nil)
	if errors.Is(err, util.ErrAlreadyExist) { // lost a race against a concurrent run
		return actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name)
	}
	return env, err
}

func DeleteEnvironment(ctx context.Context, repoID, envID int64) error {
	return actions_model.DeleteEnvironment(ctx, repoID, envID)
}

func CreateOrUpdateEnvSecret(ctx context.Context, repoID, envID int64, name, data, description string) (*secret_model.Secret, bool, error) {
	return secret_service.CreateOrUpdateSecret(ctx, 0, repoID, envID, name, data, description)
}

func DeleteEnvSecret(ctx context.Context, repoID, envID int64, name string) error {
	return secret_service.DeleteSecretByName(ctx, 0, repoID, envID, name)
}

func CreateEnvVariable(ctx context.Context, repoID, envID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	return CreateVariable(ctx, 0, repoID, envID, name, data, description)
}

// findEnvVariable scopes the lookup to the environment so a variable of another scope cannot be reached by ID.
func findEnvVariable(ctx context.Context, repoID, envID, varID int64) (*actions_model.ActionVariable, error) {
	return GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID:        repoID,
		EnvironmentID: envID,
		IDs:           []int64{varID},
	})
}

func UpdateEnvVariable(ctx context.Context, repoID, envID, varID int64, name, data, description string) (*actions_model.ActionVariable, error) {
	v, err := findEnvVariable(ctx, repoID, envID, varID)
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

func DeleteEnvVariable(ctx context.Context, repoID, envID, varID int64) error {
	v, err := findEnvVariable(ctx, repoID, envID, varID)
	if err != nil {
		return err
	}
	return DeleteVariableByID(ctx, v.ID)
}
