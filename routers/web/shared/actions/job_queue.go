// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

// JobQueue renders the instance-wide Actions job queue on the admin settings page.
func JobQueue(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsActionsJobQueue"] = true
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "job_queue"

	RenderJobQueue(ctx, 0, "admin/actions")
}

const jobQueuePageSize = 50

// RefreshIntervalMs is how often an auto-refreshing Actions list re-fetches itself.
func RefreshIntervalMs(hasActivity bool) int64 {
	if !setting.IsProd {
		return util.Iif[int64](hasActivity, 1000, 2*1000)
	}
	return util.Iif[int64](hasActivity, 3*1000, 12*1000)
}

// RenderJobQueue renders the job queue of one repository, or of the instance when repoID is 0.
func RenderJobQueue(ctx *context.Context, repoID int64, fullTemplate templates.TplName) {
	page := max(ctx.FormInt("page"), 1)

	filterStatus := ctx.FormString("status")
	status := actions_model.StatusUnknown
	switch filterStatus {
	case actions_model.StatusRunning.String():
		status = actions_model.StatusRunning
	case actions_model.StatusWaiting.String():
		status = actions_model.StatusWaiting
	default:
		filterStatus = ""
	}
	var filterOwnerID, filterRepoID int64
	if repoID == 0 {
		var err error
		if filterOwnerID, filterRepoID, err = renderJobQueueFilterOptions(ctx); err != nil {
			ctx.ServerError("renderJobQueueFilterOptions", err)
			return
		}
	}
	ctx.Data["JobQueueFilterOwnerID"], ctx.Data["JobQueueFilterRepoID"] = filterOwnerID, filterRepoID

	jobs, total, err := actions_model.FindJobQueueJobs(ctx, actions_model.JobQueueOptions{
		RepoID:  util.Iif(filterRepoID > 0, filterRepoID, repoID),
		OwnerID: filterOwnerID,
		Status:  status,
	}, page, jobQueuePageSize)
	if err != nil {
		ctx.ServerError("FindJobQueueJobs", err)
		return
	}
	if err := actions_model.ActionJobList(jobs).LoadAttributes(ctx, true); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	runners, err := actions_model.GetTaskRunnerNames(ctx, container.FilterSlice(jobs, func(job *actions_model.ActionRunJob) (int64, bool) {
		return job.TaskID, job.TaskID > 0
	}))
	if err != nil {
		ctx.ServerError("GetTaskRunnerNames", err)
		return
	}

	ctx.Data["JobQueueJobs"] = jobs
	ctx.Data["JobQueueRunners"] = runners
	ctx.Data["JobQueueTotal"] = total
	if !setting.IsProd && !ctx.FormBool("refresh") {
		// for dev mode, force the first screen to be blank to debug more edge cases
		ctx.Data["JobQueueJobs"], ctx.Data["JobQueueRunners"], ctx.Data["JobQueueTotal"] = nil, nil, 0
	}
	ctx.Data["ShowRepoColumn"] = repoID == 0
	ctx.Data["JobQueueFilterStatus"] = filterStatus
	ctx.Data["JobQueueFilterStatuses"] = []string{actions_model.StatusRunning.String(), actions_model.StatusWaiting.String()}

	pager := context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(jobQueuePageSize).CurPage(page).Build()
	query := url.Values{}
	if filterOwnerID > 0 {
		query.Set("owner_id", strconv.FormatInt(filterOwnerID, 10))
	}
	if filterRepoID > 0 {
		query.Set("repo_id", strconv.FormatInt(filterRepoID, 10))
	}
	if filterStatus != "" {
		query.Set("status", filterStatus)
	}
	pager.RemoveParam(container.SetOf("refresh", "owner_id", "repo_id", "status"))
	pager.AddParamFromQuery(query)
	ctx.Data["Page"] = pager

	ctx.Data["JobQueueRefreshIntervalMs"] = RefreshIntervalMs(len(jobs) > 0)
	query.Set("page", strconv.Itoa(pager.Paginator.Current()))
	query.Set("refresh", "1")
	ctx.Data["JobQueueRefreshLink"] = setting.AppSubURL + ctx.Req.URL.EscapedPath() + "?" + query.Encode()

	if ctx.FormBool("refresh") {
		ctx.HTML(http.StatusOK, "shared/actions/job_queue_list")
		return
	}
	ctx.HTML(http.StatusOK, fullTemplate)
}

// JobQueueFilterOwner is one entry of the job queue's owner filter.
type JobQueueFilterOwner struct {
	ID   int64
	Name string
}

// renderJobQueueFilterOptions includes pending work and the selected scope, even when its queue is empty.
func renderJobQueueFilterOptions(ctx *context.Context) (filterOwnerID, filterRepoID int64, _ error) {
	repoIDs, err := actions_model.JobQueueFilterRepoIDs(ctx, 200)
	if err != nil {
		return 0, 0, err
	}
	reqRepoID := ctx.FormInt64("repo_id")
	if reqRepoID > 0 {
		repoIDs = append(repoIDs, reqRepoID)
	}
	repoMap, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return 0, 0, err
	}

	repos := slices.Collect(maps.Values(repoMap))
	slices.SortFunc(repos, func(a, b *repo_model.Repository) int {
		return base.NaturalSortCompare(a.FullName(), b.FullName())
	})

	owners := make([]*JobQueueFilterOwner, 0, len(repos))
	seenOwners := make(container.Set[int64], len(repos))
	for _, repo := range repos {
		if seenOwners.Add(repo.OwnerID) {
			owners = append(owners, &JobQueueFilterOwner{ID: repo.OwnerID, Name: repo.OwnerName})
		}
	}

	if repo := repoMap[reqRepoID]; repo != nil {
		filterRepoID = repo.ID
		ctx.Data["JobQueueFilterRepoName"] = repo.FullName()
	} else if reqOwnerID := ctx.FormInt64("owner_id"); reqOwnerID > 0 {
		owner, err := user_model.GetUserByID(ctx, reqOwnerID)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return 0, 0, err
		}
		if owner != nil {
			filterOwnerID = owner.ID
			ctx.Data["JobQueueFilterOwnerName"] = owner.Name
			if seenOwners.Add(owner.ID) {
				owners = append(owners, &JobQueueFilterOwner{ID: owner.ID, Name: owner.Name})
			}
		}
	}
	slices.SortFunc(owners, func(a, b *JobQueueFilterOwner) int {
		return base.NaturalSortCompare(a.Name, b.Name)
	})

	if filterOwnerID > 0 {
		repos = slices.DeleteFunc(repos, func(repo *repo_model.Repository) bool { return repo.OwnerID != filterOwnerID })
	}
	ctx.Data["JobQueueFilterOwners"] = owners
	ctx.Data["JobQueueFilterRepos"] = repos
	return filterOwnerID, filterRepoID, nil
}
