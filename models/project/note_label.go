// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
)

// BoardNoteLabel represents an project-baord-note-label relation.
type BoardNoteLabel struct {
	ID          int64 `xorm:"pk autoincr"`
	BoardNoteID int64 `xorm:"UNIQUE(s) NOT NULL"`
	LabelID     int64 `xorm:"UNIQUE(s) NOT NULL"`
}

// TableName xorm will read the table name from this method
func (*BoardNoteLabel) TableName() string {
	return "project_board_note_label"
}

// LoadLabels loads labels
func (projectBoardNote *BoardNote) LoadLabelIDs(ctx context.Context) (err error) {
	if projectBoardNote.LabelIDs != nil || len(projectBoardNote.LabelIDs) == 0 {
		projectBoardNote.LabelIDs, err = GetLabelsByBoardNoteID(ctx, projectBoardNote.ID)
		if err != nil {
			return fmt.Errorf("GetLabelsByBoardNoteID [%d]: %w", projectBoardNote.ID, err)
		}
	}
	return nil
}

// LoadLabels removes all labels from project-board-note
func (projectBoardNote *BoardNote) RemoveAllLabels(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where("board_note_id = ?", projectBoardNote.ID).Delete(BoardNoteLabel{})
	return err
}

// LoadLabels add a label to project-board-note -> requires a valid labelID
func (projectBoardNote *BoardNote) AddLabel(ctx context.Context, labelID int64) error {
	_, err := db.GetEngine(ctx).Insert(BoardNoteLabel{
		BoardNoteID: projectBoardNote.ID,
		LabelID:     labelID,
	})
	return err
}

// LoadLabels removes a label from project-board-note
func (projectBoardNote *BoardNote) RemoveLabelByID(ctx context.Context, labelID int64) error {
	_, err := db.GetEngine(ctx).Delete(BoardNoteLabel{
		BoardNoteID: projectBoardNote.ID,
		LabelID:     labelID,
	})
	return err
}

// GetLabelsByBoardNoteID returns all labelIDs that belong to given projectBoardNote by ID.
func GetLabelsByBoardNoteID(ctx context.Context, projectBoardNoteID int64) ([]int64, error) {
	var labelIDs []int64
	return labelIDs, db.GetEngine(ctx).
		Table("label").
		Cols("label.id").
		Asc("label.name").
		Where("project_board_note_label.board_note_id = ?", projectBoardNoteID).
		Join("INNER", "project_board_note_label", "project_board_note_label.label_id = label.id").
		Find(&labelIDs)
}
