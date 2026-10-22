// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"cmp"
	"context"
	"slices"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"

	"xorm.io/builder"
)

// RunnerRepoQueue is one repository a runner is allowed to pick jobs from,
// and how many waiting jobs that runner's labels can match there.
// The count is a shared queue: another runner with the same scope and labels
// can claim the same jobs.
type RunnerRepoQueue struct {
	Repo    *repo_model.Repository
	Waiting int
}

// ActionsLink is the repository actions page. The waiting count is not a
// filter on that page; it only says how much work is sitting there.
func (q RunnerRepoQueue) ActionsLink() string {
	if q.Repo == nil {
		return ""
	}
	return q.Repo.Link() + "/actions"
}

// ListRunnerRepoQueues lists repositories in runner's scope with the number of
// waiting jobs runner could pick. A repository or owner runner includes
// in-scope repositories that currently have none. A global runner would
// otherwise be every repository on the instance, so it only includes
// repositories that have at least one matching waiting job.
func ListRunnerRepoQueues(ctx context.Context, runner *ActionRunner) ([]RunnerRepoQueue, error) {
	global := runner.RepoID == 0 && runner.OwnerID == 0

	var repos []*repo_model.Repository
	var err error
	if !global {
		repos, err = listRunnerScopeRepos(ctx, runner)
		if err != nil {
			return nil, err
		}
		if len(repos) == 0 {
			return nil, nil
		}
	}

	counts, err := countPickableWaitingJobs(ctx, runner, repos)
	if err != nil {
		return nil, err
	}

	if global {
		if len(counts) == 0 {
			return nil, nil
		}
		ids := make([]int64, 0, len(counts))
		for id := range counts {
			ids = append(ids, id)
		}
		repos = nil
		if err := db.GetEngine(ctx).In("id", ids).Find(&repos); err != nil {
			return nil, err
		}
	}

	queues := make([]RunnerRepoQueue, 0, len(repos))
	for _, repo := range repos {
		waiting := counts[repo.ID]
		if global && waiting == 0 {
			continue
		}
		queues = append(queues, RunnerRepoQueue{Repo: repo, Waiting: waiting})
	}
	sortRunnerRepoQueues(queues)
	return queues, nil
}

func sortRunnerRepoQueues(queues []RunnerRepoQueue) {
	slices.SortFunc(queues, func(a, b RunnerRepoQueue) int {
		if c := cmp.Compare(b.Waiting, a.Waiting); c != 0 {
			return c
		}
		return cmp.Compare(a.Repo.FullName(), b.Repo.FullName())
	})
}

// WaitingQueuesByRunner returns, per runner, only repositories with a
// non-zero waiting count. Unlike ListRunnerRepoQueues it loads the waiting
// jobs and the in-scope repositories once for all runners, so a runners list
// page does not rescan the queue per runner.
func WaitingQueuesByRunner(ctx context.Context, runners []*ActionRunner) (map[int64][]RunnerRepoQueue, error) {
	byRunner := make(map[int64][]RunnerRepoQueue, len(runners))
	if len(runners) == 0 {
		return byRunner, nil
	}

	hasGlobal := false
	ownerIDs, repoIDs := []int64{}, []int64{}
	seenOwner, seenRepo := map[int64]bool{}, map[int64]bool{}
	for _, runner := range runners {
		switch {
		case runner.RepoID > 0:
			if !seenRepo[runner.RepoID] {
				seenRepo[runner.RepoID] = true
				repoIDs = append(repoIDs, runner.RepoID)
			}
		case runner.OwnerID > 0:
			if !seenOwner[runner.OwnerID] {
				seenOwner[runner.OwnerID] = true
				ownerIDs = append(ownerIDs, runner.OwnerID)
			}
		default:
			hasGlobal = true
		}
	}

	repoByID := map[int64]*repo_model.Repository{}
	if !hasGlobal {
		repos, err := listRunnerScopeReposForOwnersAndRepos(ctx, ownerIDs, repoIDs)
		if err != nil {
			return nil, err
		}
		if len(repos) == 0 {
			return byRunner, nil
		}
		for _, repo := range repos {
			repoByID[repo.ID] = repo
		}
	}

	var cond builder.Cond = builder.Eq{"task_id": 0, "status": StatusWaiting, "is_reusable_caller": false}
	if !hasGlobal {
		ids := make([]int64, 0, len(repoByID))
		for id := range repoByID {
			ids = append(ids, id)
		}
		cond = cond.And(builder.In("repo_id", ids))
	}
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where(cond).Cols("repo_id", "runs_on").Find(&jobs); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return byRunner, nil
	}

	if hasGlobal {
		ids, seen := []int64{}, map[int64]bool{}
		for _, job := range jobs {
			if !seen[job.RepoID] {
				seen[job.RepoID] = true
				ids = append(ids, job.RepoID)
			}
		}
		var repos []*repo_model.Repository
		if err := db.GetEngine(ctx).In("id", ids).Find(&repos); err != nil {
			return nil, err
		}
		for _, repo := range repos {
			repoByID[repo.ID] = repo
		}
	}

	for _, runner := range runners {
		counts := map[int64]int{}
		for _, job := range jobs {
			repo := repoByID[job.RepoID]
			if repo == nil || !runnerScopeIncludesRepo(runner, repo) {
				continue
			}
			if runner.CanMatchLabels(job.RunsOn) {
				counts[job.RepoID]++
			}
		}
		queues := make([]RunnerRepoQueue, 0, len(counts))
		for repoID, waiting := range counts {
			queues = append(queues, RunnerRepoQueue{Repo: repoByID[repoID], Waiting: waiting})
		}
		sortRunnerRepoQueues(queues)
		byRunner[runner.ID] = queues
	}
	return byRunner, nil
}

