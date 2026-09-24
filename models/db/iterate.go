// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"

	"gitea.dev/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func iterateTableByColumn[Bean any](ctx context.Context, colName string, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	table, err := xormEngine.TableInfo(new(Bean))
	if err != nil {
		return err
	}

	var col *schemas.Column
	if colName == "" {
		if len(table.PrimaryKeys) != 1 {
			return fmt.Errorf("table %s has %d primary keys, only the table with exactly one primary key can be iterated", table.Name, len(table.PrimaryKeys))
		}
		colName = table.PrimaryKeys[0]
	}

	col = table.GetColumn(colName)
	batchSize := setting.Database.IterateBufferSize
	var lastColValue any
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		beans := make([]*Bean, 0, batchSize)
		query := GetEngine(ctx).Table(table.Name).Asc(colName)

		batchCond := cond
		if lastColValue != nil {
			batchCond = builder.And(cond, builder.Gt{col.Name: lastColValue})
		}
		if batchCond != nil {
			query = query.Where(batchCond)
		}

		if err := query.Limit(batchSize).Find(&beans); err != nil {
			return err
		}
		if len(beans) == 0 {
			return nil
		}

		reflectVal, err := col.ValueOf(beans[len(beans)-1])
		if err != nil {
			return err
		}
		lastColValue = reflectVal.Interface()

		for _, bean := range beans {
			if err := f(ctx, bean); err != nil {
				return err
			}
		}
	}
}

func IterateByColumn[Bean any](ctx context.Context, colName string, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	return iterateTableByColumn(ctx, colName, cond, f)
}

func Iterate[Bean any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	return iterateTableByColumn(ctx, "", cond, f)
}
