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
	"gitea.dev/services/gitdiff"
)

const tplRichDiff templates.TplName = "repo/diff/rich_diff"

var errRichDiffTooLarge = errors.New("file too large for rich diff")

// RichDiff renders the changes between two versions of a markup file
func RichDiff(ctx *context.Context) {
	oldPath, newPath := ctx.FormString("old_path"), ctx.FormString("path")
	canRichDiff := func(treePath string) bool { return treePath == "" || gitdiff.CanRichDiff(treePath) }
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
	return htmldiff.Diff(oldHTML, newHTML, htmldiff.Options{NormalizeLink: commitIDRemover(oldCommitID, newCommitID)})
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
	refSubURL := git.RefNameFromCommit(commitID).RefWebLinkPath()
	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefSubURL: refSubURL,
		CurrentTreePath:  path.Dir(treePath),
	}).WithRelativePath(treePath)
	metas := ctx.Repo.Repository.ComposeRepoFileMetas(ctx)
	metas["RefTypeNameSubURL"] = refSubURL
	rctx.WithMetas(metas)

	renderer := markup.DetectRendererTypeByFilename(treePath)
	_, html, err := markupRenderToHTML(ctx, rctx, renderer, charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}))
	if err != nil {
		return "", "", fmt.Errorf("renderRichDiffFile: %w", err)
	}
	return commitID, string(html), nil
}

// commitIDRemover makes the links of both versions comparable, they point to their own commit
func commitIDRemover(commitIDs ...string) func(string) string {
	var pairs []string
	for _, id := range commitIDs {
		if id != "" {
			pairs = append(pairs, id, "")
		}
	}
	return strings.NewReplacer(pairs...).Replace
}
