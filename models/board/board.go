// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package board

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type (
	// BoardsConfig is used to identify the type of board that is being created
	BoardsConfig struct {
		ColumnType  ColumnType
		Translation string
	}

	// Type is used to identify the type of board in question and ownership
	Type uint8

	// ColumnType is used to represent a board board type
	ColumnType uint8
)

const (
	// TypeIndividual is a type of board board that is owned by an individual
	TypeIndividual Type = iota + 1

	// TypeRepository is a board that is tied to a repository
	TypeRepository

	// TypeOrganization is a board that is tied to an organisation
	TypeOrganization
)

const (
	// BoardTypeNone is a board type that has no predefined columns
	BoardTypeNone ColumnType = iota

	// ColumnTypeBasicKanban is a board board type that has basic predefined columns
	BoardTypeBasicKanban

	// ColumnTypeBugTriage is a board board type that has predefined columns suited to hunting down bugs
	BoardTypeBugTriage
)

// IsBoardTypeValid checks if the board board type is valid
func IsBoardTypeValid(p ColumnType) bool {
	switch p {
	case BoardTypeNone, BoardTypeBasicKanban, BoardTypeBugTriage:
		return true
	default:
		return false
	}
}

// ErrBoardNotExist represents a "BoardNotExist" kind of error.
type ErrBoardNotExist struct {
	ID     int64
	RepoID int64
}

// IsErrBoardNotExist checks if an error is a ErrBoardNotExist
func IsErrBoardNotExist(err error) bool {
	_, ok := err.(ErrBoardNotExist)
	return ok
}

func (err ErrBoardNotExist) Error() string {
	return fmt.Sprintf("board does not exist [id: %d]", err.ID)
}

