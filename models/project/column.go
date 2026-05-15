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
	"xorm.io/xorm/schemas"
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

// TableIndices declares the unique constraint on (project_id, sorting).
// Use a naked index name so xorm prefixes it per-table (UQE_project_board_*
// on the real table, UQE_tmp_recreate__project_board_* on RecreateTable's
// temp table); SQLite index names are database-scoped, so a verbatim UQE_*
// name would collide between the two.
func (*Column) TableIndices() []*schemas.Index {
	idx := schemas.NewIndex("project_sorting", schemas.UniqueType)
	idx.AddColumn("project_id", "sorting")
	return []*schemas.Index{idx}
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
			Sorting:     0,
		}
		if err := db.Insert(ctx, &column); err != nil {
			return err
		}

		if len(items) == 0 {
			return nil
		}

		columns := make([]Column, 0, len(items))
		for i, v := range items {
			columns = append(columns, Column{
				CreatedUnix: timeutil.TimeStampNow(),
				CreatorID:   project.CreatorID,
				Title:       v,
				ProjectID:   project.ID,
				Sorting:     int8(i + 1),
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

	// Wrap the read-then-insert in a transaction: the unique index on
	// (project_id, sorting) means two concurrent callers picking the same
	// max+1 would otherwise have one fail at insert time.
	return db.WithTx(ctx, func(ctx context.Context) error {
		res := struct {
			MaxSorting  int64
			ColumnCount int64
		}{}
		if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as column_count").Table("project_board").
			Where("project_id=?", column.ProjectID).Get(&res); err != nil {
			return err
		}
		if res.ColumnCount >= maxProjectColumns {
			return errors.New("NewBoard: maximum number of columns reached")
		}
		column.Sorting = int8(util.Iif(res.ColumnCount > 0, res.MaxSorting+1, 0))
		_, err := db.GetEngine(ctx).Insert(column)
		return err
	})
}

// DeleteColumnRow deletes the column row; refuses to delete a default column
func DeleteColumnRow(ctx context.Context, column *Column) error {
	if column.Default {
		return errors.New("DeleteColumnRow: cannot delete default column")
	}
	_, err := db.GetEngine(ctx).ID(column.ID).NoAutoCondition().Delete(column)
	return err
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

func GetColumnByIDAndProjectID(ctx context.Context, columnID, projectID int64) (*Column, error) {
	column := new(Column)
	has, err := db.GetEngine(ctx).ID(columnID).And("project_id=?", projectID).Get(column)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectColumnNotExist{ColumnID: columnID}
	}

	return column, nil
}

// UpdateColumn updates a project column
func UpdateColumn(ctx context.Context, column *Column) error {
	fieldToUpdate := []string{"sorting"}

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

// getDefaultColumnWithFallback return default column if one exists
// otherwise return the first column by sorting and set it as default column
func (p *Project) getDefaultColumnWithFallback(ctx context.Context) (*Column, error) {
	var column Column

	// try to find a column "default=true"
	has, err := db.GetEngine(ctx).
		Where("project_id=? AND `default` = ?", p.ID, true).
		Desc("id").Get(&column)
	if err != nil {
		return nil, err
	}

	if has {
		return &column, nil
	}

	// try to find the first column by sorting
	has, err = db.GetEngine(ctx).Where("project_id=?", p.ID).OrderBy("sorting, id").Get(&column)
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

	return nil, ErrProjectColumnNotExist{ColumnID: 0}
}

// MustDefaultColumn returns the default column for a project.
// If one exists, it is returned
// If none exists, the first column will be elevated to the default column of this project
// If there is no column, it creates a default column and returns it
func (p *Project) MustDefaultColumn(ctx context.Context) (*Column, error) {
	c, err := p.getDefaultColumnWithFallback(ctx)
	if err != nil && !IsErrProjectColumnNotExist(err) {
		return nil, err
	}
	if c != nil {
		return c, nil
	}

	// create a default column if none is found
	column := Column{
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

func GetColumnsByIDs(ctx context.Context, projectID int64, columnsIDs []int64) (ColumnList, error) {
	columns := make([]*Column, 0, 5)
	if len(columnsIDs) == 0 {
		return columns, nil
	}
	if err := db.GetEngine(ctx).
		Where("project_id =?", projectID).
		In("id", columnsIDs).
		OrderBy("sorting").Find(&columns); err != nil {
		return nil, err
	}
	return columns, nil
}
