// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
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

// ActionRunJobSummary stores the raw job summary markdown uploaded by the runner.
// It is internal state (not a downloadable artifact).
type ActionRunJobSummary struct {
	ID int64 `xorm:"pk autoincr"`

	RepoID       int64 `xorm:"UNIQUE(summary_key) INDEX"`
	RunID        int64 `xorm:"UNIQUE(summary_key) INDEX"`
	RunAttemptID int64 `xorm:"UNIQUE(summary_key) NOT NULL DEFAULT 0 INDEX"`
	JobID        int64 `xorm:"UNIQUE(summary_key) INDEX"`

	Content     string `xorm:"LONGTEXT"`
	ContentType string `xorm:"VARCHAR(255) NOT NULL DEFAULT 'text/markdown'"`

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionRunJobSummary))
}

func GetActionRunJobSummary(ctx context.Context, repoID, runID, runAttemptID, jobID int64) (*ActionRunJobSummary, error) {
	var s ActionRunJobSummary
	has, err := db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=? AND job_id=?", repoID, runID, runAttemptID, jobID).
		Get(&s)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return &s, nil
}

func UpsertActionRunJobSummary(ctx context.Context, repoID, runID, runAttemptID, jobID int64, contentType string, content []byte) error {
	if runID <= 0 || jobID <= 0 || repoID <= 0 {
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

	engine := db.GetEngine(ctx)

	existing, err := GetActionRunJobSummary(ctx, repoID, runID, runAttemptID, jobID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}

	if existing == nil {
		_, err := engine.Insert(&ActionRunJobSummary{
			RepoID:       repoID,
			RunID:        runID,
			RunAttemptID: runAttemptID,
			JobID:        jobID,
			Content:      string(content),
			ContentType:  contentType,
		})
		return err
	}

	existing.Content = string(content)
	existing.ContentType = contentType
	_, err = engine.ID(existing.ID).Cols("content", "content_type").Update(existing)
	return err
}

func ListActionRunJobSummariesByRunAttempt(ctx context.Context, repoID, runID, runAttemptID int64) ([]*ActionRunJobSummary, error) {
	var summaries []*ActionRunJobSummary
	if err := db.GetEngine(ctx).
		Where("repo_id=? AND run_id=? AND run_attempt_id=?", repoID, runID, runAttemptID).
		OrderBy("job_id ASC").
		Find(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}
