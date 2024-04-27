// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

type (
	// BoardType is used to represent a project board type
	BoardType uint8

	// CardType is used to represent a project board card type
	CardType uint8

	// BoardList is a list of all project boards in a repository
	BoardList []*Board
)

const (
	// BoardTypeNone is a project board type that has no predefined columns
	BoardTypeNone BoardType = iota

	// BoardTypeBasicKanban is a project board type that has basic predefined columns
	BoardTypeBasicKanban

	// BoardTypeBugTriage is a project board type that has predefined columns suited to hunting down bugs
	BoardTypeBugTriage
)

const (
	// CardTypeTextOnly is a project board card type that is text only
	CardTypeTextOnly CardType = iota

	// CardTypeImagesAndText is a project board card type that has images and text
	CardTypeImagesAndText
)

// BoardColorPattern is a regexp witch can validate BoardColor
var BoardColorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

// Board is used to represent boards on a project
type Board struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool   `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific board will be assigned to this board
	Sorting int8   `xorm:"NOT NULL DEFAULT 0"`
	Color   string `xorm:"VARCHAR(7)"`

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName return the real table name
func (Board) TableName() string {
	return "project_board"
}

// NumIssues return counter of all issues assigned to the board
func (b *Board) NumIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Where("project_id=?", b.ProjectID).
		And("project_board_id=?", b.ID).
		GroupBy("issue_id").
		Cols("issue_id").
		Count()
	if err != nil {
		return 0
	}
	return int(c)
}

func init() {
	db.RegisterModel(new(Board))
}

// IsBoardTypeValid checks if the project board type is valid
func IsBoardTypeValid(p BoardType) bool {
	switch p {
	case BoardTypeNone, BoardTypeBasicKanban, BoardTypeBugTriage:
		return true
	default:
		return false
	}
}

// IsCardTypeValid checks if the project board card type is valid
func IsCardTypeValid(p CardType) bool {
	switch p {
	case CardTypeTextOnly, CardTypeImagesAndText:
		return true
	default:
		return false
	}
}

func createBoardsForProjectsType(ctx context.Context, project *Project) error {
	var items []string

	switch project.BoardType {
	case BoardTypeBugTriage:
		items = setting.Project.ProjectBoardBugTriageType

	case BoardTypeBasicKanban:
		items = setting.Project.ProjectBoardBasicKanbanType
	case BoardTypeNone:
		fallthrough
	default:
		return nil
	}

	board := Board{
		CreatedUnix: timeutil.TimeStampNow(),
		CreatorID:   project.CreatorID,
		Title:       "Backlog",
		ProjectID:   project.ID,
		Default:     true,
	}
	if err := db.Insert(ctx, board); err != nil {
		return err
	}

	if len(items) == 0 {
		return nil
	}

	boards := make([]Board, 0, len(items))

	for _, v := range items {
		boards = append(boards, Board{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
		})
	}

	return db.Insert(ctx, boards)
}

// NewBoard adds a new project board to a given project
func NewBoard(ctx context.Context, board *Board) error {
	if len(board.Color) != 0 && !BoardColorPattern.MatchString(board.Color) {
		return fmt.Errorf("bad color code: %s", board.Color)
	}

	_, err := db.GetEngine(ctx).Insert(board)
	return err
}

// DeleteBoardByID removes all issues references to the project board.
func DeleteBoardByID(ctx context.Context, boardID int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := deleteBoardByID(ctx, boardID); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteBoardByID(ctx context.Context, boardID int64) error {
	board, err := GetBoard(ctx, boardID)
	if err != nil {
		if IsErrProjectBoardNotExist(err) {
			return nil
		}

		return err
	}

	if board.Default {
		return fmt.Errorf("deleteBoardByID: cannot delete default board")
	}

	if err = board.removeIssues(ctx); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).ID(board.ID).NoAutoCondition().Delete(board); err != nil {
		return err
	}
	return nil
}

func deleteBoardByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&Board{})
	return err
}

// GetBoard fetches the current board of a project
func GetBoard(ctx context.Context, boardID int64) (*Board, error) {
	board := new(Board)
	has, err := db.GetEngine(ctx).ID(boardID).Get(board)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNotExist{BoardID: boardID}
	}

	return board, nil
}

// UpdateBoard updates a project board
func UpdateBoard(ctx context.Context, board *Board) error {
	var fieldToUpdate []string

	if board.Sorting != 0 {
		fieldToUpdate = append(fieldToUpdate, "sorting")
	}

	if board.Title != "" {
		fieldToUpdate = append(fieldToUpdate, "title")
	}

	if len(board.Color) != 0 && !BoardColorPattern.MatchString(board.Color) {
		return fmt.Errorf("bad color code: %s", board.Color)
	}
	fieldToUpdate = append(fieldToUpdate, "color")

	_, err := db.GetEngine(ctx).ID(board.ID).Cols(fieldToUpdate...).Update(board)

	return err
}

// GetBoards fetches all boards related to a project
func (p *Project) GetBoards(ctx context.Context) (BoardList, error) {
	boards := make([]*Board, 0, 5)

	if err := db.GetEngine(ctx).Where("project_id=? AND `default`=?", p.ID, false).OrderBy("sorting").Find(&boards); err != nil {
		return nil, err
	}

	defaultB, err := p.getDefaultBoard(ctx)
	if err != nil {
		return nil, err
	}

	return append([]*Board{defaultB}, boards...), nil
}

// getDefaultBoard return default board and ensure only one exists
func (p *Project) getDefaultBoard(ctx context.Context) (*Board, error) {
	var board Board
	has, err := db.GetEngine(ctx).
		Where("project_id=? AND `default` = ?", p.ID, true).
		Desc("id").Get(&board)
	if err != nil {
		return nil, err
	}

	if has {
		return &board, nil
	}

	// create a default board if none is found
	board = Board{
		ProjectID: p.ID,
		Default:   true,
		Title:     "Uncategorized",
		CreatorID: p.CreatorID,
	}
	if _, err := db.GetEngine(ctx).Insert(&board); err != nil {
		return nil, err
	}
	return &board, nil
}

// SetDefaultBoard represents a board for issues not assigned to one
func SetDefaultBoard(ctx context.Context, projectID, boardID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := GetBoard(ctx, boardID); err != nil {
			return err
		}

		if _, err := db.GetEngine(ctx).Where(builder.Eq{
			"project_id": projectID,
			"`default`":  true,
		}).Cols("`default`").Update(&Board{Default: false}); err != nil {
			return err
		}

		_, err := db.GetEngine(ctx).ID(boardID).
			Where(builder.Eq{"project_id": projectID}).
			Cols("`default`").Update(&Board{Default: true})
		return err
	})
}

// UpdateBoardSorting update project board sorting
func UpdateBoardSorting(ctx context.Context, bs BoardList) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for i := range bs {
			if _, err := db.GetEngine(ctx).ID(bs[i].ID).Cols(
				"sorting",
			).Update(bs[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
