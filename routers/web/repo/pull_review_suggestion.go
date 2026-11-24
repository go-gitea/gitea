// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	files_service "gitea.dev/services/repository/files"
)

// ApplyCodeSuggestions commits the suggestions of code comments to the head branch of the pull request
func ApplyCodeSuggestions(ctx *context.Context) {
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	pr.Issue.Repo = ctx.Repo.Repository

	form := context.GetFetchActionForm[*forms.ApplyCodeSuggestionsForm](ctx)
	if form == nil {
		return
	}
	var commentIDs []int64
	contentVersions := map[int64]int{}
	for item := range strings.SplitSeq(form.CommentIDs, ",") {
		idStr, versionStr, _ := strings.Cut(item, ":")
		id, idErr := strconv.ParseInt(idStr, 10, 64)
		version, versionErr := strconv.Atoi(versionStr)
		if idErr != nil || versionErr != nil {
			ctx.HTTPError(http.StatusBadRequest)
			return
		}
		commentIDs = append(commentIDs, id)
		contentVersions[id] = version
	}
	_, err = files_service.ApplyCodeSuggestions(ctx, ctx.Doer, pr.ID, files_service.ApplyCodeSuggestionsOptions{
		CommentIDs:      commentIDs,
		ContentVersions: contentVersions,
		HeadCommitID:    form.HeadCommitID,
		Summary:         form.CommitSummary,
		Description:     form.CommitMessage,
	})
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	if form.Origin == "diff" {
		ctx.JSONRedirect(pr.Issue.Link() + "/files")
	} else {
		ctx.JSONRedirect(pr.Issue.Link() + "#" + issues_model.CommentHashTag(commentIDs[0]))
	}
}
