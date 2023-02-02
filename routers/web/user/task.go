// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"strconv"

	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
)

// TaskStatus returns task's status
func TaskStatus(ctx *context.Context) {
	task, opts, err := admin_model.GetMigratingTaskByID(ctx.ParamsInt64("task"), ctx.Doer.ID)
	if err != nil {
		if admin_model.IsErrTaskDoesNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "task `" + strconv.FormatInt(ctx.ParamsInt64("task"), 10) + "` does not exist",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err,
		})
		return
	}

	message := task.Message

	if task.Message != "" && task.Message[0] == '{' {
		// assume message is actually a translatable string
		var translatableMessage admin_model.TranslatableMessage
		if err := json.Unmarshal([]byte(message), &translatableMessage); err != nil {
			translatableMessage = admin_model.TranslatableMessage{
				Format: "migrate.migrating_failed.error",
				Args:   []interface{}{task.Message},
			}
		}
		message = ctx.Tr(translatableMessage.Format, translatableMessage.Args...)
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"status":    task.Status,
		"message":   message,
		"repo-id":   task.RepoID,
		"repo-name": opts.RepoName,
		"start":     task.StartTime,
		"end":       task.EndTime,
	})
}
