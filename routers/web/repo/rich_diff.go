// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

	"gitea.dev/models/renderhelper"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/htmldiff"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const tplRichDiff templates.TplName = "repo/diff/rich_diff"

var errRichDiffTooLarge = errors.New("file too large for rich diff")

// RichDiff renders the changes between two versions of a markup file
func RichDiff(ctx *context.Context) {
	oldPath, newPath := ctx.FormString("old_path"), ctx.FormString("path")
	canRichDiff := func(treePath string) bool { return treePath == "" || markup.IsInlineRenderedFile(treePath) }
	if oldPath+newPath == "" || !canRichDiff(oldPath) || !canRichDiff(newPath) {
		ctx.NotFound()
		return
	}
	var err error
	ctx.Data["RichDiffHTML"], err = diffRichDiffFiles(ctx, oldPath, newPath)
	switch {
	case git.IsErrNotExist(err):
		ctx.NotFound()
		return
	case errors.Is(err, errRichDiffTooLarge):
		ctx.Data["RichDiffError"] = ctx.Locale.Tr("repo.diff.file_suppressed")
	case err != nil:
		log.Error("RichDiff: %v", err)
		ctx.Data["RichDiffError"] = ctx.Locale.Tr("repo.diff.rich_diff_failed")
	}
	ctx.HTML(http.StatusOK, tplRichDiff)
}

func diffRichDiffFiles(ctx *context.Context, oldPath, newPath string) (template.HTML, error) {
	oldCommitID, oldHTML, err := renderRichDiffFile(ctx, ctx.FormString("before"), oldPath)
	if err != nil {
		return "", err
	}
	newCommitID, newHTML, err := renderRichDiffFile(ctx, ctx.FormString("after"), newPath)
	if err != nil {
		return "", err
	}
	diffHTML, err := htmldiff.Diff(oldHTML, newHTML, htmldiff.Options{NormalizeLink: commitIDRemover(oldCommitID, newCommitID)})
	if err != nil {
		return "", err
	}
	_, escaped := charset.EscapeControlHTML(diffHTML, ctx.Locale, charset.EscapeOptionsForView())
	return escaped, nil
}

func renderRichDiffFile(ctx *context.Context, commitRef, treePath string) (commitID, rendered string, _ error) {
	if commitRef == "" || treePath == "" {
		return "", "", nil
	}
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, commitRef)
	if err != nil {
		return "", "", err
	}
	blob, err := commit.GetBlobByPath(ctx, ctx.Repo.GitRepo, treePath)
	if err != nil {
		return "", "", err
	}
	if blob.Size(ctx) > setting.UI.MaxDisplayFileSize {
		return "", "", errRichDiffTooLarge
	}
	reader, err := blob.DataAsync(ctx)
	if err != nil {
		return "", "", err
	}
	defer reader.Close()

	commitID = commit.ID.String()
	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefSubURL: git.RefNameFromCommit(commitID).RefWebLinkPath(),
		CurrentTreePath:  path.Dir(treePath),
	}).WithRelativePath(treePath)

	var sb strings.Builder
	if err := markup.RenderWithRenderer(rctx, markup.DetectRendererTypeByFilename(treePath), charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}), &sb); err != nil {
		return "", "", fmt.Errorf("renderRichDiffFile: %w", err)
	}
	return commitID, sb.String(), nil
}

// commitIDRemover makes links comparable, each version links to its own commit
func commitIDRemover(commitIDs ...string) func(string) string {
	var pairs []string
	for _, id := range commitIDs {
		if id != "" {
			pairs = append(pairs, id, "")
		}
	}
	return strings.NewReplacer(pairs...).Replace
}
