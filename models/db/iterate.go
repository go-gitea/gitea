// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"

	"code.gitea.io/gitea/modules/setting"
)

// IterateObjects iterate all the Bean object
func IterateObjects[Object any](ctx context.Context, f func(repo *Object) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	sess := GetEngine(ctx)
	for {
		repos := make([]*Object, 0, batchSize)
		if err := sess.Limit(batchSize, start).Find(&repos); err != nil {
			return err
		}
		if len(repos) == 0 {
			return nil
		}
		start += len(repos)

		for _, repo := range repos {
			if err := f(repo); err != nil {
				return err
			}
		}
	}
}
