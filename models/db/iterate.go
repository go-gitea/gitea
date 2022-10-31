// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// Iterate iterate all the Bean object
func Iterate[Object any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, repo *Object) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	sess := GetEngine(ctx)
	for {
		repos := make([]*Object, 0, batchSize)
		if cond != nil {
			sess = sess.Where(cond)
		}
		if err := sess.Limit(batchSize, start).Find(&repos); err != nil {
			return err
		}
		if len(repos) == 0 {
			return nil
		}
		start += len(repos)

		for _, repo := range repos {
			if err := f(ctx, repo); err != nil {
				return err
			}
		}
	}
}
