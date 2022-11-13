// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	// ProjectsConfig is used to identify the type of board that is being created
	ProjectsConfig struct {
		BoardType   BoardType
		Translation string
	}

	// Type is used to identify the type of project in question and ownership
	Type uint8
)

const (
	// TypeIndividual is a type of project board that is owned by an individual
	TypeIndividual Type = iota + 1

	// TypeRepository is a project that is tied to a repository
	TypeRepository

	// TypeOrganization is a project that is tied to an organisation
	TypeOrganization
)

// ErrProjectNotExist represents a "ProjectNotExist" kind of error.
type ErrProjectNotExist struct {
	ID     int64
	RepoID int64
}

// IsErrProjectNotExist checks if an error is a ErrProjectNotExist
func IsErrProjectNotExist(err error) bool {
	_, ok := err.(ErrProjectNotExist)
	return ok
}

func (err ErrProjectNotExist) Error() string {
	return fmt.Sprintf("projects does not exist [id: %d]", err.ID)
}

func (err ErrProjectNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrProjectBoardNotExist represents a "ProjectBoardNotExist" kind of error.
type ErrProjectBoardNotExist struct {
	BoardID int64
}

// IsErrProjectBoardNotExist checks if an error is a ErrProjectBoardNotExist
func IsErrProjectBoardNotExist(err error) bool {
	_, ok := err.(ErrProjectBoardNotExist)
	return ok
}

func (err ErrProjectBoardNotExist) Error() string {
	return fmt.Sprintf("project board does not exist [id: %d]", err.BoardID)
}

func (err ErrProjectBoardNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Project represents a project board
type Project struct {
	ID          int64  `xorm:"pk autoincr"`
	Title       string `xorm:"INDEX NOT NULL"`
	Description string `xorm:"TEXT"`
	RepoID      int64  `xorm:"INDEX"`
	CreatorID   int64  `xorm:"NOT NULL"`
	IsClosed    bool   `xorm:"INDEX"`
	BoardType   BoardType
	Type        Type

	RenderedContent string `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix timeutil.TimeStamp
}

func init() {
	db.RegisterModel(new(Project))
}

// GetProjectsConfig retrieves the types of configurations projects could have
func GetProjectsConfig() []ProjectsConfig {
	return []ProjectsConfig{
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

// GetProjects returns a list of all projects that have been created in the repository
func GetProjects(ctx context.Context, opts SearchOptions) ([]*Project, int64, error) {
	e := db.GetEngine(ctx)
	projects := make([]*Project, 0, setting.UI.IssuePagingNum)

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

	count, err := e.Where(cond).Count(new(Project))
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

// NewProject creates a new Project
func NewProject(p *Project) error {
	if !IsBoardTypeValid(p.BoardType) {
		p.BoardType = BoardTypeNone
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

	if err := createBoardsForProjectsType(ctx, p); err != nil {
		return err
	}

	return committer.Commit()
}

// GetProjectByID returns the projects in a repository
func GetProjectByID(ctx context.Context, id int64) (*Project, error) {
	p := new(Project)

	has, err := db.GetEngine(ctx).ID(id).Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectNotExist{ID: id}
	}

	return p, nil
}

// UpdateProject updates project properties
func UpdateProject(ctx context.Context, p *Project) error {
	_, err := db.GetEngine(ctx).ID(p.ID).Cols(
		"title",
		"description",
	).Update(p)
	return err
}

func updateRepositoryProjectCount(ctx context.Context, repoID int64) error {
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

// ChangeProjectStatusByRepoIDAndID toggles a project between opened and closed
func ChangeProjectStatusByRepoIDAndID(repoID, projectID int64, isClosed bool) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	p := new(Project)

	has, err := db.GetEngine(ctx).ID(projectID).Where("repo_id = ?", repoID).Get(p)
	if err != nil {
		return err
	} else if !has {
		return ErrProjectNotExist{ID: projectID, RepoID: repoID}
	}

	if err := changeProjectStatus(ctx, p, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

// ChangeProjectStatus toggle a project between opened and closed
func ChangeProjectStatus(p *Project, isClosed bool) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := changeProjectStatus(ctx, p, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

func changeProjectStatus(ctx context.Context, p *Project, isClosed bool) error {
	p.IsClosed = isClosed
	p.ClosedDateUnix = timeutil.TimeStampNow()
	count, err := db.GetEngine(ctx).ID(p.ID).Where("repo_id = ? AND is_closed = ?", p.RepoID, !isClosed).Cols("is_closed", "closed_date_unix").Update(p)
	if err != nil {
		return err
	}
	if count < 1 {
		return nil
	}

	return updateRepositoryProjectCount(ctx, p.RepoID)
}

// DeleteProjectByID deletes a project from a repository.
func DeleteProjectByID(id int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := DeleteProjectByIDCtx(ctx, id); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteProjectByIDCtx deletes a project from a repository.
func DeleteProjectByIDCtx(ctx context.Context, id int64) error {
	p, err := GetProjectByID(ctx, id)
	if err != nil {
		if IsErrProjectNotExist(err) {
			return nil
		}
		return err
	}

	if err := deleteProjectIssuesByProjectID(ctx, id); err != nil {
		return err
	}

	if err := deleteBoardByProjectID(ctx, id); err != nil {
		return err
	}

	if _, err = db.GetEngine(ctx).ID(p.ID).Delete(new(Project)); err != nil {
		return err
	}

	return updateRepositoryProjectCount(ctx, p.RepoID)
}

func DeleteProjectByRepoIDCtx(ctx context.Context, repoID int64) error {
	switch {
	case setting.Database.UseSQLite3:
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_issue WHERE project_issue.id IN (SELECT project_issue.id FROM project_issue INNER JOIN project WHERE project.id = project_issue.project_id AND project.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_board WHERE project_board.id IN (SELECT project_board.id FROM project_board INNER JOIN project WHERE project.id = project_board.project_id AND project.repo_id = ?)", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Project{}); err != nil {
			return err
		}
	case setting.Database.UsePostgreSQL:
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_issue USING project WHERE project.id = project_issue.project_id AND project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE FROM project_board USING project WHERE project.id = project_board.project_id AND project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Project{}); err != nil {
			return err
		}
	default:
		if _, err := db.GetEngine(ctx).Exec("DELETE project_issue FROM project_issue INNER JOIN project ON project.id = project_issue.project_id WHERE project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("DELETE project_board FROM project_board INNER JOIN project ON project.id = project_board.project_id WHERE project.repo_id = ? ", repoID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Table("project").Where("repo_id = ? ", repoID).Delete(&Project{}); err != nil {
			return err
		}
	}

	return updateRepositoryProjectCount(ctx, repoID)
}
