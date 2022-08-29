// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

	// Columns is a list of all project columns in a repository
	Columns []*Column
)

const (
	// BoardTypeNone is a project board type that has no predefined columns
	BoardTypeNone BoardType = iota

	// BoardTypeBasicKanban is a project Column type that has basic predefined columns
	BoardTypeBasicKanban

	// BoardTypeBugTriage is a project type that has predefined columns suited to hunting down bugs
	BoardTypeBugTriage
)

// ColumnColorPattern is a regexp witch can validate ColumnColor
var ColumnColorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

// Column is used to represent columns on a project
type Column struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool   `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific column will be assigned to this column
	Sorting int8   `xorm:"NOT NULL DEFAULT 0"`
	Color   string `xorm:"VARCHAR(7)"`

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName return the real table name
func (Column) TableName() string {
	return "project_column"
}

// NumIssues return counter of all issues assigned to the column
func (b *Column) NumIssues() int {
	c, err := db.GetEngine(db.DefaultContext).Table("project_issue").
		Where("project_id=?", b.ProjectID).
		And("project_column_id=?", b.ID).
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

// IsColumnTypeValid checks if the project board type is valid
func IsBoardTypeValid(p BoardType) bool {
	switch p {
	case BoardTypeNone, BoardTypeBasicKanban, BoardTypeBugTriage:
		return true
	default:
		return false
	}
}

func createColumnsForProjectsType(ctx context.Context, project *Project) error {
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

	if len(items) == 0 {
		return nil
	}

	columns := make([]Column, 0, len(items))

	for _, v := range items {
		columns = append(columns, Column{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
		})
	}

	return db.Insert(ctx, columns)
}

// NewColumn adds a new column to a given project
func NewColumn(column *Column) error {
	if len(column.Color) != 0 && !ColumnColorPattern.MatchString(column.Color) {
		return fmt.Errorf("bad color code: %s", column.Color)
	}

	_, err := db.GetEngine(db.DefaultContext).Insert(column)
	return err
}

// DeleteColumnByID removes all issues references to the column
func DeleteColumnByID(columnID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := deleteColumnByID(ctx, columnID); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteColumnByID(ctx context.Context, columnID int64) error {
	column, err := GetColumn(ctx, columnID)
	if err != nil {
		if IsErrProjectColumnNotExist(err) {
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

func deleteColumnByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&Column{})
	return err
}

// GetColumn fetches the current column of a project
func GetColumn(ctx context.Context, columnID int64) (*Column, error) {
	column := new(Column)

	has, err := db.GetEngine(ctx).ID(columnID).Get(column)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectColumnNotExist{ColumnID: columnID}
	}

	return column, nil
}

// UpdateColumn updates a project column
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

// GetColumns fetches all columns related to a project
// if no default column is set, first column is a temporary "Uncategorized" column
func GetColumns(ctx context.Context, projectID int64) (Columns, error) {
	columns := make([]*Column, 0, 5)

	if err := db.GetEngine(ctx).Where("project_id=? AND `default`=?", projectID, false).OrderBy("Sorting").Find(&columns); err != nil {
		return nil, err
	}

	defaultB, err := getDefaultColumn(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return append([]*Column{defaultB}, columns...), nil
}

// getDefaultColumn return default column and create a dummy if none exist
func getDefaultColumn(ctx context.Context, projectID int64) (*Column, error) {
	var column Column
	exist, err := db.GetEngine(ctx).Where("project_id=? AND `default`=?", projectID, true).Get(&column)
	if err != nil {
		return nil, err
	}
	if exist {
		return &column, nil
	}

	// represents a column for issues not assigned to one
	return &Column{
		ProjectID: projectID,
		Title:     "Uncategorized",
		Default:   true,
	}, nil
}

// SetDefaultColumn represents a column for issues not assigned to one
// if columnID is 0 unset default
func SetDefaultColumn(projectID, columnID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Where(builder.Eq{
		"project_id": projectID,
		"`default`":  true,
	}).Cols("`default`").Update(&Column{Default: false})
	if err != nil {
		return err
	}

	if columnID > 0 {
		_, err = db.GetEngine(db.DefaultContext).ID(columnID).Where(builder.Eq{"project_id": projectID}).
			Cols("`default`").Update(&Column{Default: true})
	}

	return err
}

// UpdateCoumnSorting update column sorting
func UpdateColumnSorting(bs Columns) error {
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
