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

// QueueScope describes what a build-queue view should query and who may act on it.
type QueueScope struct {
	RepoID  int64 // >0: a single repo
	OwnerID int64 // >0: an org/user; both 0: the whole instance
	IsRepo  bool  // repo scope hides the (redundant) repository column

	CanReorder       bool // the viewer may drag-reorder queued jobs (further limited to the first page)
	ShowRunnerColumn bool // show which runner is executing each running job

	MoveLink     string            // POST target for reordering
	FullTemplate templates.TplName // full-page template for the initial (non-refresh) render
}

// Queue renders the instance-wide Actions build queue on the admin settings page.
func Queue(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsQueue"] = true
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "queue"

	RenderQueue(ctx, QueueScope{
		CanReorder:       true,
		ShowRunnerColumn: true,
		MoveLink:         setting.AppSubURL + "/-/admin/actions/queue/move",
		FullTemplate:     "admin/actions",
	})
}

// queue status filter values, as submitted by the filter bar ("" means every listed status).
const (
	queueFilterRunning = "running"
	queueFilterWaiting = "waiting"
)

// RenderQueue queries and renders a build-queue view (running jobs followed by queued jobs in pickup order)
// for the given scope. It serves both the initial full page (s.FullTemplate) and the in-place auto-refresh
// fragment. Both lists can be narrowed by status and, outside a repo scope, by repository.
func RenderQueue(ctx *context.Context, s QueueScope) {
	pageSize := actions_model.QueuePageSize
	page := max(ctx.FormInt("page"), 1)

	filterStatus := ctx.FormString("status")
	if filterStatus != queueFilterRunning && filterStatus != queueFilterWaiting {
		filterStatus = ""
	}
	// A repo queue is already a single repository, so it offers no owner/repository filter.
	var filterOwnerID, filterRepoID int64
	ctx.Data["QueueFilterOwnerID"], ctx.Data["QueueFilterRepoID"] = filterOwnerID, filterRepoID
	if !s.IsRepo {
		if err := renderQueueFilterOptions(ctx, s, &filterOwnerID, &filterRepoID); err != nil {
			ctx.ServerError("renderQueueFilterOptions", err)
			return
		}
	}
	scopeRepoID := util.Iif(filterRepoID > 0, filterRepoID, s.RepoID)
	scopeOwnerID := util.Iif(filterOwnerID > 0, filterOwnerID, s.OwnerID)
	filtered := filterOwnerID > 0 || filterRepoID > 0

	var queuedJobs []*actions_model.ActionRunJob
	var queuedTotal int64
	if filterStatus != queueFilterRunning {
		queuedOpts := actions_model.QueuedJobsOptions(scopeRepoID, scopeOwnerID)
		queuedOpts.ListOptions = db.ListOptions{Page: page, PageSize: pageSize}
		var err error
		queuedJobs, queuedTotal, err = db.FindAndCount[actions_model.ActionRunJob](ctx, queuedOpts)
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
		runningOpts := actions_model.FindRunJobOptions{
			RepoID:      scopeRepoID,
			OwnerID:     scopeOwnerID,
			ListOptions: db.ListOptions{Page: 1, PageSize: 100},
			Statuses:    []actions_model.Status{actions_model.StatusRunning},
			OrderBy:     actions_model.RunningJobsOrderBy,
		}
		var err error
		runningJobs, err = db.Find[actions_model.ActionRunJob](ctx, runningOpts)
		if err != nil {
			ctx.ServerError("FindRunningJobs", err)
			return
		}
		if err := actions_model.ActionJobList(runningJobs).LoadAttributes(ctx, true); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return
		}
	}

	// Show which runner is executing each running job (admin "more info").
	ctx.Data["ShowRunnerColumn"] = s.ShowRunnerColumn
	if s.ShowRunnerColumn {
		runners, err := runningJobRunnerNames(ctx, runningJobs)
		if err != nil {
			ctx.ServerError("runningJobRunnerNames", err)
			return
		}
		ctx.Data["RunningJobRunners"] = runners
	}

	ctx.Data["QueuedJobs"] = queuedJobs
	ctx.Data["QueuedTotal"] = queuedTotal
	ctx.Data["QueueOffset"] = (page - 1) * pageSize // absolute position of the first row on this page
	ctx.Data["RunningJobs"] = runningJobs
	ctx.Data["QueueTotal"] = queuedTotal + int64(len(runningJobs))
	ctx.Data["ShowRepoColumn"] = !s.IsRepo
	ctx.Data["ShowOwnerRepoFilters"] = !s.IsRepo
	ctx.Data["QueueFilterStatus"] = filterStatus
	ctx.Data["QueueFilterStatuses"] = []string{queueFilterRunning, queueFilterWaiting}
	// Positions are absolute pickup positions, which an owner/repository filter would silently misnumber.
	ctx.Data["ShowQueuePositions"] = !filtered
	// Reordering renumbers the first page only (the head runners pick from), so gate the handles to page 1.
	// An owner/repository filter also hides them: the dropped row's neighbours may not be the real ones.
	ctx.Data["CanReorder"] = s.CanReorder && page == 1 && !filtered
	ctx.Data["QueueMoveLink"] = s.MoveLink

	pager := context.NewPagination(queuedTotal, pageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	pager.RemoveParam(container.SetOf("refresh")) // keep the auto-refresh flag out of the page links
	ctx.Data["Page"] = pager

	// The list auto-refreshes by re-fetching just the fragment: poll faster while there is activity,
	// slower when idle so quiet pages don't hammer the server.
	hasActivity := len(queuedJobs) > 0 || len(runningJobs) > 0
	refreshInterval := util.Iif[int64](hasActivity, 3000, 10000)
	if !setting.IsProd {
		refreshInterval = util.Iif[int64](hasActivity, 1000, 2000) // faster in dev for easier debugging
	}
	ctx.Data["QueueRefreshIntervalMs"] = refreshInterval
	ctx.Data["QueueRefreshLink"] = templates.QueryBuild(setting.AppSubURL+ctx.Req.RequestURI, "refresh", "1")

	if ctx.FormBool("refresh") {
		ctx.HTML(http.StatusOK, "shared/actions/queue_list")
		return
	}
	ctx.HTML(http.StatusOK, s.FullTemplate)
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
func renderQueueFilterOptions(ctx *context.Context, s QueueScope, filterOwnerID, filterRepoID *int64) error {
	repoIDs, err := actions_model.QueueFilterRepoIDs(ctx, s.RepoID, s.OwnerID, queueFilterOptionsLimit)
	if err != nil {
		return err
	}
	repoMap, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
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
	for _, repo := range repos {
		if !slices.ContainsFunc(owners, func(o *QueueFilterOwner) bool { return o.ID == repo.OwnerID }) {
			owners = append(owners, &QueueFilterOwner{ID: repo.OwnerID, Name: repo.OwnerName})
		}
	}

	if reqOwnerID := ctx.FormInt64("owner_id"); reqOwnerID > 0 {
		for _, owner := range owners {
			if owner.ID == reqOwnerID {
				*filterOwnerID = owner.ID
				ctx.Data["QueueFilterOwnerName"] = owner.Name
				break
			}
		}
	}
	if reqRepoID := ctx.FormInt64("repo_id"); reqRepoID > 0 {
		if repo := repoMap[reqRepoID]; repo != nil {
			*filterRepoID = repo.ID
			ctx.Data["QueueFilterRepoName"] = repo.FullName()
			*filterOwnerID = 0 // a repository is the narrower filter of the two
			ctx.Data["QueueFilterOwnerName"] = nil
		}
	}

	// The repository dropdown only lists the selected owner's repositories, mirroring the selection made.
	if *filterOwnerID > 0 {
		repos = slices.DeleteFunc(repos, func(repo *repo_model.Repository) bool { return repo.OwnerID != *filterOwnerID })
	}
	ctx.Data["QueueFilterOwners"] = owners
	ctx.Data["QueueFilterRepos"] = repos
	ctx.Data["QueueFilterOwnerID"] = *filterOwnerID
	ctx.Data["QueueFilterRepoID"] = *filterRepoID
	return nil
}

// runningJobRunnerNames maps each running job's ID to the name of the runner executing it.
func runningJobRunnerNames(ctx *context.Context, jobs []*actions_model.ActionRunJob) (map[int64]string, error) {
	taskIDByJob := make(map[int64]int64, len(jobs))
	taskIDs := make([]int64, 0, len(jobs))
	for _, j := range jobs {
		if tid := j.EffectiveTaskID(); tid > 0 {
			taskIDByJob[j.ID] = tid
			taskIDs = append(taskIDs, tid)
		}
	}
	if len(taskIDs) == 0 {
		return map[int64]string{}, nil
	}

	tasks := make(map[int64]*actions_model.ActionTask, len(taskIDs))
	if err := db.GetEngine(ctx).In("id", taskIDs).Find(&tasks); err != nil {
		return nil, err
	}
	runnerIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		if t.RunnerID > 0 {
			runnerIDs = append(runnerIDs, t.RunnerID)
		}
	}
	runners := make(map[int64]*actions_model.ActionRunner, len(runnerIDs))
	if len(runnerIDs) > 0 {
		if err := db.GetEngine(ctx).In("id", runnerIDs).Find(&runners); err != nil {
			return nil, err
		}
	}

	names := make(map[int64]string, len(jobs))
	for jobID, taskID := range taskIDByJob {
		if t := tasks[taskID]; t != nil {
			if r := runners[t.RunnerID]; r != nil {
				names[jobID] = r.Name
			}
		}
	}
	return names, nil
}

// QueueMovePost reorders a queued job on the admin queue settings page (site-admin gated by the route group).
func QueueMovePost(ctx *context.Context) {
	HandleQueueMove(ctx, 0, 0)
}

// HandleQueueMove applies a drag-and-drop reorder for the given queue scope and writes the HTTP response.
// Callers are responsible for permission checks (only admins of the scope may reorder).
func HandleQueueMove(ctx *context.Context, repoID, ownerID int64) {
	movedID := ctx.FormInt64("id")
	if movedID == 0 {
		ctx.HTTPError(http.StatusBadRequest, "missing job id")
		return
	}
	ok, err := actions_model.MoveQueuedJob(ctx, repoID, ownerID, movedID, ctx.FormInt64("after"))
	if err != nil {
		ctx.ServerError("MoveQueuedJob", err)
		return
	}
	if !ok {
		// The client's view is stale (a job left the queue); the auto-refresh will re-render it.
		ctx.JSON(http.StatusConflict, map[string]any{"needsRefresh": true})
		return
	}
	ctx.Status(http.StatusNoContent)
}
