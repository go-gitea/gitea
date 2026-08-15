// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"

	actions_model "gitea.dev/models/actions"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
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
		// re-check by name: constraint text differs per driver, and a pre-flight check would still race
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

// EnsureEnvironments creates the environments named by jobs on first reference, the way GitHub does.
// It must stay outside the transaction inserting those jobs: losing the creation race there would abort
// that whole transaction on PostgreSQL.
func EnsureEnvironments(ctx context.Context, jobs []*actions_model.ActionRunJob) {
	seen := make(container.Set[string])
	for _, job := range jobs {
		// a fork's workflow must not write rows into the base repository's settings
		if job.EnvironmentName == "" || job.IsForkPullRequest || !seen.Add(job.EnvironmentName) {
			continue
		}
		if _, err := GetOrCreateEnvironment(ctx, job.RepoID, job.EnvironmentName); err != nil {
			log.Error("Cannot resolve environment %q of repo %d: %v", job.EnvironmentName, job.RepoID, err)
		}
	}
}

// GetOrCreateEnvironment resolves an environment named by a workflow, creating it on first reference.
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

// GetEnvVariable scopes the lookup to the environment so a variable of another scope cannot be reached.
func GetEnvVariable(ctx context.Context, repoID, envID int64, name string) (*actions_model.ActionVariable, error) {
	return GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID:        repoID,
		EnvironmentID: envID,
		Name:          name,
	})
}

func UpdateEnvVariable(ctx context.Context, v *actions_model.ActionVariable, name, data, description string) error {
	if name != "" {
		v.Name = name
	}
	v.Data = data
	v.Description = description
	_, err := UpdateVariableNameData(ctx, v)
	return err
}

func DeleteEnvVariable(ctx context.Context, repoID, envID int64, name string) error {
	return DeleteVariableByName(ctx, 0, repoID, envID, name)
}
