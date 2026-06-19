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

// InsertActionRunJobLabels persists the runs_on labels of a freshly inserted job
// so it becomes matchable by the SQL assignment query. It must be called in the
// same transaction as the job insert. runs_on is immutable after creation, so
// labels never need updating afterwards.
func InsertActionRunJobLabels(ctx context.Context, jobID int64, runsOn []string) error {
	if len(runsOn) == 0 {
		return nil
	}
	seen := make(container.Set[string], len(runsOn))
	labels := make([]ActionRunJobLabel, 0, len(runsOn))
	for _, label := range runsOn {
		if label == "" || !seen.Add(label) {
			continue
		}
		labels = append(labels, ActionRunJobLabel{JobID: jobID, Label: label})
	}
	if len(labels) == 0 {
		return nil
	}
	return db.Insert(ctx, &labels)
}

// DeleteActionRunJobLabelsByRunID removes label rows for all jobs of a run.
// It must run before the jobs themselves are deleted so the subquery can resolve.
func DeleteActionRunJobLabelsByRunID(ctx context.Context, repoID, runID int64) error {
	_, err := db.GetEngine(ctx).Where(
		builder.In("job_id", builder.Select("id").From("action_run_job").
			Where(builder.Eq{"repo_id": repoID, "run_id": runID})),
	).Delete(new(ActionRunJobLabel))
	return err
}

// runnerMatchableJobCond returns a condition selecting jobs the given runner
// labels can run: jobs with no required label outside the runner's label set.
// A runner without labels matches only jobs that require no label.
func runnerMatchableJobCond(runnerLabels []string) builder.Cond {
	sub := builder.Expr("action_run_job_label.job_id = action_run_job.id")
	if len(runnerLabels) > 0 {
		sub = sub.And(builder.NotIn("action_run_job_label.label", runnerLabels))
	}
	return builder.NotExists(builder.Select("1").From("action_run_job_label").Where(sub))
}
