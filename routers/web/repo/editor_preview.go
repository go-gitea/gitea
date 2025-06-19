// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// DiffPreviewPost render preview diff page
func DiffPreviewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditPreviewDiffForm)
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

	diff, err := files_service.GetDiffPreview(ctx, ctx.Repo.Repository, ctx.Repo.BranchName, treePath, form.Content)
	if err != nil {
		ctx.ServerError("GetDiffPreview", err)
		return
	}

	if len(diff.Files) != 0 {
		ctx.Data["File"] = diff.Files[0]
	}

	ctx.HTML(http.StatusOK, tplEditDiffPreview)
}
