// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"fmt"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// RegistrationToken is response related to registration token
// swagger:response RegistrationToken
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	token, err := actions_model.GetLatestRunnerToken(ctx, ownerID, repoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, ownerID, repoID)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}

// ListRunners lists runners for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all runners including global runners, does not appear in sql where clause
// ownerID == 0 and repoID != 0 means all runners for the given repo
// ownerID != 0 and repoID == 0 means all runners for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func ListRunners(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	runners, total, err := db.FindAndCount[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionRunnersResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionRunner, len(runners))
	for i, runner := range runners {
		res.Entries[i] = convert.ToActionRunner(ctx, runner)
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetRunner get the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func GetRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.APIErrorNotFound(err)
		return
	}
	if !runner.EditableInContext(ownerID, repoID) {
		ctx.APIErrorNotFound("No permission to get this runner")
		return
	}
	ctx.JSON(http.StatusOK, convert.ToActionRunner(ctx, runner))
}

// DeleteRunner deletes the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func DeleteRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !runner.EditableInContext(ownerID, repoID) {
		ctx.APIErrorNotFound("No permission to delete this runner")
		return
	}

	err = actions_model.DeleteRunner(ctx, runner.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListJobs lists jobs for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all jobs
// ownerID == 0 and repoID != 0 means all jobs for the given repo
// ownerID != 0 and repoID == 0 means all jobs for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// runID == 0 means all jobs
// Access rights are checked at the API route level
func ListJobs(ctx *context.APIContext, ownerID, repoID, runID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	opts := actions_model.FindRunJobOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		RunID:       runID,
		ListOptions: utils.GetListOptions(ctx),
	}
	if statuses, ok := ctx.Req.URL.Query()["status"]; ok {
		for _, status := range statuses {
			values, err := convertToInternal(status)
			if err != nil {
				ctx.APIError(http.StatusBadRequest, fmt.Errorf("Invalid status %s", status))
				return
			}
			opts.Statuses = append(opts.Statuses, values...)
		}
	}

	jobs, total, err := db.FindAndCount[actions_model.ActionRunJob](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionWorkflowJobsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionWorkflowJob, len(jobs))

	isRepoLevel := repoID != 0 && ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == repoID
	for i := range jobs {
		var repository *repo_model.Repository
		if isRepoLevel {
			repository = ctx.Repo.Repository
		} else {
			repository, err = repo_model.GetRepositoryByID(ctx, repoID)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
		}

		convertedWorkflowJob, err := convert.ToActionWorkflowJob(ctx, repository, nil, jobs[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedWorkflowJob
	}

	ctx.JSON(http.StatusOK, &res)
}

func convertToInternal(s string) ([]actions_model.Status, error) {
	switch s {
	case "pending", "waiting", "requested", "action_required":
		return []actions_model.Status{actions_model.StatusBlocked}, nil
	case "queued":
		return []actions_model.Status{actions_model.StatusWaiting}, nil
	case "in_progress":
		return []actions_model.Status{actions_model.StatusRunning}, nil
	case "completed":
		return []actions_model.Status{
			actions_model.StatusSuccess,
			actions_model.StatusFailure,
			actions_model.StatusSkipped,
			actions_model.StatusCancelled,
		}, nil
	case "failure":
		return []actions_model.Status{actions_model.StatusFailure}, nil
	case "success":
		return []actions_model.Status{actions_model.StatusSuccess}, nil
	case "skipped", "neutral":
		return []actions_model.Status{actions_model.StatusSkipped}, nil
	case "cancelled", "timed_out":
		return []actions_model.Status{actions_model.StatusCancelled}, nil
	default:
		return nil, fmt.Errorf("invalid status %s", s)
	}
}

// ListRuns lists jobs for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all runs
// ownerID == 0 and repoID != 0 means all runs for the given repo
// ownerID != 0 and repoID == 0 means all runs for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func ListRuns(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	opts := actions_model.FindRunOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: utils.GetListOptions(ctx),
	}

	if event := ctx.Req.URL.Query().Get("event"); event != "" {
		opts.TriggerEvent = webhook.HookEventType(event)
	}
	if branch := ctx.Req.URL.Query().Get("branch"); branch != "" {
		opts.Ref = string(git.RefNameFromBranch(branch))
	}
	if statuses, ok := ctx.Req.URL.Query()["status"]; ok {
		for _, status := range statuses {
			values, err := convertToInternal(status)
			if err != nil {
				ctx.APIError(http.StatusBadRequest, fmt.Errorf("Invalid status %s", status))
				return
			}
			opts.Status = append(opts.Status, values...)
		}
	}
	if actor := ctx.Req.URL.Query().Get("actor"); actor != "" {
		user, err := user_model.GetUserByName(ctx, actor)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		opts.TriggerUserID = user.ID
	}

	runs, total, err := db.FindAndCount[actions_model.ActionRun](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionWorkflowRunsResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionWorkflowRun, len(runs))
	isRepoLevel := repoID != 0 && ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == repoID
	for i := range runs {
		var repository *repo_model.Repository
		if isRepoLevel {
			repository = ctx.Repo.Repository
		} else {
			repository, err = repo_model.GetRepositoryByID(ctx, runs[i].RepoID)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
		}

		convertedRun, err := convert.ToActionWorkflowRun(ctx, repository, runs[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedRun
	}

	ctx.JSON(http.StatusOK, &res)
}
