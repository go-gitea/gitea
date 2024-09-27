// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"strconv"

	admin_model "code.gitea.io/gitea/models/admin"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/services/context"
)

// TaskStatus returns task's status
func TaskStatus(ctx *context.Context) {
	task, _, err := admin_model.GetMigratingTaskByID(ctx, ctx.PathParamInt64("task"), 0)
	if err != nil {
		if admin_model.IsErrTaskDoesNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]any{
				"error": "task `" + strconv.FormatInt(ctx.PathParamInt64("task"), 10) + "` does not exist",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"err": err,
		})
		return
	}

	if err := task.LoadRepo(ctx); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"err": err,
		})
		return
	}

	perm, err := access_model.GetUserRepoPermission(ctx, task.Repo, ctx.Doer)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"err": err,
		})
		return
	}
	if !perm.CanRead(unit.TypeCode) {
		ctx.JSON(http.StatusForbidden, map[string]any{
			"error": "you do not have access to this task",
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
				Args:   []any{task.Message},
			}
		}
		message = ctx.Locale.TrString(translatableMessage.Format, translatableMessage.Args...)
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"status":  task.Status,
		"message": message,
	})
}
