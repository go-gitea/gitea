// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type (

	// CardType is used to represent a project column card type
	CardType uint8

	// ColumnList is a list of all project columns in a repository
	ColumnList []*Column
)

const (
	// CardTypeTextOnly is a project column card type that is text only
	CardTypeTextOnly CardType = iota

	// CardTypeImagesAndText is a project column card type that has images and text
	CardTypeImagesAndText
)

// ColumnColorPattern is a regexp witch can validate ColumnColor
var ColumnColorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

// Column is used to represent column on a project
type Column struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool   `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific column will be assigned to this column
	Sorting int8   `xorm:"NOT NULL DEFAULT 0"`
	Color   string `xorm:"VARCHAR(7)"`

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	NumIssues int64 `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName return the real table name
func (Column) TableName() string {
	return "project_board" // TODO: the legacy table name should be project_column
}

func (c *Column) GetIssues(ctx context.Context) ([]*ProjectIssue, error) {
	issues := make([]*ProjectIssue, 0, 5)
	if err := db.GetEngine(ctx).Where("project_id=?", c.ProjectID).
		And("project_board_id=?", c.ID).
		OrderBy("sorting, id").
		Find(&issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func init() {
	db.RegisterModel(new(Column))
}

// IsCardTypeValid checks if the project column card type is valid
func IsCardTypeValid(p CardType) bool {
	switch p {
	case CardTypeTextOnly, CardTypeImagesAndText:
		return true
	default:
		return false
	}
}

func createDefaultColumnsForProject(ctx context.Context, project *Project) error {
	var items []string

	switch project.TemplateType {
	case TemplateTypeBugTriage:
		items = setting.Project.ProjectBoardBugTriageType
	case TemplateTypeBasicKanban:
		items = setting.Project.ProjectBoardBasicKanbanType
	case TemplateTypeNone:
		fallthrough
	default:
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		column := Column{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       "Backlog",
			ProjectID:   project.ID,
			Default:     true,
		}
		if err := db.Insert(ctx, column); err != nil {
			return err
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
	})
}

// maxProjectColumns max columns allowed in a project, this should not bigger than 127
// because sorting is int8 in database
const maxProjectColumns = 20

// NewColumn adds a new project column to a given project
func NewColumn(ctx context.Context, column *Column) error {
	if len(column.Color) != 0 && !ColumnColorPattern.MatchString(column.Color) {
		return fmt.Errorf("bad color code: %s", column.Color)
	}

	res := struct {
		MaxSorting  int64
		ColumnCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as column_count").Table("project_board").
		Where("project_id=?", column.ProjectID).Get(&res); err != nil {
		return err
	}
	if res.ColumnCount >= maxProjectColumns {
		return fmt.Errorf("NewBoard: maximum number of columns reached")
	}
	column.Sorting = int8(util.Iif(res.ColumnCount > 0, res.MaxSorting+1, 0))
	_, err := db.GetEngine(ctx).Insert(column)
	return err
}

// DeleteColumnByID removes all issues references to the project column.
func DeleteColumnByID(ctx context.Context, columnID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		return deleteColumnByID(ctx, columnID)
	})
}

func deleteColumnByID(ctx context.Context, columnID int64) error {
	column, err := GetColumn(ctx, columnID)
	if err != nil {
		if IsErrProjectColumnNotExist(err) {
			return nil
		}

		return err
	}

	if column.Default {
		return fmt.Errorf("deleteColumnByID: cannot delete default column")
	}

	// move all issues to the default column
	project, err := GetProjectByID(ctx, column.ProjectID)
	if err != nil {
		return err
	}
	defaultColumn, err := project.MustDefaultColumn(ctx)
	if err != nil {
		return err
	}

	if err = column.moveIssuesToAnotherColumn(ctx, defaultColumn); err != nil {
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
func (p *Project) GetColumns(ctx context.Context) (ColumnList, error) {
	columns := make([]*Column, 0, 5)
	if err := db.GetEngine(ctx).Where("project_id=?", p.ID).OrderBy("sorting, id").Find(&columns); err != nil {
		return nil, err
	}

	return columns, nil
}

// getDefaultColumn return default column and ensure only one exists
func (p *Project) getDefaultColumn(ctx context.Context) (*Column, error) {
	var column Column
	has, err := db.GetEngine(ctx).
		Where("project_id=? AND `default` = ?", p.ID, true).
		Desc("id").Get(&column)
	if err != nil {
		return nil, err
	}

	if has {
		return &column, nil
	}
	return nil, ErrProjectColumnNotExist{ColumnID: 0}
}

// MustDefaultColumn returns the default column for a project.
// If one exists, it is returned
// If none exists, the first column will be elevated to the default column of this project
func (p *Project) MustDefaultColumn(ctx context.Context) (*Column, error) {
	c, err := p.getDefaultColumn(ctx)
	if err != nil && !IsErrProjectColumnNotExist(err) {
		return nil, err
	}
	if c != nil {
		return c, nil
	}

	var column Column
	has, err := db.GetEngine(ctx).Where("project_id=?", p.ID).OrderBy("sorting, id").Get(&column)
	if err != nil {
		return nil, err
	}
	if has {
		column.Default = true
		if _, err := db.GetEngine(ctx).ID(column.ID).Cols("`default`").Update(&column); err != nil {
			return nil, err
		}
		return &column, nil
	}

	// create a default column if none is found
	column = Column{
		ProjectID: p.ID,
		Default:   true,
		Title:     "Uncategorized",
		CreatorID: p.CreatorID,
	}
	if _, err := db.GetEngine(ctx).Insert(&column); err != nil {
		return nil, err
	}
	return &column, nil
}

// SetDefaultColumn represents a column for issues not assigned to one
func SetDefaultColumn(ctx context.Context, projectID, columnID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := GetColumn(ctx, columnID); err != nil {
			return err
		}

		if _, err := db.GetEngine(ctx).Where(builder.Eq{
			"project_id": projectID,
			"`default`":  true,
		}).Cols("`default`").Update(&Column{Default: false}); err != nil {
			return err
		}

		_, err := db.GetEngine(ctx).ID(columnID).
			Where(builder.Eq{"project_id": projectID}).
			Cols("`default`").Update(&Column{Default: true})
		return err
	})
}

// UpdateColumnSorting update project column sorting
func UpdateColumnSorting(ctx context.Context, cl ColumnList) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for i := range cl {
			if _, err := db.GetEngine(ctx).ID(cl[i].ID).Cols(
				"sorting",
			).Update(cl[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func GetColumnsByIDs(ctx context.Context, projectID int64, columnsIDs []int64) (ColumnList, error) {
	columns := make([]*Column, 0, 5)
	if err := db.GetEngine(ctx).
		Where("project_id =?", projectID).
		In("id", columnsIDs).
		OrderBy("sorting").Find(&columns); err != nil {
		return nil, err
	}
	return columns, nil
}

// MoveColumnsOnProject sorts columns in a project
func MoveColumnsOnProject(ctx context.Context, project *Project, sortedColumnIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)
		columnIDs := util.ValuesOfMap(sortedColumnIDs)
		movedColumns, err := GetColumnsByIDs(ctx, project.ID, columnIDs)
		if err != nil {
			return err
		}
		if len(movedColumns) != len(sortedColumnIDs) {
			return errors.New("some columns do not exist")
		}

		for _, column := range movedColumns {
			if column.ProjectID != project.ID {
				return fmt.Errorf("column[%d]'s projectID is not equal to project's ID [%d]", column.ProjectID, project.ID)
			}
		}

		for sorting, columnID := range sortedColumnIDs {
			if _, err := sess.Exec("UPDATE `project_board` SET sorting=? WHERE id=?", sorting, columnID); err != nil {
				return err
			}
		}
		return nil
	})
}
