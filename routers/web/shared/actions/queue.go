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

// Queue renders the instance-wide Actions build queue on the admin settings page.
func Queue(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsQueue"] = true
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = "queue"

	RenderQueue(ctx, 0, "admin/actions")
}

const queuePageSize = 50

// RefreshIntervalMs is how often an auto-refreshing Actions list re-fetches itself.
func RefreshIntervalMs(hasActivity bool) int64 {
	if !setting.IsProd {
		return util.Iif[int64](hasActivity, 1000, 2*1000)
	}
	return util.Iif[int64](hasActivity, 3*1000, 12*1000)
}

// RenderQueue renders the build queue of one repository, or of the instance when repoID is 0.
func RenderQueue(ctx *context.Context, repoID int64, fullTemplate templates.TplName) {
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

	runners, err := actions_model.GetTaskRunnerNames(ctx, container.FilterSlice(jobs, func(job *actions_model.ActionRunJob) (int64, bool) {
		return job.TaskID, job.TaskID > 0
	}))
	if err != nil {
		ctx.ServerError("GetTaskRunnerNames", err)
		return
	}

	ctx.Data["QueueJobs"] = jobs
	ctx.Data["QueueJobRunners"] = runners
	ctx.Data["QueueTotal"] = total
	ctx.Data["ShowRepoColumn"] = repoID == 0
	ctx.Data["QueueFilterStatus"] = filterStatus
	ctx.Data["QueueFilterStatuses"] = []string{actions_model.StatusRunning.String(), actions_model.StatusWaiting.String()}

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

	if ctx.FormBool("refresh") {
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

// renderQueueFilterOptions includes pending work and the selected scope, even when its queue is empty.
func renderQueueFilterOptions(ctx *context.Context) (filterOwnerID, filterRepoID int64, _ error) {
	repoIDs, err := actions_model.QueueFilterRepoIDs(ctx, 200)
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

	if filterOwnerID > 0 {
		repos = slices.DeleteFunc(repos, func(repo *repo_model.Repository) bool { return repo.OwnerID != filterOwnerID })
	}
	ctx.Data["QueueFilterOwners"] = owners
	ctx.Data["QueueFilterRepos"] = repos
	return filterOwnerID, filterRepoID, nil
}
