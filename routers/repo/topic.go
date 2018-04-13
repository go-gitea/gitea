// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// TopicPost response for creating repository
func TopicPost(ctx *context.Context) {
	if ctx.User == nil {
		ctx.JSON(403, map[string]interface{}{
			"message": "Only owners could change the topics.",
		})
		return
	}

	topics := strings.Split(ctx.Query("topics"), ",")

	err := models.SaveTopics(ctx.Repo.Repository.ID, topics...)
	if err != nil {
		log.Error(2, "SaveTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "Save topics failed.",
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"status": "ok",
	})
}
