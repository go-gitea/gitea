// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

const (
	// JobSummaryCapability is the runner-declare capability string for job summaries.
	JobSummaryCapability = "job-summary"

	// JobSummaryContentTypeMarkdown is the only accepted content type for job summaries.
	JobSummaryContentTypeMarkdown = "text/markdown"

	// MaxJobSummarySize is the maximum accepted per-step summary payload size in bytes.
	MaxJobSummarySize = 1024 * 1024 // 1 MiB

	// MaxJobSummaryAggregateSize is the maximum aggregate size of all step summaries within
	// a single job attempt. Matches GitHub's documented per-job summary cap of 1 MiB.
	MaxJobSummaryAggregateSize = 1024 * 1024 // 1 MiB
)

// RunnerCapabilities returns the value advertised in the X-Gitea-Actions-Capabilities header.
// When more capabilities are added, return them comma-separated so runners can split on ", ".
func RunnerCapabilities() string {
	return JobSummaryCapability
}

type ActionRunJobSummary struct {
	ID int64 `xorm:"pk autoincr"`

	RepoID       int64 `xorm:"UNIQUE(summary_key)"`
	RunID        int64 `xorm:"UNIQUE(summary_key)"`
	RunAttemptID int64 `xorm:"UNIQUE(summary_key) NOT NULL DEFAULT 0"`
	JobID        int64 `xorm:"UNIQUE(summary_key)"`
	StepIndex    int64 `xorm:"UNIQUE(summary_key)"`

	Content     string `xorm:"LONGTEXT"`
	ContentType string `xorm:"VARCHAR(255) NOT NULL DEFAULT 'text/markdown'"`
	// ContentSize is the byte length of Content. Stored explicitly because LENGTH()
	// counts characters (not bytes) on PostgreSQL, SQLite and MSSQL, which would let
	// multibyte UTF-8 content bypass the aggregate cap.
	ContentSize int64 `xorm:"NOT NULL DEFAULT 0"`

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionRunJobSummary))
}

func GetActionRunJobSummary(ctx context.Context, repoID, runID, runAttemptID, jobID, stepIndex int64) (*ActionRunJobSummary, error) {
	var s ActionRunJobSummary
	has, err := db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=? AND job_id=? AND step_index=?", repoID, runID, runAttemptID, jobID, stepIndex).
		Get(&s)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return &s, nil
}

// ErrJobSummaryAggregateExceeded is returned when a step summary upload would push the
// aggregate size of summaries for a single job attempt over MaxJobSummaryAggregateSize.
var ErrJobSummaryAggregateExceeded = util.NewInvalidArgumentErrorf("job summary aggregate size exceeded")

func UpsertActionRunJobSummary(ctx context.Context, repoID, runID, runAttemptID, jobID, stepIndex int64, contentType string, content []byte) error {
	if runID <= 0 || jobID <= 0 || repoID <= 0 || stepIndex < 0 {
		return util.ErrInvalidArgument
	}
	if len(content) == 0 {
		// Treat empty summaries as no-op; runner may create SUMMARY.md but never write to it.
		return nil
	}
	if len(content) > MaxJobSummarySize {
		return util.ErrInvalidArgument
	}
	if contentType != JobSummaryContentTypeMarkdown {
		return util.ErrInvalidArgument
	}

	// The aggregate check is best-effort: a tx wouldn't actually serialize concurrent
	// step uploads (no row-level lock on the parent job), so wrapping these two
	// statements only adds round-trip cost without changing the race semantics.
	// The current step is excluded because the upsert below replaces its size with len(content).
	otherSize, err := sumOtherJobSummarySizes(ctx, repoID, runID, runAttemptID, jobID, stepIndex)
	if err != nil {
		return err
	}
	if otherSize+int64(len(content)) > MaxJobSummaryAggregateSize {
		return ErrJobSummaryAggregateExceeded
	}

	now := timeutil.TimeStampNow()
	return upsertActionRunJobSummary(ctx, &ActionRunJobSummary{
		RepoID:       repoID,
		RunID:        runID,
		RunAttemptID: runAttemptID,
		JobID:        jobID,
		StepIndex:    stepIndex,
		Content:      string(content),
		ContentSize:  int64(len(content)),
		ContentType:  contentType,
		Created:      now,
		Updated:      now,
	})
}

