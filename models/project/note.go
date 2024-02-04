// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// BoardNote is used to represent a note on a boards
type BoardNote struct {
	ID              int64  `xorm:"pk autoincr"`
	Title           string `xorm:"TEXT NOT NULL"`
	Content         string `xorm:"LONGTEXT"`
	RenderedContent string `xorm:"-"`
	Sorting         int64  `xorm:"NOT NULL DEFAULT 0"`

	ProjectID int64            `xorm:"INDEX NOT NULL"`
	BoardID   int64            `xorm:"INDEX NOT NULL"`
	CreatorID int64            `xorm:"NOT NULL"`
	Creator   *user_model.User `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type BoardNoteList = []*BoardNote

// NotesOptions represents options of an note.
type NotesOptions struct {
	db.Paginator
	ProjectID int64
	BoardID   int64
}

func init() {
	db.RegisterModel(new(BoardNote))
}

// GetBoardNoteByID load notes assigned to the boards
func GetBoardNoteByID(ctx context.Context, noteID int64) (*BoardNote, error) {
	note := new(BoardNote)

	has, err := db.GetEngine(ctx).ID(noteID).Get(note)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNoteNotExist{BoardNoteID: noteID}
	}

	return note, nil
}

// GetBoardNoteByIds return notes with the given IDs.
func GetBoardNoteByIds(ctx context.Context, noteIDs []int64) (BoardNoteList, error) {
	notes := make(BoardNoteList, 0, len(noteIDs))

	if err := db.GetEngine(ctx).In("id", noteIDs).Find(&notes); err != nil {
		return nil, err
	}

	return notes, nil
}

// LoadBoardNotesFromBoardList load notes assigned to the boards
func (p *Project) LoadBoardNotesFromBoardList(ctx context.Context, bs BoardList) (map[int64]BoardNoteList, error) {
	notesMap := make(map[int64]BoardNoteList, len(bs))
	for i := range bs {
		il, err := LoadBoardNotesFromBoard(ctx, bs[i])
		if err != nil {
			return nil, err
		}
		notesMap[bs[i].ID] = il
	}
	return notesMap, nil
}

// LoadBoardNotesFromBoard load notes assigned to this board
func LoadBoardNotesFromBoard(ctx context.Context, board *Board) (BoardNoteList, error) {
	notes, err := BoardNotes(ctx, &NotesOptions{
		ProjectID: board.ProjectID,
		BoardID:   board.ID,
	})
	if err != nil {
		return nil, err
	}

	return notes, nil
}

// BoardNotes returns a list of notes by given conditions.
func BoardNotes(ctx context.Context, opts *NotesOptions) (BoardNoteList, error) {
	sess := db.GetEngine(ctx)

	sess.Where(builder.Eq{"board_id": opts.BoardID}).And(builder.Eq{"project_id": opts.ProjectID})

	notes := BoardNoteList{}
	if err := sess.Asc("sorting").Desc("updated_unix").Desc("id").Find(&notes); err != nil {
		return nil, fmt.Errorf("unable to query Notes: %w", err)
	}

	for _, note := range notes {
		creator := new(user_model.User)
		has, err := db.GetEngine(ctx).ID(note.CreatorID).Get(creator)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, user_model.ErrUserNotExist{UID: note.CreatorID}
		}
		note.Creator = creator
	}

	return notes, nil
}

// NewBoardNote adds a new note to a given board
func NewBoardNote(ctx context.Context, note *BoardNote) error {
	_, err := db.GetEngine(ctx).Insert(note)
	return err
}

// GetLastEventTimestamp returns the last user visible event timestamp, either the creation of this issue or the close.
func (note *BoardNote) GetLastEventTimestamp() timeutil.TimeStamp {
	return max(note.CreatedUnix, note.UpdatedUnix)
}

// GetLastEventLabel returns the localization label for the current note.
func (note *BoardNote) GetLastEventLabel() string {
	if note.UpdatedUnix > note.CreatedUnix {
		return "repo.projects.note.updated_by"
	}
	return "repo.projects.note.created_by"
}

// UpdateBoardNote changes a BoardNote
func UpdateBoardNote(ctx context.Context, note *BoardNote) error {
	var fieldToUpdate []string

	fieldToUpdate = append(fieldToUpdate, "title")
	fieldToUpdate = append(fieldToUpdate, "content")

	_, err := db.GetEngine(ctx).ID(note.ID).Cols(fieldToUpdate...).Update(note)
	return err
}

// MoveBoardNoteOnProjectBoard moves or keeps notes in a column and sorts them inside that column
func MoveBoardNoteOnProjectBoard(ctx context.Context, board *Board, sortedNoteIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		for sorting, issueID := range sortedNoteIDs {
			_, err := sess.Exec("UPDATE `board_note` SET board_id=?, sorting=? WHERE id=?", board.ID, sorting, issueID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func deleteBoardNoteByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&BoardNote{})
	return err
}

// DeleteBoardNote removes the BoardNote from the project board.
func DeleteBoardNote(ctx context.Context, boardNote *BoardNote) error {
	if _, err := db.GetEngine(ctx).Delete(boardNote); err != nil {
		return err
	}
	return nil
}

// removeBoardNotes sets the boardID to 0 for the board
func (b *Board) removeBoardNotes(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `board_note` SET board_id = 0 WHERE board_id = ?", b.ID)
	return err
}
