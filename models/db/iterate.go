// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"reflect"

	"gitea.dev/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func int64PrimaryKey(table *schemas.Table) *schemas.Column {
	if len(table.PrimaryKeys) != 1 {
		return nil
	}
	primaryKey := table.GetColumn(table.PrimaryKeys[0])
	if primaryKey != nil && table.Type.FieldByIndex(primaryKey.FieldIndex).Type.Kind() == reflect.Int64 {
		return primaryKey
	}
	return nil
}

// Iterate iterates all the Bean object
func Iterate[Bean any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	batchSize := setting.Database.IterateBufferSize
	table, err := xormEngine.TableInfo(new(Bean))
	if err != nil {
		return err
	}
	primaryKey := int64PrimaryKey(table)
	var start int
	var lastPrimaryKey *int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		beans := make([]*Bean, 0, batchSize)
		query := GetEngine(ctx).Table(table.Name).Asc(table.PrimaryKeys...)
		batchCond := cond
		if lastPrimaryKey != nil {
			batchCond = builder.And(cond, builder.Gt{primaryKey.Name: *lastPrimaryKey})
		}
		if batchCond != nil {
			query = query.Where(batchCond)
		}
		if err := query.Limit(batchSize, start).Find(&beans); err != nil { // start stays 0 while keyset paging
			return err
		}
		if len(beans) == 0 {
			return nil
		}
		if primaryKey != nil {
			primaryKeyValue, err := primaryKey.ValueOf(beans[len(beans)-1])
			if err != nil {
				return err
			}
			lastPrimaryKey = new(primaryKeyValue.Int())
		} else {
			start += len(beans)
		}

		for _, bean := range beans {
			if err := f(ctx, bean); err != nil {
				return err
			}
		}
	}
}