// sumOtherJobSummarySizes returns the total stored size of all step summaries for a job
// except excludeStepIndex, computed in the database to avoid loading every row.
func sumOtherJobSummarySizes(ctx context.Context, repoID, runID, runAttemptID, jobID, excludeStepIndex int64) (int64, error) {
	return db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=? AND job_id=? AND step_index<>?", repoID, runID, runAttemptID, jobID, excludeStepIndex).
		SumInt(new(ActionRunJobSummary), "content_size")
}

// DeleteActionRunJobSummary removes the stored summary for a specific step. Used when
// a runner PUTs an empty body to clear a previously-uploaded step summary.
func DeleteActionRunJobSummary(ctx context.Context, repoID, runID, runAttemptID, jobID, stepIndex int64) error {
	_, err := db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=? AND job_id=? AND step_index=?", repoID, runID, runAttemptID, jobID, stepIndex).
		Delete(new(ActionRunJobSummary))
	return err
}

func upsertActionRunJobSummary(ctx context.Context, summary *ActionRunJobSummary) error {
	engine := db.GetEngine(ctx)
	columns := "`repo_id`, `run_id`, `run_attempt_id`, `job_id`, `step_index`, `content`, `content_type`, `content_size`, `created`, `updated`"
	values := []any{
		summary.RepoID,
		summary.RunID,
		summary.RunAttemptID,
		summary.JobID,
		summary.StepIndex,
		summary.Content,
		summary.ContentType,
		summary.ContentSize,
		summary.Created,
		summary.Updated,
	}

	if setting.Database.Type.IsPostgreSQL() || setting.Database.Type.IsSQLite3() {
		args := append([]any{"INSERT INTO `action_run_job_summary` (" + columns + ") VALUES (?,?,?,?,?,?,?,?,?,?) " +
			"ON CONFLICT (`repo_id`, `run_id`, `run_attempt_id`, `job_id`, `step_index`) DO UPDATE SET " +
			"`content` = excluded.`content`, `content_type` = excluded.`content_type`, `content_size` = excluded.`content_size`, `updated` = excluded.`updated`"}, values...)
		_, err := engine.Exec(args...)
		return err
	}

	if setting.Database.Type.IsMySQL() {
		args := append([]any{
			"INSERT INTO `action_run_job_summary` (" + columns + ") VALUES (?,?,?,?,?,?,?,?,?,?) " +
				"ON DUPLICATE KEY UPDATE `content` = VALUES(`content`), `content_type` = VALUES(`content_type`), `content_size` = VALUES(`content_size`), `updated` = VALUES(`updated`)",
		}, values...)
		_, err := engine.Exec(args...)
		return err
	}

	if setting.Database.Type.IsMSSQL() {
		_, err := engine.Exec(`
MERGE INTO action_run_job_summary WITH (HOLDLOCK) AS target
USING (SELECT ? AS repo_id, ? AS run_id, ? AS run_attempt_id, ? AS job_id, ? AS step_index) AS source
ON target.repo_id = source.repo_id
	AND target.run_id = source.run_id
	AND target.run_attempt_id = source.run_attempt_id
	AND target.job_id = source.job_id
	AND target.step_index = source.step_index
WHEN MATCHED THEN
	UPDATE SET content = ?, content_type = ?, content_size = ?, updated = ?
WHEN NOT MATCHED THEN
	INSERT (repo_id, run_id, run_attempt_id, job_id, step_index, content, content_type, content_size, created, updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`,
			summary.RepoID, summary.RunID, summary.RunAttemptID, summary.JobID, summary.StepIndex,
			summary.Content, summary.ContentType, summary.ContentSize, summary.Updated,
			summary.RepoID, summary.RunID, summary.RunAttemptID, summary.JobID, summary.StepIndex, summary.Content, summary.ContentType, summary.ContentSize, summary.Created, summary.Updated)
		return err
	}

	return util.ErrInvalidArgument
}

// ListActionRunJobSummaries lists the stored summaries for a run attempt, ordered by job
// then step. A positive jobID scopes the lookup to that single job, used by the job view to
// avoid rendering every job's summary on each poll; jobID<=0 returns all jobs in the attempt.
func ListActionRunJobSummaries(ctx context.Context, repoID, runID, runAttemptID, jobID int64) ([]*ActionRunJobSummary, error) {
	sess := db.GetEngine(ctx).Where("repo_id=? AND run_id=? AND run_attempt_id=?", repoID, runID, runAttemptID)
	if jobID > 0 {
		sess = sess.And("job_id=?", jobID)
	}
	var summaries []*ActionRunJobSummary
	if err := sess.OrderBy("job_id ASC, step_index ASC").Find(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}
