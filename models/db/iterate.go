// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"

	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

// Iterate iterates all the Bean object
func Iterate[Bean any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	batchSize := setting.Database.IterateBufferSize
	table, err := xormEngine.TableInfo(new(Bean))
	if err != nil {
		return err
	}
	if len(table.PrimaryKeys) != 1 {
		return fmt.Errorf("table %s has %d primary keys, only the table with exactly one primary key can be iterated", table.Name, len(table.PrimaryKeys))
	}

	primaryKeyColumn := table.GetColumn(table.PrimaryKeys[0])
	var lastPrimaryValue any
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		beans := make([]*Bean, 0, batchSize)
		query := GetEngine(ctx).Table(table.Name).OrderBy(table.PrimaryKeys[0])
		batchCond := cond
		if lastPrimaryValue != nil {
			batchCond = builder.And(cond, builder.Gt{primaryKeyColumn.Name: lastPrimaryValue})
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

		reflectVal, err := primaryKeyColumn.ValueOf(beans[len(beans)-1])
		if err != nil {
			return err
		}
		lastPrimaryValue = reflectVal.Interface()
		for _, bean := range beans {
			if err := f(ctx, bean); err != nil {
				return err
			}
		}
	}
}
