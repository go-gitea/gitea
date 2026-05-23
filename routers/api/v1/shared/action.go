// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"fmt"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ListJobs lists jobs for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all jobs
// ownerID == 0 and repoID != 0 means all jobs for the given repo
// ownerID != 0 and repoID == 0 means all jobs for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// runID == 0 means all jobs
// runID is used as an additional filter together with ownerID and repoID to only return jobs for the given run
// runAttemptID, when set, additionally limits the result to jobs of the specified run attempt. Only takes effect when runID > 0.
// Access rights are checked at the API route level
func ListJobs(ctx *context.APIContext, ownerID, repoID, runID int64, runAttemptID optional.Option[int64]) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	listOptions := utils.GetListOptions(ctx)
	orderBy, ok := utils.ResolveSortOrder(ctx, actions_model.JobOrderByMap, actions_model.JobOrderByMap["asc"]["id"])
	if !ok {
		return
	}
	opts := actions_model.FindRunJobOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		RunID:       runID,
		ListOptions: listOptions,
		OrderBy:     orderBy,
	}
	if runID > 0 {
		opts.RunAttemptID = runAttemptID
	}
	for _, status := range ctx.FormStrings("status") {
		values, err := convertToInternal(status)
		if err != nil {
			ctx.APIError(http.StatusBadRequest, fmt.Errorf("Invalid status %s", status))
			return
		}
		opts.Statuses = append(opts.Statuses, values...)
	}

	jobs, total, err := db.FindAndCount[actions_model.ActionRunJob](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionWorkflowJobsResponse)
	res.TotalCount = total

	jobList := actions_model.ActionJobList(jobs)
	if err := jobList.LoadAttributes(ctx, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res.Entries = make([]*api.ActionWorkflowJob, len(jobs))

	isRepoLevel := repoID != 0 && ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == repoID
	for i := range jobs {
		var repository *repo_model.Repository
		if isRepoLevel {
			repository = ctx.Repo.Repository
		} else {
			if jobs[i].Run == nil || jobs[i].Run.Repo == nil {
				ctx.APIErrorInternal(fmt.Errorf("job %d is missing its run or repository", jobs[i].ID))
				return
			}
			repository = jobs[i].Run.Repo
		}

		convertedWorkflowJob, err := convert.ToActionWorkflowJob(ctx, repository, nil, jobs[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedWorkflowJob
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
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
	listOptions := utils.GetListOptions(ctx)
	opts := actions_model.FindRunOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: listOptions,
	}

	if event := ctx.FormString("event"); event != "" {
		opts.TriggerEvent = webhook.HookEventType(event)
	}
	if branch := ctx.FormString("branch"); branch != "" {
		opts.Ref = string(git.RefNameFromBranch(branch))
	}
	for _, status := range ctx.FormStrings("status") {
		values, err := convertToInternal(status)
		if err != nil {
			ctx.APIError(http.StatusBadRequest, fmt.Errorf("Invalid status %s", status))
			return
		}
		opts.Status = append(opts.Status, values...)
	}
	if actor := ctx.FormString("actor"); actor != "" {
		user, err := user_model.GetUserByName(ctx, actor)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		opts.TriggerUserID = user.ID
	}
	if headSHA := ctx.FormString("head_sha"); headSHA != "" {
		opts.CommitSHA = headSHA
	}

	runs, total, err := db.FindAndCount[actions_model.ActionRun](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionWorkflowRunsResponse)
	res.TotalCount = total

	runList := actions_model.RunList(runs)
	if err := runList.LoadTriggerUser(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := runList.LoadRepos(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	repos := repo_model.RepositoryList(container.FilterSlice(runs, func(r *actions_model.ActionRun) (*repo_model.Repository, bool) {
		return r.Repo, r.Repo != nil
	}))
	if err := repos.LoadOwners(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res.Entries = make([]*api.ActionWorkflowRun, len(runs))
	for i := range runs {
		// TODO: load run attempts in batch
		convertedRun, err := convert.ToActionWorkflowRun(ctx, runs[i], nil)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		res.Entries[i] = convertedRun
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &res)
}
