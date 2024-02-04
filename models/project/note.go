// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// BoardNote is used to represent a note on a boards
type BoardNote struct {
	ID              int64  `xorm:"pk autoincr"`
	Title           string `xorm:"TEXT NOT NULL"`
	Content         string `xorm:"LONGTEXT"`
	RenderedContent string `xorm:"-"`
	Sorting         int64  `xorm:"NOT NULL DEFAULT 0"`
	PinOrder        int64  `xorm:"NOT NULL DEFAULT 0"`

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

	// @TODO: same code in `GetPinnedBoardNotes()` and should be used with `LoadAttributes()`
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

/* @TODO: make it work - markdown.RenderString should also be at this function
// IsPinned returns if a BoardNote is pinned
func (notes BoardNoteList) LoadAttributes() error {
	return nil
} */

var ErrBoardNoteMaxPinReached = util.NewInvalidArgumentErrorf("the max number of pinned board-notes has been readched")

// IsPinned returns if a BoardNote is pinned
func (note *BoardNote) IsPinned() bool {
	return note.PinOrder != 0
}

// IsPinned returns if a BoardNote is pinned
func (note *BoardNote) GetMaxPinOrder(ctx context.Context) (int64, error) {
	var maxPin int64
	_, err := db.GetEngine(ctx).SQL("SELECT MAX(pin_order) FROM board_note WHERE project_id = ?", note.ProjectID).Get(&maxPin)
	if err != nil {
		return -1, err
	}
	return maxPin, nil
}

// IsPinned returns if a BoardNote is pinned
func (note *BoardNote) IsNewPinAllowed(ctx context.Context) bool {
	maxPin, err := note.GetMaxPinOrder(ctx)
	if err != nil {
		return false
	}

	// Check if the maximum allowed Pins reached
	return maxPin < setting.Repository.Project.MaxPinned
}

// Pin pins a BoardNote
func (note *BoardNote) Pin(ctx context.Context) error {
	// If the BoardNote is already pinned, we don't need to pin it twice
	if note.IsPinned() {
		return nil
	}

	maxPin, err := note.GetMaxPinOrder(ctx)
	if err != nil {
		return err
	}

	// Check if the maximum allowed Pins reached
	if maxPin >= setting.Repository.Project.MaxPinned {
		return ErrBoardNoteMaxPinReached
	}

	_, err = db.GetEngine(ctx).Table("board_note").
		Where("id = ?", note.ID).
		Update(map[string]any{
			"pin_order": maxPin + 1,
		})
	if err != nil {
		return err
	}

	return nil
}

// Unpin unpins a BoardNote
func (note *BoardNote) Unpin(ctx context.Context) error {
	// If the BoardNote is not pinned, we don't need to unpin it
	if !note.IsPinned() {
		return nil
	}

	// This sets the Pin for all BoardNotes that come after the unpined BoardNote to the correct value
	_, err := db.GetEngine(ctx).Exec("UPDATE board_note SET pin_order = pin_order - 1 WHERE project_id = ? AND pin_order > ?", note.ProjectID, note.PinOrder)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Table("board_note").
		Where("id = ?", note.ID).
		Update(map[string]any{
			"pin_order": 0,
		})
	if err != nil {
		return err
	}

	return nil
}

// MovePin moves a Pinned BoardNote to a new Position
func (note *BoardNote) MovePin(ctx context.Context, newPosition int64) error {
	// If the BoardNote is not pinned, we can't move them
	if !note.IsPinned() {
		return nil
	}

	if newPosition < 1 {
		return fmt.Errorf("The Position can't be lower than 1")
	}

	dbctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	maxPin, err := note.GetMaxPinOrder(ctx)
	if err != nil {
		return err
	}

	// If the new Position bigger than the current Maximum, set it to the Maximum
	if newPosition > maxPin+1 {
		newPosition = maxPin + 1
	}

	// Lower the Position of all Pinned BoardNotes that came after the current Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE board_note SET pin_order = pin_order - 1 WHERE project_id = ? AND pin_order > ?", note.ProjectID, note.PinOrder)
	if err != nil {
		return err
	}

	// Higher the Position of all Pinned BoardNotes that comes after the new Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE board_note SET pin_order = pin_order + 1 WHERE project_id = ? AND pin_order >= ?", note.ProjectID, newPosition)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(dbctx).Table("board_note").
		Where("id = ?", note.ID).
		Update(map[string]any{
			"pin_order": newPosition,
		})
	if err != nil {
		return err
	}

	return committer.Commit()
}

// GetPinnedBoardNotes returns the pinned BaordNotes for the given Project
func GetPinnedBoardNotes(ctx context.Context, projectID int64) (BoardNoteList, error) {
	notes := make(BoardNoteList, 0)

	err := db.GetEngine(ctx).
		Table("board_note").
		Where("project_id = ?", projectID).
		And("pin_order > 0").
		OrderBy("pin_order").
		Find(&notes)
	if err != nil {
		return nil, err
	}

	// @TODO: same code in `BoardNotes()` and should be used with `LoadAttributes()`
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

// GetLastEventTimestamp returns the last user visible event timestamp, either the creation or the update.
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

// GetTasks returns the amount of tasks in the board-notes content
func (note *BoardNote) GetTasks() int {
	return len(markdown.MarkdownTasksRegex.FindAllStringIndex(note.Content, -1))
}

// GetTasksDone returns the amount of completed tasks in the board-notes content
func (note *BoardNote) GetTasksDone() int {
	return len(markdown.MarkdownTasksDoneRegex.FindAllStringIndex(note.Content, -1))
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
