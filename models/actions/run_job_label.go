// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

// ActionRunJobLabel is a normalized projection of ActionRunJob.RunsOn, one row
// per label. It exists so runner assignment can filter waiting jobs by label in
// SQL (runs_on is a JSON column that can't be matched portably across DBs).
// A job with no rows here has an empty runs_on and matches any runner.
type ActionRunJobLabel struct {
	ID    int64  `xorm:"pk autoincr"`
	JobID int64  `xorm:"UNIQUE(job_label) NOT NULL"`
	Label string `xorm:"UNIQUE(job_label) INDEX VARCHAR(255) NOT NULL"`
}

func init() {
	db.RegisterModel(new(ActionRunJobLabel))
}

// InsertActionRunJob inserts a job together with its runs_on label rows, keeping
// the action_run_job_label projection in sync. Every job-insert site must use this
// so a job is never persisted without its labels (which would make it match any
// runner). Must run inside the job's insert transaction.
func InsertActionRunJob(ctx context.Context, job *ActionRunJob) error {
	if err := db.Insert(ctx, job); err != nil {
		return err
	}
	return InsertActionRunJobLabels(ctx, job.ID, job.RunsOn)
}

// InsertActionRunJobs inserts jobs together with their runs_on label rows. Same
// contract as InsertActionRunJob, for the batch insert sites. The jobs are inserted
// one by one rather than as a single multi-row statement, which does not assign the
// ids back and so leaves nothing to key the label rows on.
func InsertActionRunJobs(ctx context.Context, jobs []*ActionRunJob) error {
	for _, job := range jobs {
		if err := InsertActionRunJob(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

// InsertActionRunJobLabels persists the runs_on labels of a freshly inserted job
// so it becomes matchable by the SQL assignment query. It must be called in the
// same transaction as the job insert.
func InsertActionRunJobLabels(ctx context.Context, jobID int64, runsOn []string) error {
	// FilterSlice drops empty labels and deduplicates by label, so the UNIQUE(job_label) constraint holds.
	labels := container.FilterSlice(runsOn, func(label string) (ActionRunJobLabel, bool) {
		return ActionRunJobLabel{JobID: jobID, Label: label}, label != ""
	})
	if len(labels) == 0 {
		return nil
	}
	return db.Insert(ctx, &labels)
}

// ReplaceActionRunJobLabels re-syncs a job's label rows after its runs_on changed.
// Deferred-matrix expansion rewrites runs_on in place (the placeholder is inserted
// with the raw, unevaluated matrix labels), so its stale rows must be replaced or
// the job would be matched against labels it no longer requires. It must run in the
// same transaction as the runs_on update.
func ReplaceActionRunJobLabels(ctx context.Context, jobID int64, runsOn []string) error {
	if _, err := db.GetEngine(ctx).Where(builder.Eq{"job_id": jobID}).Delete(new(ActionRunJobLabel)); err != nil {
		return err
	}
	return InsertActionRunJobLabels(ctx, jobID, runsOn)
}

// DeleteActionRunJobLabelsByRunID removes label rows for all jobs of a run.
// It must run before the jobs themselves are deleted so the subquery can resolve.
func DeleteActionRunJobLabelsByRunID(ctx context.Context, repoID, runID int64) error {
	return deleteActionRunJobLabels(ctx, repoID, runID)
}

// DeleteActionRunJobLabelsByRepoID removes label rows for every job of a repo.
// Used on repo deletion, which deletes the jobs directly by repo_id and would
// otherwise orphan their label rows. It must run before the jobs themselves are
// deleted so the subquery can resolve.
func DeleteActionRunJobLabelsByRepoID(ctx context.Context, repoID int64) error {
	return deleteActionRunJobLabels(ctx, repoID, 0)
}

func deleteActionRunJobLabels(ctx context.Context, repoID, runID int64) error {
	jobWhere := builder.Cond(builder.Eq{"repo_id": repoID})
	if runID != 0 {
		jobWhere = jobWhere.And(builder.Eq{"run_id": runID})
	}
	return deleteActionRunJobLabelsByJobCond(ctx, jobWhere)
}

// deleteActionRunJobLabelsByJobCond removes the label rows of every job matching
// jobCond. It must run before those jobs are deleted so the subquery can resolve.
func deleteActionRunJobLabelsByJobCond(ctx context.Context, jobCond builder.Cond) error {
	_, err := db.GetEngine(ctx).Where(
		builder.In("job_id", builder.Select("id").From("action_run_job").Where(jobCond)),
	).Delete(new(ActionRunJobLabel))
	return err
}

// runnerMatchableJobCond returns a condition selecting jobs the given runner
// labels can run: jobs with no required label outside the runner's label set.
// A runner without labels matches only jobs that require no label.
//
// Label comparison happens in SQL, so its case sensitivity follows the label
// column's collation. This matches the case-sensitive Go comparison it replaces
// (ActionRunner.CanMatchLabels) only on a case-sensitive collation, which is the
// collation Gitea converges DBs to; a case-insensitive collation would match
// labels that differ only in case.
func runnerMatchableJobCond(runnerLabels []string) builder.Cond {
	sub := builder.Expr("action_run_job_label.job_id = action_run_job.id")
	if len(runnerLabels) > 0 {
		sub = sub.And(builder.NotIn("action_run_job_label.label", runnerLabels))
	}
	return builder.NotExists(builder.Select("1").From("action_run_job_label").Where(sub))
}
