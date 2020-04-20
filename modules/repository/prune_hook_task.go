// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"

	"code.gitea.io/gitea/modules/log"
)

// PruneHookTaskTable deletes rows from hook_task as needed.
func PruneHookTaskTable(ctx context.Context) error {
	log.Trace("Doing: PruneHookTaskTable")

	//TODO implement me!!!!

	log.Trace("Finished: PruneHookTaskTable")
	return nil
}
