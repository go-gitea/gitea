// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// TopicsPost response for creating repository
func TopicsPost(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]any{
			"message": "Only owners could change the topics.",
		})
		return
	}

	topics := make([]string, 0)
	topicsStr := ctx.FormTrim("topics")
	if len(topicsStr) > 0 {
		topics = strings.Split(topicsStr, ",")
	}

	validTopics, invalidTopics := repo_model.SanitizeAndValidateTopics(topics)

	if len(validTopics) > 25 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": nil,
			"message":       ctx.Tr("repo.topic.count_prompt"),
		})
		return
	}

	if len(invalidTopics) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": invalidTopics,
			"message":       ctx.Tr("repo.topic.format_prompt"),
		})
		return
	}

	err := repo_model.SaveTopics(ctx, ctx.Repo.Repository.ID, validTopics...)
	if err != nil {
		log.Error("SaveTopics failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"message": "Save topics failed.",
		})
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"status": "ok",
	})
}
