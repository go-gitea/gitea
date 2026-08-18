// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"

	actions_model "gitea.dev/models/actions"
	secret_model "gitea.dev/models/secret"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
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
		if err == nil {
			return created, true, nil
		}
		if !errors.Is(err, util.ErrAlreadyExist) {
			return nil, false, err
		}
		// a concurrent PUT won the race, so update the row it created
		if env, err = actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name); err != nil {
			return nil, false, err
		}
	}
	return env, false, UpdateEnvironment(ctx, env, name, branchPatterns)
}

// jobEnvironmentName is the "environment:" name as a job row stores it, capped to what the column
// and ValidateEnvironmentName accept so that inserting a job can never fail on it.
func jobEnvironmentName(job *jobparser.Job) string {
	return util.EllipsisDisplayString(job.DeploymentEnvironmentName(), actions_model.EnvironmentNameMaxLength)
}

// ResolveJobEnvironment returns the environment a job deploys to, nil when it names none, and
// whether it may run. A job that names one it cannot get must fail rather than run with the
// repository's credentials and no branch policy.
// It creates the environment on first reference, so that a runner picking the job up before
// EnsureEnvironments got to it waits for the next poll instead of failing.
func ResolveJobEnvironment(ctx context.Context, job *actions_model.ActionRunJob) (*actions_model.ActionEnvironment, bool, error) {
	if job.EnvironmentName == "" {
		return nil, true, nil
	}
	if err := job.LoadRun(ctx); err != nil {
		return nil, false, err
	}
	if actions_module.IsUntrustedForkRun(job.Run) {
		// no environment is ever created for it, and its secrets are withheld regardless
		return nil, true, nil
	}

	env, err := GetOrCreateEnvironment(ctx, job.RepoID, job.EnvironmentName)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			return nil, false, nil // no environment can carry this name, so the job can never deploy
		}
		return nil, false, err
	}
	return env, env.MatchesRef(job.Run.Ref), nil
}

// EnsureEnvironments creates the environments named by jobs on first reference, the way GitHub does.
// It must stay outside the transaction inserting those jobs: losing the creation race there would abort
// that whole transaction on PostgreSQL.
func EnsureEnvironments(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) {
	if actions_module.IsUntrustedForkRun(run) { // its workflow must not write into the base repository
		return
	}
	seen := make(container.Set[string])
	for _, job := range jobs {
		// the unique constraint is on the lowercased name, so two spellings would collide on insert
		if job.EnvironmentName == "" || !seen.Add(strings.ToLower(job.EnvironmentName)) {
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
