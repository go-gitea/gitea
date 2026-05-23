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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
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

// RenderedDiffPreviewPost renders a side-by-side rendered rich diff for the editor,
// dispatching to whichever inline-HTML renderer matches the current file (markdown, orgmode, ...).
func RenderedDiffPreviewPost(ctx *context.Context) {
	newContent := ctx.FormString("content")
	treePath := files_service.CleanGitTreePath(ctx.Repo.TreePath)
	if treePath == "" {
		ctx.HTTPError(http.StatusBadRequest, "file name to diff is invalid")
		return
	}

	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefSubURL: ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath:  path.Dir(treePath),
	}).WithRelativePath(treePath).
		WithMetas(ctx.Repo.Repository.ComposeRepoFileMetas(ctx))

	var baseHTML template.HTML
	tooLarge := false
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err != nil {
		log.Error("RenderedDiffPreviewPost: GetTreeEntryByPath %s: %v", treePath, err)
		ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.rich_diff_unable_to_render")
		ctx.HTML(http.StatusOK, tplRenderedDiffPreview)
		return
	}
	if !entry.IsDir() {
		blob := entry.Blob()
		// Match the compare view: skip the rich diff for oversized blobs so we
		// do not hand a multi-megabyte document to diffmatchpatch.
		if setting.UI.MaxDisplayFileSize > 0 && blob.Size() > setting.UI.MaxDisplayFileSize {
			tooLarge = true
		} else {
			reader, err := blob.DataAsync()
			if err != nil {
				log.Error("RenderedDiffPreviewPost: DataAsync %s: %v", treePath, err)
				ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.rich_diff_unable_to_render")
				ctx.HTML(http.StatusOK, tplRenderedDiffPreview)
				return
			}
			defer reader.Close()
			var buf strings.Builder
			if err := markup.Render(rctx, charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}), &buf); err != nil {
				log.Error("RenderedDiffPreviewPost: markup.Render %s: %v", treePath, err)
				ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.rich_diff_unable_to_render")
				ctx.HTML(http.StatusOK, tplRenderedDiffPreview)
				return
			}
			baseHTML = template.HTML(buf.String())
		}
	}

	if !tooLarge && setting.UI.MaxDisplayFileSize > 0 && int64(len(newContent)) > setting.UI.MaxDisplayFileSize {
		tooLarge = true
	}

	if tooLarge {
		ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.file_suppressed")
	} else {
		headStr, err := markup.RenderString(rctx, newContent)
		if err != nil {
			headStr = ""
		}
		ctx.Data["RichDiff"] = gitdiff.HTMLDiff(baseHTML, template.HTML(headStr))
	}

	ctx.HTML(http.StatusOK, tplRenderedDiffPreview)
}