func runnerScopeIncludesRepo(runner *ActionRunner, repo *repo_model.Repository) bool {
	if runner.RepoID > 0 {
		return repo.ID == runner.RepoID
	}
	if runner.OwnerID > 0 {
		return repo.OwnerID == runner.OwnerID
	}
	return true
}

func listRunnerScopeReposForOwnersAndRepos(ctx context.Context, ownerIDs, repoIDs []int64) ([]*repo_model.Repository, error) {
	scope := builder.NewCond()
	if len(ownerIDs) > 0 {
		scope = scope.Or(builder.In("`repository`.owner_id", ownerIDs))
	}
	if len(repoIDs) > 0 {
		scope = scope.Or(builder.In("`repository`.id", repoIDs))
	}
	cond := builder.NewCond().And(builder.Eq{"`repo_unit`.type": unit.TypeActions}).And(scope)
	repoIDQuery := builder.Select("`repository`.id").From("repository").
		Join("INNER", "repo_unit", "`repository`.id = `repo_unit`.repo_id").
		Where(cond)

	var repos []*repo_model.Repository
	err := db.GetEngine(ctx).Where(builder.In("id", repoIDQuery)).Find(&repos)
	return repos, err
}

func listRunnerScopeRepos(ctx context.Context, runner *ActionRunner) ([]*repo_model.Repository, error) {
	// Same scope as CreateTaskForRunner: one repository, or every actions-enabled
	// repository owned by the runner's user or organization.
	cond := builder.NewCond().And(builder.Eq{"`repo_unit`.type": unit.TypeActions})
	if runner.RepoID != 0 {
		cond = cond.And(builder.Eq{"`repository`.id": runner.RepoID})
	} else {
		cond = cond.And(builder.Eq{"`repository`.owner_id": runner.OwnerID})
	}
	repoIDs := builder.Select("`repository`.id").From("repository").
		Join("INNER", "repo_unit", "`repository`.id = `repo_unit`.repo_id").
		Where(cond)

	var repos []*repo_model.Repository
	err := db.GetEngine(ctx).Where(builder.In("id", repoIDs)).Find(&repos)
	return repos, err
}

func countPickableWaitingJobs(ctx context.Context, runner *ActionRunner, repos []*repo_model.Repository) (map[int64]int, error) {
	var cond builder.Cond = builder.Eq{"task_id": 0, "status": StatusWaiting, "is_reusable_caller": false}
	if len(repos) > 0 {
		ids := make([]int64, len(repos))
		for i, repo := range repos {
			ids[i] = repo.ID
		}
		cond = cond.And(builder.In("repo_id", ids))
	}

	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where(cond).Cols("repo_id", "runs_on").Find(&jobs); err != nil {
		return nil, err
	}

	counts := make(map[int64]int)
	for _, job := range jobs {
		if runner.CanMatchLabels(job.RunsOn) {
			counts[job.RepoID]++
		}
	}
	return counts, nil
}

// FindRunningTasksByRunnerIDs returns tasks with status running, grouped by runner.
// An empty runnerIDs does not mean "every runner".
func FindRunningTasksByRunnerIDs(ctx context.Context, runnerIDs []int64) (map[int64][]*ActionTask, error) {
	found := make(map[int64][]*ActionTask, len(runnerIDs))
	if len(runnerIDs) == 0 {
		return found, nil
	}

	var tasks []*ActionTask
	if err := db.GetEngine(ctx).
		Where(builder.Eq{"status": StatusRunning}).
		In("runner_id", runnerIDs).
		Find(&tasks); err != nil {
		return nil, err
	}
	for _, task := range tasks {
		found[task.RunnerID] = append(found[task.RunnerID], task)
	}
	return found, nil
}
