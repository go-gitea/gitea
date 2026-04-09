// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"
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

// RenderedDiffPreviewPost renders a side-by-side rendered markdown diff for the editor
func RenderedDiffPreviewPost(ctx *context.Context) {
	newContent := ctx.FormString("content")
	treePath := files_service.CleanGitTreePath(ctx.Repo.TreePath)
	if treePath == "" {
		ctx.HTTPError(http.StatusBadRequest, "file name to diff is invalid")
		return
	}

	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefPath:  ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath: path.Dir(treePath),
	}).WithRelativePath(treePath).
		WithMetas(ctx.Repo.Repository.ComposeRepoFileMetas(ctx))

	var baseHTML template.HTML
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err == nil && !entry.IsDir() {
		reader, err := entry.Blob().DataAsync()
		if err == nil {
			var buf strings.Builder
			err = markdown.Render(rctx, charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}), &buf)
			reader.Close()
			if err == nil {
				baseHTML = template.HTML(buf.String())
			}
		}
	}

	headHTML, err := markdown.RenderString(rctx, newContent)
	if err != nil {
		headHTML = ""
	}

	ctx.Data["MarkdownDiff"] = gitdiff.HTMLDiff(baseHTML, headHTML)

	ctx.HTML(http.StatusOK, tplRenderedDiffPreview)
}
