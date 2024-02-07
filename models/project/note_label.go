// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
)

// ProjectBoardNoteLabel represents an project-baord-note-label relation.
type ProjectBoardNoteLabel struct {
	ID                 int64 `xorm:"pk autoincr"`
	ProjectBoardNoteID int64 `xorm:"UNIQUE(s) NOT NULL"`
	LabelID            int64 `xorm:"UNIQUE(s) NOT NULL"`
}

// LoadLabels loads labels
func (projectBoardNote *ProjectBoardNote) LoadLabelIDs(ctx context.Context) (err error) {
	if projectBoardNote.LabelIDs != nil || len(projectBoardNote.LabelIDs) == 0 {
		projectBoardNote.LabelIDs, err = GetLabelsByProjectBoardNoteID(ctx, projectBoardNote.ID)
		if err != nil {
			return fmt.Errorf("GetLabelsByProjectBoardNoteID [%d]: %w", projectBoardNote.ID, err)
		}
	}
	return nil
}

// LoadLabels removes all labels from project-board-note
func (projectBoardNote *ProjectBoardNote) RemoveAllLabels(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where("project_board_note_id = ?", projectBoardNote.ID).Delete(ProjectBoardNoteLabel{})
	return err
}

// LoadLabels add a label to project-board-note -> requires a valid labelID
func (projectBoardNote *ProjectBoardNote) AddLabel(ctx context.Context, labelID int64) error {
	_, err := db.GetEngine(ctx).Insert(ProjectBoardNoteLabel{
		ProjectBoardNoteID: projectBoardNote.ID,
		LabelID:            labelID,
	})
	return err
}

// LoadLabels removes a label from project-board-note
func (projectBoardNote *ProjectBoardNote) RemoveLabelByID(ctx context.Context, labelID int64) error {
	_, err := db.GetEngine(ctx).Delete(ProjectBoardNoteLabel{
		ProjectBoardNoteID: projectBoardNote.ID,
		LabelID:            labelID,
	})
	return err
}

// GetLabelsByProjectBoardNoteID returns all labelIDs that belong to given projectBoardNote by ID.
func GetLabelsByProjectBoardNoteID(ctx context.Context, projectBoardNoteID int64) ([]int64, error) {
	var labelIDs []int64
	return labelIDs, db.GetEngine(ctx).
		Table("label").
		Cols("label.id").
		Asc("label.name").
		Where("project_board_note_label.project_board_note_id = ?", projectBoardNoteID).
		Join("INNER", "project_board_note_label", "project_board_note_label.label_id = label.id").
		Find(&labelIDs)
}
