// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package board

import (
	"context"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ColumnColorPattern is a regexp witch can validate BoardColor
var ColumnColorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

// ErrColumnNotExist represents a "ColumnNotExist" kind of error.
type ErrColumnNotExist struct {
	ColumnID int64
}

// IsErrColumnNotExist checks if an error is a ErrColumnNotExist
func IsErrColumnNotExist(err error) bool {
	_, ok := err.(ErrColumnNotExist)
	return ok
}

func (err ErrColumnNotExist) Error() string {
	return fmt.Sprintf("board column does not exist [id: %d]", err.ColumnID)
}

func (err ErrColumnNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Column is used to represent columns on a board
type Column struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool   `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific board will be assigned to this board
	Sorting int8   `xorm:"NOT NULL DEFAULT 0"`
	Color   string `xorm:"VARCHAR(7)"`

	BoardID   int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName return the real table name
func (Column) TableName() string {
	return "board_column"
}

// NumIssues return counter of all issues assigned to the board
func (b *Column) NumIssues() int {
	c, err := db.GetEngine(db.DefaultContext).Table("board_issue").
		Where("board_id=?", b.BoardID).
		And("board_column_id=?", b.ID).
		GroupBy("issue_id").
		Cols("issue_id").
		Count()
	if err != nil {
		return 0
	}
	return int(c)
}

func init() {
	db.RegisterModel(new(Column))
}

func createColumnsForBoardType(ctx context.Context, board *Board) error {
	var items []string

	switch board.ColumnType {

	case BoardTypeBugTriage:
		items = setting.Board.ProjectBoardBugTriageType

	case BoardTypeBasicKanban:
		items = setting.Board.ProjectBoardBasicKanbanType

	case BoardTypeNone:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	columns := make([]Column, 0, len(items))

	for _, v := range items {
		columns = append(columns, Column{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   board.CreatorID,
			Title:       v,
			ID:          board.ID,
		})
	}

	return db.Insert(ctx, columns)
}

// NewColumn adds a new board column to a given board
func NewColumn(board *Column) error {
	if len(board.Color) != 0 && !ColumnColorPattern.MatchString(board.Color) {
		return fmt.Errorf("bad color code: %s", board.Color)
	}

	_, err := db.GetEngine(db.DefaultContext).Insert(board)
	return err
}

// DeleteColumnByID removes all issues references to the board column.
func DeleteColumnByID(boardID int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := deleteColumnByID(ctx, boardID); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteColumnByID(ctx context.Context, columnID int64) error {
	column, err := GetColumn(ctx, columnID)
	if err != nil {
		if IsErrColumnNotExist(err) {
			return nil
		}

		return err
	}

	if err = column.removeIssues(ctx); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).ID(column.ID).NoAutoCondition().Delete(column); err != nil {
		return err
	}
	return nil
}

func deleteColumnsByBoardID(ctx context.Context, boardID int64) error {
	_, err := db.GetEngine(ctx).Where("board_id=?", boardID).Delete(&Column{})
	return err
}

// GetColumn fetches the current column of a board
func GetColumn(ctx context.Context, columnID int64) (*Column, error) {
	column := new(Column)

	has, err := db.GetEngine(ctx).ID(columnID).Get(column)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrColumnNotExist{ColumnID: columnID}
	}

	return column, nil
}

// UpdateColumn updates a board column
func UpdateColumn(ctx context.Context, column *Column) error {
	var fieldToUpdate []string

	if column.Sorting != 0 {
		fieldToUpdate = append(fieldToUpdate, "sorting")
	}

	if column.Title != "" {
		fieldToUpdate = append(fieldToUpdate, "title")
	}

	if len(column.Color) != 0 && !ColumnColorPattern.MatchString(column.Color) {
		return fmt.Errorf("bad color code: %s", column.Color)
	}
	fieldToUpdate = append(fieldToUpdate, "color")

	_, err := db.GetEngine(ctx).ID(column.ID).Cols(fieldToUpdate...).Update(column)

	return err
}

// ColumnList is a list of all board columns in a repository
type ColumnList []*Column

// FindColumns fetches all columns related to a board
// if no default board set, first board is a temporary "Uncategorized" board
func FindColumns(ctx context.Context, boardID int64) (ColumnList, error) {
	columns := make([]*Column, 0, 5)

	if err := db.GetEngine(ctx).Where("board_id=? AND `default`=?", boardID, false).OrderBy("Sorting").Find(&columns); err != nil {
		return nil, err
	}

	defaultB, err := getDefaultColumn(ctx, boardID)
	if err != nil {
		return nil, err
	}

	return append([]*Column{defaultB}, columns...), nil
}

// getDefaultColumn return default column and create a dummy if none exist
func getDefaultColumn(ctx context.Context, boardID int64) (*Column, error) {
	var board Column
	exist, err := db.GetEngine(ctx).Where("board_id=? AND `default`=?", boardID, true).Get(&board)
	if err != nil {
		return nil, err
	}
	if exist {
		return &board, nil
	}

	// represents a board for issues not assigned to one
	return &Column{
		BoardID: boardID,
		Title:   "Uncategorized",
		Default: true,
	}, nil
}

// SetDefaultColumn represents a board for issues not assigned to one
// if boardID is 0 unset default
func SetDefaultColumn(boardID, columnID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Where(builder.Eq{
		"board_id":  boardID,
		"`default`": true,
	}).Cols("`default`").Update(&Column{Default: false})
	if err != nil {
		return err
	}

	if boardID > 0 {
		_, err = db.GetEngine(db.DefaultContext).ID(boardID).Where(builder.Eq{"board_id": boardID}).
			Cols("`default`").Update(&Column{Default: true})
	}

	return err
}

// UpdateColumnSorting update board column sorting
func UpdateColumnSorting(bs ColumnList) error {
	for i := range bs {
		_, err := db.GetEngine(db.DefaultContext).ID(bs[i].ID).Cols(
			"sorting",
		).Update(bs[i])
		if err != nil {
			return err
		}
	}
	return nil
}
