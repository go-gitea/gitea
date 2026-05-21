// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ActionRunAnalysis is a structured per-attempt note explaining a workflow attempt's outcome.
// At most one analysis exists per attempt; it is created lazily on first save.
type ActionRunAnalysis struct {
	ID          int64              `xorm:"pk autoincr"`
	AttemptID   int64              `xorm:"UNIQUE NOT NULL"`
	RunID       int64              `xorm:"INDEX NOT NULL"`
	RepoID      int64              `xorm:"INDEX NOT NULL"` // denormalized for repo-wide aggregation
	AuthorID    int64              `xorm:"NOT NULL"`
	Note        string             `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (*ActionRunAnalysis) TableName() string { return "action_run_analysis" }

// ActionRunAnalysisTag links an analysis to a failure tag (many-to-many).
type ActionRunAnalysisTag struct {
	AnalysisID  int64              `xorm:"pk"`
	TagID       int64              `xorm:"pk INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (*ActionRunAnalysisTag) TableName() string { return "action_run_analysis_tag" }

func init() {
	db.RegisterModel(new(ActionRunAnalysis))
	db.RegisterModel(new(ActionRunAnalysisTag))
}

// GetAnalysisByAttemptID returns the analysis for an attempt, or util.ErrNotExist if none exists.
func GetAnalysisByAttemptID(ctx context.Context, attemptID int64) (*ActionRunAnalysis, error) {
	var a ActionRunAnalysis
	has, err := db.GetEngine(ctx).Where("attempt_id = ?", attemptID).Get(&a)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return &a, nil
}

// GetAnalysisTags returns the resolved failure tags attached to an analysis.
func GetAnalysisTags(ctx context.Context, analysisID int64) ([]*ActionRunFailureTag, error) {
	tags := make([]*ActionRunFailureTag, 0)
	return tags, db.GetEngine(ctx).
		Join("INNER", "action_run_analysis_tag", "action_run_analysis_tag.tag_id = action_run_failure_tag.id").
		Where("action_run_analysis_tag.analysis_id = ?", analysisID).
		Asc("action_run_failure_tag.name").
		Find(&tags)
}

// UpsertAnalysis creates or updates the analysis for an attempt, replacing its tag set.
// repoID and runID are required on insert and ignored on update.
// tagIDs are filtered to those that actually belong to repoID for safety.
func UpsertAnalysis(ctx context.Context, repoID, runID, attemptID, authorID int64, note string, tagIDs []int64) (*ActionRunAnalysis, error) {
	var analysis *ActionRunAnalysis
	err := db.WithTx(ctx, func(ctx context.Context) error {
		existing, err := GetAnalysisByAttemptID(ctx, attemptID)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
		if existing == nil {
			existing = &ActionRunAnalysis{
				AttemptID: attemptID,
				RunID:     runID,
				RepoID:    repoID,
				AuthorID:  authorID,
				Note:      note,
			}
			if _, err := db.GetEngine(ctx).Insert(existing); err != nil {
				return err
			}
		} else {
			existing.Note = note
			existing.AuthorID = authorID
			if _, err := db.GetEngine(ctx).ID(existing.ID).Cols("note", "author_id").Update(existing); err != nil {
				return err
			}
		}

		validTagIDs, err := filterTagIDsByRepo(ctx, repoID, tagIDs)
		if err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Where("analysis_id = ?", existing.ID).Delete(new(ActionRunAnalysisTag)); err != nil {
			return err
		}
		if len(validTagIDs) > 0 {
			links := make([]*ActionRunAnalysisTag, 0, len(validTagIDs))
			for _, tid := range validTagIDs {
				links = append(links, &ActionRunAnalysisTag{AnalysisID: existing.ID, TagID: tid})
			}
			if _, err := db.GetEngine(ctx).Insert(&links); err != nil {
				return err
			}
		}
		analysis = existing
		return nil
	})
	return analysis, err
}

// DeleteAnalysis removes the analysis for an attempt (and its tag links).
func DeleteAnalysis(ctx context.Context, repoID, attemptID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		existing, err := GetAnalysisByAttemptID(ctx, attemptID)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
		if existing == nil {
			return nil
		}
		if existing.RepoID != repoID {
			return fmt.Errorf("analysis %d does not belong to repo %d: %w", existing.ID, repoID, util.ErrPermissionDenied)
		}
		if _, err := db.GetEngine(ctx).Where("analysis_id = ?", existing.ID).Delete(new(ActionRunAnalysisTag)); err != nil {
			return err
		}
		_, err = db.GetEngine(ctx).ID(existing.ID).Delete(new(ActionRunAnalysis))
		return err
	})
}

func filterTagIDsByRepo(ctx context.Context, repoID int64, tagIDs []int64) ([]int64, error) {
	if len(tagIDs) == 0 {
		return []int64{}, nil
	}
	valid := make([]int64, 0, len(tagIDs))
	err := db.GetEngine(ctx).Table("action_run_failure_tag").
		Where("repo_id = ?", repoID).In("id", tagIDs).Cols("id").Find(&valid)
	return valid, err
}
