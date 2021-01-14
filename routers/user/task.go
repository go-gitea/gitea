// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// TaskStatus returns task's status
func TaskStatus(ctx *context.Context) {
	task, opts, err := models.GetMigratingTaskByID(ctx.ParamsInt64("task"), ctx.User.ID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err,
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"status":    task.Status,
		"err":       task.Errors,
		"repo-id":   task.RepoID,
		"repo-name": opts.RepoName,
		"start":     task.StartTime,
		"end":       task.EndTime,
	})
}
