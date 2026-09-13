// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
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

// Queue renders the instance-wide Actions build queue on the admin settings page.
func Queue(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsQueue"] = true
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "queue"

	RenderQueue(ctx, 0, "admin/actions")
}

const (
	queuePageSize = 50 // jobs per page

	// status filter values, as submitted by the filter bar ("" means every listed status)
	queueFilterRunning = "running"
	queueFilterWaiting = "waiting"
)

// RefreshIntervalMs is how often an auto-refreshing Actions list re-fetches itself.
func RefreshIntervalMs(hasActivity bool) int64 {
	if !setting.IsProd {
		return util.Iif[int64](hasActivity, 1000, 2*1000)
	}
	return util.Iif[int64](hasActivity, 3*1000, 12*1000)
}

// RenderQueue queries and renders a build-queue view (running jobs followed by the queued ones in pickup
// order): a single repository when repoID > 0, otherwise the whole instance. It serves both the initial
// full page (fullTemplate) and the in-place auto-refresh fragment. The list can be narrowed by status and,
// outside a repo scope, by owner and repository.
func RenderQueue(ctx *context.Context, repoID int64, fullTemplate templates.TplName) {
	page := max(ctx.FormInt("page"), 1)

	filterStatus := ctx.FormString("status")
	status := actions_model.StatusUnknown
	switch filterStatus {
	case queueFilterRunning:
		status = actions_model.StatusRunning
	case queueFilterWaiting:
		status = actions_model.StatusWaiting
	default:
		filterStatus = ""
	}
	isRefresh := ctx.FormBool("refresh")
	// A repo queue is already a single repository, so it offers no owner/repository filter.
	var filterOwnerID, filterRepoID int64
	if repoID == 0 {
		var err error
		if filterOwnerID, filterRepoID, err = renderQueueFilterOptions(ctx); err != nil {
			ctx.ServerError("renderQueueFilterOptions", err)
			return
		}
	}
	ctx.Data["QueueFilterOwnerID"], ctx.Data["QueueFilterRepoID"] = filterOwnerID, filterRepoID

	jobs, total, err := actions_model.FindQueueJobs(ctx, actions_model.QueueJobsOptions{
		RepoID:  util.Iif(filterRepoID > 0, filterRepoID, repoID),
		OwnerID: filterOwnerID,
		Status:  status,
	}, page, queuePageSize)
	if err != nil {
		ctx.ServerError("FindQueueJobs", err)
		return
	}
	if err := actions_model.ActionJobList(jobs).LoadAttributes(ctx, true); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	runners, err := jobRunnerNames(ctx, jobs)
	if err != nil {
		ctx.ServerError("jobRunnerNames", err)
		return
	}

	ctx.Data["QueueJobs"] = jobs
	ctx.Data["QueueJobRunners"] = runners
	ctx.Data["QueueTotal"] = total
	ctx.Data["ShowRepoColumn"] = repoID == 0
	ctx.Data["QueueFilterStatus"] = filterStatus
	ctx.Data["QueueFilterStatuses"] = []string{queueFilterRunning, queueFilterWaiting}

	pager := context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(queuePageSize).CurPage(page).Build()
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

	ctx.Data["QueueRefreshIntervalMs"] = RefreshIntervalMs(len(jobs) > 0)
	query.Set("page", strconv.Itoa(pager.Paginator.Current()))
	query.Set("refresh", "1")
	ctx.Data["QueueRefreshLink"] = setting.AppSubURL + ctx.Req.URL.EscapedPath() + "?" + query.Encode()

	if isRefresh {
		ctx.HTML(http.StatusOK, "shared/actions/queue_list")
		return
	}
	ctx.HTML(http.StatusOK, fullTemplate)
}

// QueueFilterOwner is one entry of the build queue's owner filter.
type QueueFilterOwner struct {
	ID   int64
	Name string
}

// queueFilterOptionsLimit caps how many repositories the filter dropdowns offer. The list only covers
// repositories with pending work, so the cap is far above any realistic queue.
const queueFilterOptionsLimit = 200

// renderQueueFilterOptions includes pending work and the selected scope, even when its queue is empty.
func renderQueueFilterOptions(ctx *context.Context) (filterOwnerID, filterRepoID int64, _ error) {
	repoIDs, err := actions_model.QueueFilterRepoIDs(ctx, actions_model.QueueJobsOptions{}, queueFilterOptionsLimit)
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

	repos := make([]*repo_model.Repository, 0, len(repoMap))
	for _, repo := range repoMap {
		if repo != nil {
			repos = append(repos, repo)
		}
	}
	slices.SortFunc(repos, func(a, b *repo_model.Repository) int {
		return base.NaturalSortCompare(a.FullName(), b.FullName())
	})

	owners := make([]*QueueFilterOwner, 0, len(repos))
	seenOwners := make(container.Set[int64], len(repos))
	for _, repo := range repos {
		if seenOwners.Add(repo.OwnerID) {
			owners = append(owners, &QueueFilterOwner{ID: repo.OwnerID, Name: repo.OwnerName})
		}
	}

	if repo := repoMap[reqRepoID]; repo != nil {
		filterRepoID = repo.ID
		ctx.Data["QueueFilterRepoName"] = repo.FullName()
	} else if reqOwnerID := ctx.FormInt64("owner_id"); reqOwnerID > 0 {
		owner, err := user_model.GetUserByID(ctx, reqOwnerID)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return 0, 0, err
		}
		if owner != nil {
			filterOwnerID = owner.ID
			ctx.Data["QueueFilterOwnerName"] = owner.Name
			if seenOwners.Add(owner.ID) {
				owners = append(owners, &QueueFilterOwner{ID: owner.ID, Name: owner.Name})
			}
		}
	}
	slices.SortFunc(owners, func(a, b *QueueFilterOwner) int {
		return base.NaturalSortCompare(a.Name, b.Name)
	})

	// The repository dropdown only lists the selected owner's repositories, mirroring the selection made.
	if filterOwnerID > 0 {
		repos = slices.DeleteFunc(repos, func(repo *repo_model.Repository) bool { return repo.OwnerID != filterOwnerID })
	}
	ctx.Data["QueueFilterOwners"] = owners
	ctx.Data["QueueFilterRepos"] = repos
	return filterOwnerID, filterRepoID, nil
}

// jobRunnerNames maps each running job's ID to the name of the runner executing it.
func jobRunnerNames(ctx *context.Context, jobs []*actions_model.ActionRunJob) (map[int64]string, error) {
	taskIDs := make([]int64, 0, len(jobs))
	for _, j := range jobs {
		if tid := j.EffectiveTaskID(); tid > 0 {
			taskIDs = append(taskIDs, tid)
		}
	}
	names := make(map[int64]string, len(jobs))
	if len(taskIDs) == 0 {
		return names, nil
	}

	runnerNames, err := actions_model.GetTaskRunnerNames(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if name, ok := runnerNames[job.EffectiveTaskID()]; ok {
			names[job.ID] = name
		}
	}
	return names, nil
}
