// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
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
	queuePageSize = 50 // queued jobs per page

	// runningJobsLimit caps the running-job list, which is bounded by the number of busy runners.
	runningJobsLimit = 100

	// status filter values, as submitted by the filter bar ("" means every listed status)
	queueFilterRunning = "running"
	queueFilterWaiting = "waiting"
)

// RenderQueue queries and renders a build-queue view (running jobs followed by queued jobs in pickup order):
// a single repository when repoID > 0, otherwise the whole instance. It serves both the initial full page
// (fullTemplate) and the in-place auto-refresh fragment. Both lists can be narrowed by status and, outside
// a repo scope, by owner and repository.
func RenderQueue(ctx *context.Context, repoID int64, fullTemplate templates.TplName) {
	page := max(ctx.FormInt("page"), 1)

	filterStatus := ctx.FormString("status")
	if filterStatus != queueFilterRunning && filterStatus != queueFilterWaiting {
		filterStatus = ""
	}
	isRefresh := ctx.FormBool("refresh")
	// A repo queue is already a single repository, so it offers no owner/repository filter.
	var filterOwnerID, filterRepoID int64
	if repoID == 0 {
		if isRefresh {
			// The filter bar sits outside the auto-refresh morph (see initActionQueueList), so its dropdown
			// options are never seen on a refresh; skip building them and just re-apply the scope the full
			// page already validated and put in the refresh link's query string.
			filterOwnerID, filterRepoID = ctx.FormInt64("owner_id"), ctx.FormInt64("repo_id")
		} else {
			var err error
			if filterOwnerID, filterRepoID, err = renderQueueFilterOptions(ctx); err != nil {
				ctx.ServerError("renderQueueFilterOptions", err)
				return
			}
		}
	}
	ctx.Data["QueueFilterOwnerID"], ctx.Data["QueueFilterRepoID"] = filterOwnerID, filterRepoID
	queueOpts := actions_model.QueueJobsOptions{
		RepoID:  util.Iif(filterRepoID > 0, filterRepoID, repoID),
		OwnerID: filterOwnerID,
	}
	filtered := filterOwnerID > 0 || filterRepoID > 0

	var queuedJobs []*actions_model.ActionRunJob
	var queuedTotal int64
	if filterStatus != queueFilterRunning {
		var err error
		queuedJobs, queuedTotal, err = actions_model.FindQueuedJobs(ctx, queueOpts, page, queuePageSize)
		if err != nil {
			ctx.ServerError("FindQueuedJobs", err)
			return
		}
		if err := actions_model.ActionJobList(queuedJobs).LoadAttributes(ctx, true); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return
		}
	}

	// Running jobs are bounded by the number of online runners, so a single capped page is enough:
	// they head the list on every page instead of taking part in the queued-job pagination.
	var runningJobs []*actions_model.ActionRunJob
	if filterStatus != queueFilterWaiting {
		var err error
		runningJobs, err = actions_model.FindRunningJobs(ctx, queueOpts, runningJobsLimit)
		if err != nil {
			ctx.ServerError("FindRunningJobs", err)
			return
		}
		if err := actions_model.ActionJobList(runningJobs).LoadAttributes(ctx, true); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return
		}
	}

	runners, err := runningJobRunnerNames(ctx, runningJobs)
	if err != nil {
		ctx.ServerError("runningJobRunnerNames", err)
		return
	}
	ctx.Data["RunningJobRunners"] = runners

	ctx.Data["QueuedJobs"] = queuedJobs
	ctx.Data["QueueOffset"] = (page - 1) * queuePageSize // absolute position of the first row on this page
	ctx.Data["RunningJobs"] = runningJobs
	ctx.Data["QueueTotal"] = queuedTotal + int64(len(runningJobs))
	ctx.Data["ShowRepoColumn"] = repoID == 0
	ctx.Data["ShowOwnerRepoFilters"] = repoID == 0
	ctx.Data["QueueFilterStatus"] = filterStatus
	ctx.Data["QueueFilterStatuses"] = []string{queueFilterRunning, queueFilterWaiting}
	// Positions are absolute pickup positions, which an owner/repository filter would silently misnumber.
	ctx.Data["ShowQueuePositions"] = !filtered

	pager := context.NewPagerBuilder(ctx).TotalCount(queuedTotal).PerPageLimit(queuePageSize).CurPage(page).Build()
	pager.RemoveParam(container.SetOf("refresh")) // keep the auto-refresh flag out of the page links
	ctx.Data["Page"] = pager

	ctx.Data["QueueRefreshIntervalMs"] = RefreshIntervalMs(len(queuedJobs) > 0 || len(runningJobs) > 0)
	ctx.Data["QueueRefreshLink"] = templates.QueryBuild(setting.AppSubURL+ctx.Req.RequestURI, "refresh", "1")

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

// renderQueueFilterOptions fills the owner/repository filter dropdowns with the repositories that
// currently have queued or running jobs, and resolves the requested filters against them. Ids that match
// nothing on offer are dropped, so a stale link cannot leave the view stuck on an empty filter.
func renderQueueFilterOptions(ctx *context.Context) (filterOwnerID, filterRepoID int64, _ error) {
	repoIDs, err := actions_model.QueueFilterRepoIDs(ctx, actions_model.QueueJobsOptions{}, queueFilterOptionsLimit)
	if err != nil {
		return 0, 0, err
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

	if reqOwnerID := ctx.FormInt64("owner_id"); reqOwnerID > 0 {
		for _, owner := range owners {
			if owner.ID == reqOwnerID {
				filterOwnerID = owner.ID
				ctx.Data["QueueFilterOwnerName"] = owner.Name
				break
			}
		}
	}
	if reqRepoID := ctx.FormInt64("repo_id"); reqRepoID > 0 {
		if repo := repoMap[reqRepoID]; repo != nil {
			filterRepoID = repo.ID
			ctx.Data["QueueFilterRepoName"] = repo.FullName()
			filterOwnerID = 0 // a repository is the narrower filter of the two
			ctx.Data["QueueFilterOwnerName"] = nil
		}
	}

	// The repository dropdown only lists the selected owner's repositories, mirroring the selection made.
	if filterOwnerID > 0 {
		repos = slices.DeleteFunc(repos, func(repo *repo_model.Repository) bool { return repo.OwnerID != filterOwnerID })
	}
	ctx.Data["QueueFilterOwners"] = owners
	ctx.Data["QueueFilterRepos"] = repos
	return filterOwnerID, filterRepoID, nil
}

// runningJobRunnerNames maps each running job's ID to the name of the runner executing it.
func runningJobRunnerNames(ctx *context.Context, jobs []*actions_model.ActionRunJob) (map[int64]string, error) {
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

	tasks, err := actions_model.GetTasksMapByIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	runnerIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		if t.RunnerID > 0 {
			runnerIDs = append(runnerIDs, t.RunnerID)
		}
	}
	if len(runnerIDs) == 0 {
		return names, nil
	}
	runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{
		IDs:         runnerIDs,
		ListOptions: db.ListOptionsAll,
	})
	if err != nil {
		return nil, err
	}
	runnerNames := make(map[int64]string, len(runners))
	for _, r := range runners {
		runnerNames[r.ID] = r.Name
	}

	for _, j := range jobs {
		if t := tasks[j.EffectiveTaskID()]; t != nil {
			if name, ok := runnerNames[t.RunnerID]; ok {
				names[j.ID] = name
			}
		}
	}
	return names, nil
}