func (err ErrBoardNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Board represents a board
type Board struct {
	ID          int64  `xorm:"pk autoincr"`
	Title       string `xorm:"INDEX NOT NULL"`
	Description string `xorm:"TEXT"`
	RepoID      int64  `xorm:"INDEX"`
	CreatorID   int64  `xorm:"NOT NULL"`
	IsClosed    bool   `xorm:"INDEX"`
	ColumnType  ColumnType
	Type        Type

	RenderedContent string `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix timeutil.TimeStamp
}

func init() {
	db.RegisterModel(new(Board))
}

// GetBoardsConfig retrieves the types of configurations boards could have
func GetBoardsConfig() []BoardsConfig {
	return []BoardsConfig{
		{BoardTypeNone, "repo.projects.type.none"},
		{BoardTypeBasicKanban, "repo.projects.type.basic_kanban"},
		{BoardTypeBugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsTypeValid checks if a board type is valid
func IsTypeValid(p Type) bool {
	switch p {
	case TypeRepository:
		return true
	default:
		return false
	}
}

// SearchOptions are options for FindBoards
type SearchOptions struct {
	RepoID   int64
	Page     int
	IsClosed util.OptionalBool
	SortType string
	Type     Type
}

// FindBoards returns a list of all boards that have been created in the repository
func FindBoards(ctx context.Context, opts SearchOptions) ([]*Board, int64, error) {
	e := db.GetEngine(ctx)
	boards := make([]*Board, 0, setting.UI.IssuePagingNum)

	var cond builder.Cond = builder.Eq{"repo_id": opts.RepoID}
	switch opts.IsClosed {
	case util.OptionalBoolTrue:
		cond = cond.And(builder.Eq{"is_closed": true})
	case util.OptionalBoolFalse:
		cond = cond.And(builder.Eq{"is_closed": false})
	}

	if opts.Type > 0 {
		cond = cond.And(builder.Eq{"type": opts.Type})
	}

	count, err := e.Where(cond).Count(new(Board))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %w", err)
	}

	e = e.Where(cond)

	if opts.Page > 0 {
		e = e.Limit(setting.UI.IssuePagingNum, (opts.Page-1)*setting.UI.IssuePagingNum)
	}

	switch opts.SortType {
	case "oldest":
		e.Desc("created_unix")
	case "recentupdate":
		e.Desc("updated_unix")
	case "leastupdate":
		e.Asc("updated_unix")
	default:
		e.Asc("created_unix")
	}

	return boards, count, e.Find(&boards)
}

// NewBoard creates a new board
func NewBoard(p *Board) error {
	if !IsBoardTypeValid(p.ColumnType) {
		p.ColumnType = BoardTypeNone
	}

	if !IsTypeValid(p.Type) {
		return errors.New("board type is not valid")
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := db.Insert(ctx, p); err != nil {
		return err
	}

	if _, err := db.Exec(ctx, "UPDATE `repository` SET num_boards = num_boards + 1 WHERE id = ?", p.RepoID); err != nil {
		return err
	}

	if err := createColumnsForBoardType(ctx, p); err != nil {
		return err
	}

	return committer.Commit()
}

// GetBoardByID returns the borad in a repository
func GetBoardByID(ctx context.Context, id int64) (*Board, error) {
	p := new(Board)

	has, err := db.GetEngine(ctx).ID(id).Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBoardNotExist{ID: id}
	}

	return p, nil
}

// UpdateBoard updates board properties
func UpdateBoard(ctx context.Context, p *Board) error {
	_, err := db.GetEngine(ctx).ID(p.ID).Cols(
		"title",
		"description",
	).Update(p)
	return err
}

func updateRepositoryBoardCount(ctx context.Context, repoID int64) error {
	if _, err := db.GetEngine(ctx).Exec(builder.Update(
		builder.Eq{
			"`num_boards`": builder.Select("count(*)").From("`board`").
				Where(builder.Eq{"`board`.`repo_id`": repoID}.
					And(builder.Eq{"`board`.`type`": TypeRepository})),
		}).From("`repository`").Where(builder.Eq{"id": repoID})); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).Exec(builder.Update(
		builder.Eq{
			"`num_closed_boards`": builder.Select("count(*)").From("`board`").
				Where(builder.Eq{"`board`.`repo_id`": repoID}.
					And(builder.Eq{"`board`.`type`": TypeRepository}).
					And(builder.Eq{"`board`.`is_closed`": true})),
		}).From("`repository`").Where(builder.Eq{"id": repoID})); err != nil {
		return err
	}
	return nil
}

// ChangeBoardStatusByRepoIDAndID toggles a board between opened and closed
func ChangeBoardStatusByRepoIDAndID(repoID, boardID int64, isClosed bool) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	p := new(Board)

	has, err := db.GetEngine(ctx).ID(boardID).Where("repo_id = ?", repoID).Get(p)
	if err != nil {
		return err
	} else if !has {
		return ErrBoardNotExist{ID: boardID, RepoID: repoID}
	}

	if err := changeBoardStatus(ctx, p, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

// ChangeBoardStatus toggle a board between opened and closed
func ChangeBoardStatus(p *Board, isClosed bool) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := changeBoardStatus(ctx, p, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

func changeBoardStatus(ctx context.Context, p *Board, isClosed bool) error {
	p.IsClosed = isClosed
	p.ClosedDateUnix = timeutil.TimeStampNow()
	count, err := db.GetEngine(ctx).ID(p.ID).Where("repo_id = ? AND is_closed = ?", p.RepoID, !isClosed).Cols("is_closed", "closed_date_unix").Update(p)
	if err != nil {
		return err
	}
	if count < 1 {
		return nil
	}

	return updateRepositoryBoardCount(ctx, p.RepoID)
}

// DeleteBoardByID deletes a board from a repository. if it's not in a database
// transaction, it will start a new database transaction
func DeleteBoardByID(ctx context.Context, id int64) error {
	return db.AutoTx(ctx, func(ctx context.Context) error {
		p, err := GetBoardByID(ctx, id)
		if err != nil {
			if IsErrBoardNotExist(err) {
				return nil
			}
			return err
		}

		if err := deleteBoardIssuesByBoardID(ctx, id); err != nil {
			return err
		}

		if err := deleteColumnsByBoardID(ctx, id); err != nil {
			return err
		}

		if _, err = db.GetEngine(ctx).ID(p.ID).Delete(new(Board)); err != nil {
			return err
		}

		return updateRepositoryBoardCount(ctx, p.RepoID)
	})
}

func DeleteBoardByRepoID(ctx context.Context, repoID int64) error {
	switch {
	case setting.Database.UseSQLite3:
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM board_issue WHERE board_issue.id IN (SELECT board_issue.id FROM board_issue INNER JOIN board WHERE board.id = board_issue.board_id AND board.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM board_column WHERE board_column.id IN (SELECT board_column.id FROM board_column INNER JOIN board WHERE board.id = board_column.board_id AND board.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("board").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	case setting.Database.UsePostgreSQL:
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM board_issue USING board WHERE board.id = board_issue.board_id AND board.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM board_column USING board WHERE board.id = board_column.board_id AND board.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("board").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	default:
		if _, err := db.GetEngine(ctx).Exec("DELETE board_issue FROM board_issue INNER JOIN board ON board.id = board_issue.board_id WHERE board.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE board_column FROM board_column INNER JOIN board ON board.id = board_column.board_id WHERE board.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("board").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	}

	return updateRepositoryBoardCount(ctx, repoID)
}
