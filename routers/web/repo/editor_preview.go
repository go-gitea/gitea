// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"
)

func DiffPreviewPost(ctx *context.Context) {
	newContent := ctx.FormString("content")
	treePath := files_service.CleanGitTreePath(ctx.Repo.TreePath)
	if treePath == "" {
		ctx.HTTPError(http.StatusBadRequest, "file name to diff is invalid")
		return
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err != nil {
		ctx.ServerError("GetTreeEntryByPath", err)
		return
	} else if entry.IsDir() {
		ctx.HTTPError(http.StatusUnprocessableEntity)
		return
	}

	oldContent, err := entry.Blob().GetBlobContent(setting.UI.MaxDisplayFileSize)
	if err != nil {
		ctx.ServerError("GetBlobContent", err)
		return
	}
	diff, err := files_service.GetDiffPreview(ctx, ctx.Repo.Repository, ctx.Repo.BranchName, treePath, oldContent, newContent)
	if err != nil {
		ctx.ServerError("GetDiffPreview", err)
		return
	}

	if len(diff.Files) != 0 {
		ctx.Data["File"] = diff.Files[0]
	}

	ctx.HTML(http.StatusOK, tplEditDiffPreview)
}
