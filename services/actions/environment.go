// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	actions_model "gitea.dev/models/actions"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
)

// GetOrCreateEnvironment trims and validates name, then returns the environment, creating it when absent.
func GetOrCreateEnvironment(ctx context.Context, repo *repo_model.Repository, name string, branchPatterns []string) (env *actions_model.ActionEnvironment, created bool, err error) {
	name = strings.TrimSpace(name)
	if err := actions_model.ValidateEnvironmentName(name); err != nil {
		return nil, false, err
	}
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil {
		return nil, false, err
	}

	env, err = actions_model.GetEnvironmentByRepoAndName(ctx, repo.ID, name)
	if err == nil {
		return env, false, nil
	}
	if !errors.Is(err, util.ErrNotExist) {
		return nil, false, err
	}

	env, err = actions_model.InsertEnvironment(ctx, repo.ID, name, patterns)
	if err != nil {
		// the constraint text differs per driver, so re-fetch to tell a lost race from a real failure
		if existing, lookupErr := actions_model.GetEnvironmentByRepoAndName(ctx, repo.ID, name); lookupErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	audit.Record(ctx, audit_model.RepositoryEnvironmentAdd, repo, "environment", env.Name)
	return env, true, nil
}

func UpdateEnvironment(ctx context.Context, repo *repo_model.Repository, env *actions_model.ActionEnvironment, branchPatterns []string) error {
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil || patterns == env.AllowedBranchPatterns {
		return err
	}
	env.AllowedBranchPatterns = patterns
	if err := actions_model.UpdateEnvironment(ctx, env); err != nil {
		return err
	}
	audit.Record(ctx, audit_model.RepositoryEnvironmentUpdate, repo, "environment", env.Name)
	return nil
}

func DeleteEnvironment(ctx context.Context, repo *repo_model.Repository, env *actions_model.ActionEnvironment) error {
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		return db.DeleteBeans(ctx,
			&secret_model.Secret{RepoID: env.RepoID, EnvironmentID: env.ID},
			&actions_model.ActionVariable{RepoID: env.RepoID, EnvironmentID: env.ID},
			&actions_model.ActionEnvironment{ID: env.ID},
		)
	}); err != nil {
		return err
	}
	audit.Record(ctx, audit_model.RepositoryEnvironmentRemove, repo, "environment", env.Name)
	return nil
}

// resolveJobEnvironment returns the environment a job deploys to (nil for none), or why the job is denied.
// A job naming an environment it cannot get must fail, not run without its branch policy.
func resolveJobEnvironment(ctx context.Context, job *actions_model.ActionRunJob) (env *actions_model.ActionEnvironment, denied, err error) {
	parsed, err := job.ParseJob()
	if err != nil {
		return nil, nil, err
	}
	name := parsed.DeploymentEnvironmentName()
	if name == "" {
		return nil, nil, nil
	}
	if err := job.LoadRun(ctx); err != nil {
		return nil, nil, err
	}
	run := job.Run
	if run.IsUntrustedFork() {
		return nil, nil, nil
	}
	if err := run.LoadRepo(ctx); err != nil {
		return nil, nil, err
	}
	if err := run.LoadTriggerUser(ctx); err != nil {
		return nil, nil, err
	}

	// whoever triggered the run is the one creating an environment it names for the first time
	env, _, err = GetOrCreateEnvironment(audit.WithDoer(ctx, run.TriggerUser), run.Repo, name, nil)
	if errors.Is(err, util.ErrInvalidArgument) {
		return nil, err, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if _, _, ref, _ := runRefs(run); !env.MatchesRef(ref) {
		return nil, fmt.Errorf("branch is not allowed to deploy to %q due to environment protection rules", env.Name), nil
	}
	return env, nil, nil
}
