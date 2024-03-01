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
	ID          int64   `xorm:"pk autoincr"`
	Title       string  `xorm:"TEXT NOT NULL"`
	Content     string  `xorm:"LONGTEXT"`
	Sorting     int64   `xorm:"NOT NULL DEFAULT 0"`
	PinOrder    int64   `xorm:"NOT NULL DEFAULT 0"`
	LabelIDs    []int64 `xorm:"-"` // can't be []*Label because of 'import cycle not allowed'
	MilestoneID int64   `xorm:"INDEX"`

	ProjectID int64            `xorm:"INDEX NOT NULL"`
	BoardID   int64            `xorm:"INDEX NOT NULL"`
	CreatorID int64            `xorm:"NOT NULL"`
	Creator   *user_model.User `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName xorm will read the table name from this method
func (*BoardNote) TableName() string {
	return "project_board_note"
}

type BoardNoteList []*BoardNote

// BoardNotesOptions represents options of an note.
type BoardNotesOptions struct {
	ProjectID int64
	BoardID   int64
	IsPinned  util.OptionalBool
}

func init() {
	db.RegisterModel(new(BoardNote))
	db.RegisterModel(new(BoardNoteLabel))
}

// GetBoardNoteByID load note by ID
func GetBoardNoteByID(ctx context.Context, projectBoardNoteID int64) (*BoardNote, error) {
	projectBoardNote := new(BoardNote)

	has, err := db.GetEngine(ctx).ID(projectBoardNoteID).Get(projectBoardNote)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBoardNoteNotExist{BoardNoteID: projectBoardNoteID}
	}

	return projectBoardNote, nil
}

// GetBoardNotesByIds return notes with the given IDs.
func GetBoardNotesByIds(ctx context.Context, projectBoardNoteIDs []int64) (BoardNoteList, error) {
	projectBoardNoteList := make(BoardNoteList, 0, len(projectBoardNoteIDs))

	if err := db.GetEngine(ctx).In("id", projectBoardNoteIDs).Find(&projectBoardNoteList); err != nil {
		return nil, err
	}

	if err := projectBoardNoteList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// GetBoardNotesByProjectID load pinned notes assigned to the project
func GetBoardNotesByProjectID(ctx context.Context, projectID int64, isPinned bool) (BoardNoteList, error) {
	projectBoardNoteList, err := BoardNotes(ctx, &BoardNotesOptions{
		ProjectID: projectID,
		BoardID:   -1,
		IsPinned:  util.OptionalBoolOf(isPinned),
	})
	if err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// LoadBoardNotesFromBoardList load notes assigned to the boards
func (p *Project) LoadBoardNotesFromBoardList(ctx context.Context, boardList BoardList) (map[int64]BoardNoteList, error) {
	projectBoardNoteListMap := make(map[int64]BoardNoteList, len(boardList))
	for i := range boardList {
		il, err := LoadBoardNotesFromBoard(ctx, boardList[i])
		if err != nil {
			return nil, err
		}
		projectBoardNoteListMap[boardList[i].ID] = il
	}
	return projectBoardNoteListMap, nil
}

// LoadBoardNotesFromBoard load notes assigned to this board
func LoadBoardNotesFromBoard(ctx context.Context, board *Board) (BoardNoteList, error) {
	projectBoardNoteList, err := BoardNotes(ctx, &BoardNotesOptions{
		ProjectID: board.ProjectID,
		BoardID:   board.ID,
	})
	if err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// BoardNotes returns a list of notes by given conditions.
func BoardNotes(ctx context.Context, opts *BoardNotesOptions) (BoardNoteList, error) {
	sess := db.GetEngine(ctx)

	if opts.BoardID >= 0 {
		sess.Where(builder.Eq{"board_id": opts.BoardID})
	}
	if opts.ProjectID >= 0 {
		sess.Where(builder.Eq{"project_id": opts.ProjectID})
	}
	if !opts.IsPinned.IsNone() {
		if opts.IsPinned.IsTrue() {
			sess.Where(builder.NotNull{"pin_order"}).And(builder.Gt{"pin_order": 0})
		} else {
			sess.Where(builder.IsNull{"pin_order"}).Or(builder.Eq{"pin_order": 0})
		}
	}

	projectBoardNoteList := BoardNoteList{}
	if err := sess.Asc("sorting").Desc("updated_unix").Desc("id").Find(&projectBoardNoteList); err != nil {
		return nil, fmt.Errorf("unable to query project-board-notes: %w", err)
	}

	if err := projectBoardNoteList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// NewBoardNote adds a new note to a given board
func NewBoardNote(ctx context.Context, projectBoardNote *BoardNote) error {
	_, err := db.GetEngine(ctx).Insert(projectBoardNote)
	return err
}

// LoadAttributes prerenders the markdown content and sets the creator
func (projectBoardNoteList BoardNoteList) LoadAttributes(ctx context.Context) error {
	for _, projectBoardNote := range projectBoardNoteList {
		creator := new(user_model.User)
		has, err := db.GetEngine(ctx).ID(projectBoardNote.CreatorID).Get(creator)
		if err != nil {
			return err
		}
		if !has {
			return user_model.ErrUserNotExist{UID: projectBoardNote.CreatorID}
		}
		projectBoardNote.Creator = creator

		if err := projectBoardNote.LoadLabelIDs(ctx); err != nil {
			return err
		}
	}

	return nil
}

var ErrBoardNoteMaxPinReached = util.NewInvalidArgumentErrorf("the max number of pinned project-board-notes has been readched")

// IsPinned returns if a BoardNote is pinned
func (projectBoardNote *BoardNote) IsPinned() bool {
	return projectBoardNote.PinOrder != 0
}

// IsPinned returns if a BoardNote is pinned
func (projectBoardNote *BoardNote) GetMaxPinOrder(ctx context.Context) (int64, error) {
	var maxPin int64
	_, err := db.GetEngine(ctx).SQL("SELECT MAX(pin_order) FROM project_board_note WHERE project_id = ?", projectBoardNote.ProjectID).Get(&maxPin)
	if err != nil {
		return -1, err
	}
	return maxPin, nil
}

// IsPinned returns if a BoardNote is pinned
func (projectBoardNote *BoardNote) IsNewPinAllowed(ctx context.Context) bool {
	maxPin, err := projectBoardNote.GetMaxPinOrder(ctx)
	if err != nil {
		return false
	}

	// Check if the maximum allowed Pins reached
	return maxPin < setting.Repository.Project.MaxPinned
}

// Pin pins a BoardNote
func (projectBoardNote *BoardNote) Pin(ctx context.Context) error {
	// If the BoardNote is already pinned, we don't need to pin it twice
	if projectBoardNote.IsPinned() {
		return nil
	}

	maxPin, err := projectBoardNote.GetMaxPinOrder(ctx)
	if err != nil {
		return err
	}

	// Check if the maximum allowed Pins reached
	if maxPin >= setting.Repository.Project.MaxPinned {
		return ErrBoardNoteMaxPinReached
	}

	_, err = db.GetEngine(ctx).Table("project_board_note").
		Where("id = ?", projectBoardNote.ID).
		Update(map[string]any{
			"pin_order": maxPin + 1,
		})
	if err != nil {
		return err
	}

	return nil
}

// Unpin unpins a BoardNote
func (projectBoardNote *BoardNote) Unpin(ctx context.Context) error {
	// If the BoardNote is not pinned, we don't need to unpin it
	if !projectBoardNote.IsPinned() {
		return nil
	}

	// This sets the Pin for all BoardNotes that come after the unpined BoardNote to the correct value
	_, err := db.GetEngine(ctx).Exec("UPDATE project_board_note SET pin_order = pin_order - 1 WHERE project_id = ? AND pin_order > ?", projectBoardNote.ProjectID, projectBoardNote.PinOrder)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Table("project_board_note").
		Where("id = ?", projectBoardNote.ID).
		Update(map[string]any{
			"pin_order": 0,
		})
	if err != nil {
		return err
	}

	return nil
}

// MovePin moves a Pinned BoardNote to a new Position
func (projectBoardNote *BoardNote) MovePin(ctx context.Context, newPosition int64) error {
	// If the BoardNote is not pinned, we can't move them
	if !projectBoardNote.IsPinned() {
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

	maxPin, err := projectBoardNote.GetMaxPinOrder(ctx)
	if err != nil {
		return err
	}

	// If the new Position bigger than the current Maximum, set it to the Maximum
	if newPosition > maxPin+1 {
		newPosition = maxPin + 1
	}

	// Lower the Position of all Pinned BoardNotes that came after the current Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE project_board_note SET pin_order = pin_order - 1 WHERE project_id = ? AND pin_order > ?", projectBoardNote.ProjectID, projectBoardNote.PinOrder)
	if err != nil {
		return err
	}

	// Higher the Position of all Pinned BoardNotes that comes after the new Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE project_board_note SET pin_order = pin_order + 1 WHERE project_id = ? AND pin_order >= ?", projectBoardNote.ProjectID, newPosition)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(dbctx).Table("project_board_note").
		Where("id = ?", projectBoardNote.ID).
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
	projectBoardNoteList := make(BoardNoteList, 0)

	err := db.GetEngine(ctx).
		Table("project_board_note").
		Where("project_id = ?", projectID).
		And("pin_order > 0").
		OrderBy("pin_order").
		Find(&projectBoardNoteList)
	if err != nil {
		return nil, err
	}

	if err := projectBoardNoteList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// GetTasks returns the amount of tasks in the project-board-notes content
func (projectBoardNote *BoardNote) GetTasks() int {
	return len(markdown.MarkdownTasksRegex.FindAllStringIndex(projectBoardNote.Content, -1))
}

// GetTasksDone returns the amount of completed tasks in the project-board-notes content
func (projectBoardNote *BoardNote) GetTasksDone() int {
	return len(markdown.MarkdownTasksDoneRegex.FindAllStringIndex(projectBoardNote.Content, -1))
}

// UpdateBoardNote changes a BoardNote
func UpdateBoardNote(ctx context.Context, projectBoardNote *BoardNote) error {
	var fieldToUpdate []string

	fieldToUpdate = append(fieldToUpdate, "title")
	fieldToUpdate = append(fieldToUpdate, "content")
	fieldToUpdate = append(fieldToUpdate, "milestone_id")

	_, err := db.GetEngine(ctx).ID(projectBoardNote.ID).Cols(fieldToUpdate...).Update(projectBoardNote)
	return err
}

// MoveBoardNoteOnProjectBoard moves or keeps notes in a column and sorts them inside that column
func MoveBoardNoteOnProjectBoard(ctx context.Context, board *Board, sortedBoardNoteIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		for sorting, issueID := range sortedBoardNoteIDs {
			_, err := sess.Exec("UPDATE `project_board_note` SET board_id=?, sorting=? WHERE id=?", board.ID, sorting, issueID)
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
	_, err := db.GetEngine(ctx).Exec("UPDATE `project_board_note` SET board_id = 0 WHERE board_id = ?", b.ID)
	return err
}
