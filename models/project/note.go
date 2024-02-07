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

// ProjectBoardNote is used to represent a note on a boards
type ProjectBoardNote struct {
	ID              int64   `xorm:"pk autoincr"`
	Title           string  `xorm:"TEXT NOT NULL"`
	Content         string  `xorm:"LONGTEXT"`
	RenderedContent string  `xorm:"-"`
	Sorting         int64   `xorm:"NOT NULL DEFAULT 0"`
	PinOrder        int64   `xorm:"NOT NULL DEFAULT 0"`
	LabelIDs        []int64 `xorm:"-"` // can't be []*Label because of 'import cycle not allowed'

	ProjectID int64            `xorm:"INDEX NOT NULL"`
	BoardID   int64            `xorm:"INDEX NOT NULL"`
	CreatorID int64            `xorm:"NOT NULL"`
	Creator   *user_model.User `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type ProjectBoardNoteList []*ProjectBoardNote

// ProjectBoardNotesOptions represents options of an note.
type ProjectBoardNotesOptions struct {
	db.Paginator
	ProjectID int64
	BoardID   int64
}

func init() {
	db.RegisterModel(new(ProjectBoardNote))
	db.RegisterModel(new(ProjectBoardNoteLabel))
}

// GetProjectBoardNoteByID load notes assigned to the boards
func GetProjectBoardNoteByID(ctx context.Context, projectBoardNoteID int64) (*ProjectBoardNote, error) {
	projectBoardNote := new(ProjectBoardNote)

	has, err := db.GetEngine(ctx).ID(projectBoardNoteID).Get(projectBoardNote)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNoteNotExist{ProjectBoardNoteID: projectBoardNoteID}
	}

	return projectBoardNote, nil
}

// GetProjectBoardNotesByIds return notes with the given IDs.
func GetProjectBoardNotesByIds(ctx context.Context, projectBoardNoteIDs []int64) (ProjectBoardNoteList, error) {
	projectBoardNoteList := make(ProjectBoardNoteList, 0, len(projectBoardNoteIDs))

	if err := db.GetEngine(ctx).In("id", projectBoardNoteIDs).Find(&projectBoardNoteList); err != nil {
		return nil, err
	}

	if err := projectBoardNoteList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// LoadProjectBoardNotesFromBoardList load notes assigned to the boards
func (p *Project) LoadProjectBoardNotesFromBoardList(ctx context.Context, boardList BoardList) (map[int64]ProjectBoardNoteList, error) {
	projectBoardNoteListMap := make(map[int64]ProjectBoardNoteList, len(boardList))
	for i := range boardList {
		il, err := LoadProjectBoardNotesFromBoard(ctx, boardList[i])
		if err != nil {
			return nil, err
		}
		projectBoardNoteListMap[boardList[i].ID] = il
	}
	return projectBoardNoteListMap, nil
}

// LoadProjectBoardNotesFromBoard load notes assigned to this board
func LoadProjectBoardNotesFromBoard(ctx context.Context, board *Board) (ProjectBoardNoteList, error) {
	projectBoardNoteList, err := ProjectBoardNotes(ctx, &ProjectBoardNotesOptions{
		ProjectID: board.ProjectID,
		BoardID:   board.ID,
	})
	if err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// ProjectBoardNotes returns a list of notes by given conditions.
func ProjectBoardNotes(ctx context.Context, opts *ProjectBoardNotesOptions) (ProjectBoardNoteList, error) {
	sess := db.GetEngine(ctx)

	sess.Where(builder.Eq{"board_id": opts.BoardID}).And(builder.Eq{"project_id": opts.ProjectID})

	projectBoardNoteList := ProjectBoardNoteList{}
	if err := sess.Asc("sorting").Desc("updated_unix").Desc("id").Find(&projectBoardNoteList); err != nil {
		return nil, fmt.Errorf("unable to query project-board-notes: %w", err)
	}

	if err := projectBoardNoteList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return projectBoardNoteList, nil
}

// NewProjectBoardNote adds a new note to a given board
func NewProjectBoardNote(ctx context.Context, projectBoardNote *ProjectBoardNote) error {
	_, err := db.GetEngine(ctx).Insert(projectBoardNote)
	return err
}

// LoadAttributes prerenders the markdown content and sets the creator
func (projectBoardNoteList ProjectBoardNoteList) LoadAttributes(ctx context.Context) error {
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
func (projectBoardNote *ProjectBoardNote) IsPinned() bool {
	return projectBoardNote.PinOrder != 0
}

// IsPinned returns if a BoardNote is pinned
func (projectBoardNote *ProjectBoardNote) GetMaxPinOrder(ctx context.Context) (int64, error) {
	var maxPin int64
	_, err := db.GetEngine(ctx).SQL("SELECT MAX(pin_order) FROM project_board_note WHERE project_id = ?", projectBoardNote.ProjectID).Get(&maxPin)
	if err != nil {
		return -1, err
	}
	return maxPin, nil
}

// IsPinned returns if a BoardNote is pinned
func (projectBoardNote *ProjectBoardNote) IsNewPinAllowed(ctx context.Context) bool {
	maxPin, err := projectBoardNote.GetMaxPinOrder(ctx)
	if err != nil {
		return false
	}

	// Check if the maximum allowed Pins reached
	return maxPin < setting.Repository.Project.MaxPinned
}

// Pin pins a BoardNote
func (projectBoardNote *ProjectBoardNote) Pin(ctx context.Context) error {
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
func (projectBoardNote *ProjectBoardNote) Unpin(ctx context.Context) error {
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
func (projectBoardNote *ProjectBoardNote) MovePin(ctx context.Context, newPosition int64) error {
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

// GetPinnedProjectBoardNotes returns the pinned BaordNotes for the given Project
func GetPinnedProjectBoardNotes(ctx context.Context, projectID int64) (ProjectBoardNoteList, error) {
	projectBoardNoteList := make(ProjectBoardNoteList, 0)

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

// GetLastEventTimestamp returns the last user visible event timestamp, either the creation or the update.
func (projectBoardNote *ProjectBoardNote) GetLastEventTimestamp() timeutil.TimeStamp {
	return max(projectBoardNote.CreatedUnix, projectBoardNote.UpdatedUnix)
}

// GetLastEventLabel returns the localization label for the current note.
func (projectBoardNote *ProjectBoardNote) GetLastEventLabel() string {
	if projectBoardNote.UpdatedUnix > projectBoardNote.CreatedUnix {
		return "repo.projects.note.updated_by"
	}
	return "repo.projects.note.created_by"
}

// GetTasks returns the amount of tasks in the project-board-notes content
func (projectBoardNote *ProjectBoardNote) GetTasks() int {
	return len(markdown.MarkdownTasksRegex.FindAllStringIndex(projectBoardNote.Content, -1))
}

// GetTasksDone returns the amount of completed tasks in the project-board-notes content
func (projectBoardNote *ProjectBoardNote) GetTasksDone() int {
	return len(markdown.MarkdownTasksDoneRegex.FindAllStringIndex(projectBoardNote.Content, -1))
}

// UpdateProjectBoardNote changes a BoardNote
func UpdateProjectBoardNote(ctx context.Context, projectBoardNote *ProjectBoardNote) error {
	var fieldToUpdate []string

	fieldToUpdate = append(fieldToUpdate, "title")
	fieldToUpdate = append(fieldToUpdate, "content")

	_, err := db.GetEngine(ctx).ID(projectBoardNote.ID).Cols(fieldToUpdate...).Update(projectBoardNote)
	return err
}

// MoveProjectBoardNoteOnProjectBoard moves or keeps notes in a column and sorts them inside that column
func MoveProjectBoardNoteOnProjectBoard(ctx context.Context, board *Board, sortedProjectBoardNoteIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		for sorting, issueID := range sortedProjectBoardNoteIDs {
			_, err := sess.Exec("UPDATE `project_board_note` SET board_id=?, sorting=? WHERE id=?", board.ID, sorting, issueID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func deleteProjectBoardNoteByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&ProjectBoardNote{})
	return err
}

// DeleteProjectBoardNote removes the BoardNote from the project board.
func DeleteProjectBoardNote(ctx context.Context, boardNote *ProjectBoardNote) error {
	if _, err := db.GetEngine(ctx).Delete(boardNote); err != nil {
		return err
	}
	return nil
}

// removeProjectBoardNotes sets the boardID to 0 for the board
func (b *Board) removeProjectBoardNotes(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `project_board_note` SET board_id = 0 WHERE board_id = ?", b.ID)
	return err
}
