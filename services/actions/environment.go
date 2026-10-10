// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

// GetOrCreateEnvironment trims and validates name, then returns the environment, creating it when absent.
func GetOrCreateEnvironment(ctx context.Context, repoID int64, name string, branchPatterns []string) (env *actions_model.ActionEnvironment, created bool, err error) {
	name = strings.TrimSpace(name)
	if err := actions_model.ValidateEnvironmentName(name); err != nil {
		return nil, false, err
	}
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil {
		return nil, false, err
	}

	env, err = actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name)
	if err == nil {
		return env, false, nil
	}
	if !errors.Is(err, util.ErrNotExist) {
		return nil, false, err
	}

	env, err = actions_model.InsertEnvironment(ctx, repoID, name, patterns)
	if err != nil {
		// the constraint text differs per driver, so re-fetch to tell a lost race from a real failure
		if existing, lookupErr := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name); lookupErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return env, true, nil
}

func UpdateEnvironment(ctx context.Context, env *actions_model.ActionEnvironment, branchPatterns []string) (changed bool, err error) {
	patterns, err := actions_model.JoinBranchPatterns(branchPatterns)
	if err != nil {
		return false, err
	}
	if patterns == env.AllowedBranchPatterns {
		return false, nil
	}
	env.AllowedBranchPatterns = patterns
	if err := actions_model.UpdateEnvironment(ctx, env); err != nil {
		return false, err
	}
	return true, nil
}

// ResolveJobEnvironment returns the environment a job deploys to (nil for none) and, when the job may not
// run, the reason. A job naming an environment it cannot get must fail, not run without its branch policy.
func ResolveJobEnvironment(ctx context.Context, job *actions_model.ActionRunJob) (env *actions_model.ActionEnvironment, denyReason string, err error) {
	if job.EnvironmentName == "" {
		return nil, "", nil
	}
	if err := job.LoadRun(ctx); err != nil {
		return nil, "", err
	}
	if job.Run.IsUntrustedFork() {
		return nil, "", nil
	}

	env, _, err = GetOrCreateEnvironment(ctx, job.RepoID, job.EnvironmentName, nil)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			return nil, fmt.Sprintf("Environment name `%s` is invalid.", job.EnvironmentName), nil
		}
		return nil, "", err
	}
	if !env.MatchesRef(runRef(job.Run)) {
		return env, fmt.Sprintf("Branch is not allowed to deploy to `%s` due to environment protection rules.", env.Name), nil
	}
	return env, "", nil
}

// EnsureEnvironments creates the named environments up front so they show in the settings UI before any job runs.
// It must stay outside the transaction inserting the jobs: a lost creation race would abort it on PostgreSQL.
func EnsureEnvironments(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) {
	if run.IsUntrustedFork() {
		return
	}
	seen := make(container.Set[string])
	for _, job := range jobs {
		if job.EnvironmentName == "" || !seen.Add(strings.ToLower(job.EnvironmentName)) {
			continue
		}
		if _, _, err := GetOrCreateEnvironment(ctx, job.RepoID, job.EnvironmentName, nil); err != nil {
			log.Error("Cannot resolve environment %q of repo %d: %v", job.EnvironmentName, job.RepoID, err)
		}
	}
}

// runRef is the ref github.ref reports: pull_request_target runs on the base branch.
func runRef(run *actions_model.ActionRun) string {
	if base := run.PullRequestTargetBase(); base != nil {
		return git.BranchPrefix + base.Ref
	}
	return run.Ref
}
