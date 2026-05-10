// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

const (
	// JobSummaryCapability is the runner-declare capability string for job summaries.
	JobSummaryCapability = "job-summary"

	// JobSummaryContentTypeMarkdown is the only accepted content type for job summaries.
	JobSummaryContentTypeMarkdown = "text/markdown"

	// MaxJobSummarySize is the maximum accepted summary payload size in bytes.
	// This is intentionally conservative to avoid DB bloat and UI abuse.
	MaxJobSummarySize = 1024 * 1024 // 1 MiB
)

// ActionRunJobSummary stores one raw GITHUB_STEP_SUMMARY markdown upload for a job step.
// It is internal state grouped for display as a job summary (not a downloadable artifact).
type ActionRunJobSummary struct {
	ID int64 `xorm:"pk autoincr"`

	RepoID       int64 `xorm:"UNIQUE(summary_key)"`
	RunID        int64 `xorm:"UNIQUE(summary_key)"`
	RunAttemptID int64 `xorm:"UNIQUE(summary_key) NOT NULL DEFAULT 0"`
	JobID        int64 `xorm:"UNIQUE(summary_key)"`
	StepIndex    int64 `xorm:"UNIQUE(summary_key)"`

	Content     string `xorm:"LONGTEXT"`
	ContentType string `xorm:"VARCHAR(255) NOT NULL DEFAULT 'text/markdown'"`

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
	if contentType == "" {
		contentType = JobSummaryContentTypeMarkdown
	}
	if contentType != JobSummaryContentTypeMarkdown {
		return util.ErrInvalidArgument
	}

	now := timeutil.TimeStampNow()
	summary := &ActionRunJobSummary{
		RepoID:       repoID,
		RunID:        runID,
		RunAttemptID: runAttemptID,
		JobID:        jobID,
		StepIndex:    stepIndex,
		Content:      string(content),
		ContentType:  contentType,
		Created:      now,
		Updated:      now,
	}
	return upsertActionRunJobSummary(ctx, summary)
}

func upsertActionRunJobSummary(ctx context.Context, summary *ActionRunJobSummary) error {
	engine := db.GetEngine(ctx)
	columns := "`repo_id`, `run_id`, `run_attempt_id`, `job_id`, `step_index`, `content`, `content_type`, `created`, `updated`"
	values := []any{
		summary.RepoID,
		summary.RunID,
		summary.RunAttemptID,
		summary.JobID,
		summary.StepIndex,
		summary.Content,
		summary.ContentType,
		summary.Created,
		summary.Updated,
	}

	if setting.Database.Type.IsPostgreSQL() || setting.Database.Type.IsSQLite3() {
		args := append([]any{"INSERT INTO `action_run_job_summary` (" + columns + ") VALUES (?,?,?,?,?,?,?,?,?) " +
			"ON CONFLICT (`repo_id`, `run_id`, `run_attempt_id`, `job_id`, `step_index`) DO UPDATE SET " +
			"`content` = excluded.`content`, `content_type` = excluded.`content_type`, `updated` = excluded.`updated`"}, values...)
		_, err := engine.Exec(args...)
		return err
	}

	if setting.Database.Type.IsMySQL() {
		args := append([]any{
			"INSERT INTO `action_run_job_summary` (" + columns + ") VALUES (?,?,?,?,?,?,?,?,?) " +
				"ON DUPLICATE KEY UPDATE `content` = VALUES(`content`), `content_type` = VALUES(`content_type`), `updated` = VALUES(`updated`)",
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
	UPDATE SET content = ?, content_type = ?, updated = ?
WHEN NOT MATCHED THEN
	INSERT (repo_id, run_id, run_attempt_id, job_id, step_index, content, content_type, created, updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
`,
			summary.RepoID, summary.RunID, summary.RunAttemptID, summary.JobID, summary.StepIndex,
			summary.Content, summary.ContentType, summary.Updated,
			summary.RepoID, summary.RunID, summary.RunAttemptID, summary.JobID, summary.StepIndex, summary.Content, summary.ContentType, summary.Created, summary.Updated)
		return err
	}

	return util.ErrInvalidArgument
}

func ListActionRunJobSummariesByRunAttempt(ctx context.Context, repoID, runID, runAttemptID int64) ([]*ActionRunJobSummary, error) {
	var summaries []*ActionRunJobSummary
	if err := db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=?", repoID, runID, runAttemptID).
		OrderBy("job_id ASC, step_index ASC").
		Find(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}
