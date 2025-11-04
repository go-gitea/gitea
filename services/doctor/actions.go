// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	repo_service "code.gitea.io/gitea/services/repository"

	"xorm.io/builder"
)

func disableMirrorActionsUnit(ctx context.Context, logger log.Logger, autofix bool) error {
	var reposToFix []*repo_model.Repository

	for page := 1; ; page++ {
		repos, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: repo_model.RepositoryListDefaultPageSize,
				Page:     page,
			},
			Mirror: optional.Some(true),
		})
		if err != nil {
			return fmt.Errorf("SearchRepository: %w", err)
		}
		if len(repos) == 0 {
			break
		}

		for _, repo := range repos {
			if repo.UnitEnabled(ctx, unit_model.TypeActions) {
				reposToFix = append(reposToFix, repo)
			}
		}
	}

	if len(reposToFix) == 0 {
		logger.Info("Found no mirror with actions unit enabled")
	} else {
		logger.Warn("Found %d mirrors with actions unit enabled", len(reposToFix))
	}
	if !autofix || len(reposToFix) == 0 {
		return nil
	}

	for _, repo := range reposToFix {
		if err := repo_service.UpdateRepositoryUnits(ctx, repo, nil, []unit_model.Type{unit_model.TypeActions}); err != nil {
			return err
		}
	}
	logger.Info("Fixed %d mirrors with actions unit enabled", len(reposToFix))

	return nil
}

func fixUnfinishedRunStatus(ctx context.Context, logger log.Logger, autofix bool) error {
	total := 0
	inconsistent := 0
	fixed := 0

	cond := builder.In("status", []actions_model.Status{
		actions_model.StatusWaiting,
		actions_model.StatusRunning,
		actions_model.StatusBlocked,
	}).And(builder.Lt{"updated": timeutil.TimeStampNow().AddDuration(-setting.Actions.ZombieTaskTimeout)})

	err := db.Iterate(
		ctx,
		cond,
		func(ctx context.Context, run *actions_model.ActionRun) error {
			total++

			jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
			if err != nil {
				return fmt.Errorf("GetRunJobsByRunID: %w", err)
			}
			expected := actions_model.AggregateJobStatus(jobs)
			if expected == run.Status {
				return nil
			}

			inconsistent++
			logger.Warn("Run %d (repo_id=%d, index=%d) has status %s, expected %s", run.ID, run.RepoID, run.Index, run.Status, expected)

			if !autofix {
				return nil
			}

			run.Started, run.Stopped = getRunTimestampsFromJobs(run, expected, jobs)
			run.Status = expected

			if err := actions_model.UpdateRun(ctx, run, "status", "started", "stopped"); err != nil {
				return fmt.Errorf("UpdateRun: %w", err)
			}
			fixed++

			return nil
		},
	)
	if err != nil {
		logger.Critical("Unable to iterate unfinished runs: %v", err)
		return err
	}

	if inconsistent == 0 {
		logger.Info("Checked %d unfinished runs; all statuses are consistent.", total)
		return nil
	}

	if autofix {
		logger.Info("Checked %d unfinished runs; fixed %d of %d runs.", total, fixed, inconsistent)
	} else {
		logger.Warn("Checked %d unfinished runs; found %d runs need to be fixed", total, inconsistent)
	}

	return nil
}

func getRunTimestampsFromJobs(run *actions_model.ActionRun, newStatus actions_model.Status, jobs actions_model.ActionJobList) (started, stopped timeutil.TimeStamp) {
	started = run.Started
	if (newStatus.IsRunning() || newStatus.IsDone()) && started.IsZero() {
		var earliest timeutil.TimeStamp
		for _, job := range jobs {
			if job.Started > 0 && (earliest.IsZero() || job.Started < earliest) {
				earliest = job.Started
			}
		}
		started = earliest
	}

	stopped = run.Stopped
	if newStatus.IsDone() && stopped.IsZero() {
		var latest timeutil.TimeStamp
		for _, job := range jobs {
			if job.Stopped > latest {
				latest = job.Stopped
			}
		}
		stopped = latest
	}

	return started, stopped
}

func init() {
	Register(&Check{
		Title:     "Disable the actions unit for all mirrors",
		Name:      "disable-mirror-actions-unit",
		IsDefault: false,
		Run:       disableMirrorActionsUnit,
		Priority:  9,
	})
	Register(&Check{
		Title:     "Fix inconsistent status for unfinished actions runs",
		Name:      "fix-actions-unfinished-run-status",
		IsDefault: false,
		Run:       fixUnfinishedRunStatus,
		Priority:  9,
	})
}
