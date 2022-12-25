// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

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
		ColumnType  BoardType
		Translation string
	}

	// Type is used to identify the type of project in question and ownership
	Type uint8

	// ColumnType is used to represent a project board type
	BoardType uint8
)

const (
	// TypeIndividual is a type of project board that is owned by an individual
	TypeIndividual Type = iota + 1

	// TypeRepository is a project that is tied to a repository
	TypeRepository

	// TypeOrganization is a project that is tied to an organisation
	TypeOrganization
)

const (
	// BoardTypeNone is a board type that has no predefined columns
	BoardTypeNone BoardType = iota

	// ColumnTypeBasicKanban is a project board type that has basic predefined columns
	BoardTypeBasicKanban

	// ColumnTypeBugTriage is a project board type that has predefined columns suited to hunting down bugs
	BoardTypeBugTriage
)

// IsBoardTypeValid checks if the project board type is valid
func IsBoardTypeValid(p BoardType) bool {
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
	ColumnType  BoardType
	Type        Type

	RenderedContent string `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix timeutil.TimeStamp
}

func init() {
	db.RegisterModel(new(Board))
}

// GetBoardsConfig retrieves the types of configurations projects could have
func GetBoardsConfig() []BoardsConfig {
	return []BoardsConfig{
		{BoardTypeNone, "repo.projects.type.none"},
		{BoardTypeBasicKanban, "repo.projects.type.basic_kanban"},
		{BoardTypeBugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsTypeValid checks if a project type is valid
func IsTypeValid(p Type) bool {
	switch p {
	case TypeRepository:
		return true
	default:
		return false
	}
}

// SearchOptions are options for GetProjects
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
	projects := make([]*Board, 0, setting.UI.IssuePagingNum)

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

	return projects, count, e.Find(&projects)
}

// NewBoard creates a new board
func NewBoard(p *Board) error {
	if !IsBoardTypeValid(p.ColumnType) {
		p.ColumnType = BoardTypeNone
	}

	if !IsTypeValid(p.Type) {
		return errors.New("project type is not valid")
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := db.Insert(ctx, p); err != nil {
		return err
	}

	if _, err := db.Exec(ctx, "UPDATE `repository` SET num_projects = num_projects + 1 WHERE id = ?", p.RepoID); err != nil {
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
			"`num_projects`": builder.Select("count(*)").From("`project`").
				Where(builder.Eq{"`project`.`repo_id`": repoID}.
					And(builder.Eq{"`project`.`type`": TypeRepository})),
		}).From("`repository`").Where(builder.Eq{"id": repoID})); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).Exec(builder.Update(
		builder.Eq{
			"`num_closed_projects`": builder.Select("count(*)").From("`project`").
				Where(builder.Eq{"`project`.`repo_id`": repoID}.
					And(builder.Eq{"`project`.`type`": TypeRepository}).
					And(builder.Eq{"`project`.`is_closed`": true})),
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

// DeleteBoardByID deletes a project from a repository. if it's not in a database
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
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_issue WHERE project_issue.id IN (SELECT project_issue.id FROM project_issue INNER JOIN project WHERE project.id = project_issue.project_id AND project.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_board WHERE project_board.id IN (SELECT project_board.id FROM project_board INNER JOIN project WHERE project.id = project_board.project_id AND project.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	case setting.Database.UsePostgreSQL:
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_issue USING project WHERE project.id = project_issue.project_id AND project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_board USING project WHERE project.id = project_board.project_id AND project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	default:
		if _, err := db.GetEngine(ctx).Exec("DELETE project_issue FROM project_issue INNER JOIN project ON project.id = project_issue.project_id WHERE project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE project_board FROM project_board INNER JOIN project ON project.id = project_board.project_id WHERE project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Board{}); err != nil {
			return err
		}
	}

	return updateRepositoryBoardCount(ctx, repoID)
}
