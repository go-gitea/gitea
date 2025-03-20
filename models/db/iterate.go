// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// Iterate iterates all the Bean object
func Iterate[Bean any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	sess := GetEngine(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			beans := make([]*Bean, 0, batchSize)
			if cond != nil {
				sess = sess.Where(cond)
			}
			if err := sess.Limit(batchSize, start).Find(&beans); err != nil {
				return err
			}
			if len(beans) == 0 {
				return nil
			}
			start += len(beans)

			for _, bean := range beans {
				if err := f(ctx, bean); err != nil {
					return err
				}
			}
		}
	}
}
