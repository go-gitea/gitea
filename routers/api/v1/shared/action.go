// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// parseISO8601DateRange parses flexible date formats: YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ (ISO8601)
func parseISO8601DateRange(dateStr string) (time.Time, error) {
	// Try ISO8601 format first: 2017-01-01T01:00:00+07:00 or 2016-03-21T14:11:00Z
	if strings.Contains(dateStr, "T") {
		// Try with timezone offset (RFC3339)
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			return t, nil
		}
		// Try with Z suffix (UTC)
		if t, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
			return t, nil
		}
		// Try without timezone
		if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			return t, nil
		}
	}
	// Try simple date format: YYYY-MM-DD
	return time.Parse("2006-01-02", dateStr)
}

// ListJobs lists jobs for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all jobs
// ownerID == 0 and repoID != 0 means all jobs for the given repo
// ownerID != 0 and repoID == 0 means all jobs for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// runID == 0 means all jobs
// runID is used as an additional filter together with ownerID and repoID to only return jobs for the given run
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

	res.Entries = make([]*api.ActionWorkflowJob, len(jobs))

	isRepoLevel := repoID != 0 && ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == repoID
	for i := range jobs {
		var repository *repo_model.Repository
		if isRepoLevel {
			repository = ctx.Repo.Repository
		} else {
			repository, err = repo_model.GetRepositoryByID(ctx, jobs[i].RepoID)
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
// buildRunOptions builds the FindRunOptions from context parameters
func buildRunOptions(ctx *context.APIContext, ownerID, repoID int64) (actions_model.FindRunOptions, error) {
	opts := actions_model.FindRunOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		WorkflowID:  ctx.PathParam("workflow_id"),
		ListOptions: utils.GetListOptions(ctx),
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
			return opts, fmt.Errorf("Invalid status %s", status)
		}
		opts.Status = append(opts.Status, values...)
	}
	if actor := ctx.FormString("actor"); actor != "" {
		user, err := user_model.GetUserByName(ctx, actor)
		if err != nil {
			return opts, err
		}
		opts.TriggerUserID = user.ID
	}
	if headSHA := ctx.FormString("head_sha"); headSHA != "" {
		opts.CommitSHA = headSHA
	}

	// Handle exclude_pull_requests parameter
	if ctx.FormBool("exclude_pull_requests") {
		opts.ExcludePullRequests = true
	}

	// Handle created parameter for date filtering
	// Supports ISO8601 date formats: YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ
	if created := ctx.FormString("created"); created != "" {
		// Parse the date range in the format like ">=2023-01-01", "<=2023-12-31", or "2023-01-01..2023-12-31"
		if strings.Contains(created, "..") {
			// Range format: "2023-01-01..2023-12-31"
			dateRange := strings.Split(created, "..")
			if len(dateRange) == 2 {
				startDate, err := parseISO8601DateRange(dateRange[0])
				if err == nil {
					opts.CreatedAfter = startDate
				}

				endDate, err := parseISO8601DateRange(dateRange[1])
				if err == nil {
					// Set to end of day if only date provided
					if !strings.Contains(dateRange[1], "T") {
						endDate = endDate.Add(24*time.Hour - time.Second)
					}
					opts.CreatedBefore = endDate
				}
			}
		} else if after, ok := strings.CutPrefix(created, ">="); ok {
			// Greater than or equal format: ">=2023-01-01"
			startDate, err := parseISO8601DateRange(after)
			if err == nil {
				opts.CreatedAfter = startDate
			}
		} else if after, ok := strings.CutPrefix(created, ">"); ok {
			// Greater than format: ">2023-01-01"
			startDate, err := parseISO8601DateRange(after)
			if err == nil {
				if strings.Contains(after, "T") {
					opts.CreatedAfter = startDate.Add(time.Second)
				} else {
					opts.CreatedAfter = startDate.Add(24 * time.Hour)
				}
			}
		} else if after, ok := strings.CutPrefix(created, "<="); ok {
			// Less than or equal format: "<=2023-12-31"
			endDate, err := parseISO8601DateRange(after)
			if err == nil {
				// Set to end of day if only date provided
				if !strings.Contains(after, "T") {
					endDate = endDate.Add(24*time.Hour - time.Second)
				}
				opts.CreatedBefore = endDate
			}
		} else if after, ok := strings.CutPrefix(created, "<"); ok {
			// Less than format: "<2023-12-31"
			endDate, err := parseISO8601DateRange(after)
			if err == nil {
				if strings.Contains(after, "T") {
					opts.CreatedBefore = endDate.Add(-time.Second)
				} else {
					opts.CreatedBefore = endDate
				}
			}
		} else {
			// Exact date format: "2023-01-01"
			exactDate, err := time.Parse("2006-01-02", created)
			if err == nil {
				opts.CreatedAfter = exactDate
				opts.CreatedBefore = exactDate.Add(24*time.Hour - time.Second)
			}
		}
	}

	return opts, nil
}

func ListRuns(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}

	opts, err := buildRunOptions(ctx, ownerID, repoID)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err)
		return
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
