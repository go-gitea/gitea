// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"
	files_service "code.gitea.io/gitea/services/repository/files"
)

const tplRichDiffFragment templates.TplName = "repo/diff/rich_diff_fragment"

// RichDiffComparePost renders the rich diff for a single file in a compare
// view on demand. The compare template emits an HTMX placeholder per eligible
// file so this endpoint only runs for files the viewer actually reveals,
// avoiding the N markdown renders per page-load that the previous in-template
// implementation did.
//
// Expected form values:
//   - base: base commit SHA (empty means added file)
//   - head: head commit SHA (empty means deleted file)
//   - old_name: file path in the base commit
//   - new_name: file path in the head commit
func RichDiffComparePost(ctx *context.Context) {
	baseSHA := ctx.FormString("base")
	headSHA := ctx.FormString("head")
	oldName := files_service.CleanGitTreePath(ctx.FormString("old_name"))
	newName := files_service.CleanGitTreePath(ctx.FormString("new_name"))
	if oldName == "" && newName == "" {
		ctx.HTTPError(http.StatusBadRequest, "missing file path")
		return
	}

	// An empty SHA means the file does not exist on that side (added or
	// deleted file), so we skip the commit lookup and leave the commit nil.
	gitRepo := ctx.Repo.GitRepo
	var baseCommit, headCommit *git.Commit
	if baseSHA != "" {
		c, err := gitRepo.GetCommit(baseSHA)
		if err != nil {
			ctx.HTTPError(http.StatusBadRequest, "invalid base commit")
			return
		}
		baseCommit = c
	}
	if headSHA != "" {
		c, err := gitRepo.GetCommit(headSHA)
		if err != nil {
			ctx.HTTPError(http.StatusBadRequest, "invalid head commit")
			return
		}
		headCommit = c
	}

	getBlob := func(commit *git.Commit, path string) *git.Blob {
		if commit == nil || path == "" {
			return nil
		}
		blob, err := commit.GetBlobByPath(path)
		if err != nil {
			return nil
		}
		return blob
	}

	baseBlob := getBlob(baseCommit, oldName)
	headBlob := getBlob(headCommit, newName)

	baseHTML, err := renderRichDiffBlob(ctx, baseBlob, oldName, baseCommit)
	if err != nil {
		if errors.Is(err, errRichDiffTooLarge) {
			ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.file_suppressed")
			ctx.HTML(http.StatusOK, tplRichDiffFragment)
			return
		}
		log.Error("error rendering base rich diff %s: %v", oldName, err)
		ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.rich_diff_unable_to_render")
		ctx.HTML(http.StatusOK, tplRichDiffFragment)
		return
	}

	headHTML, err := renderRichDiffBlob(ctx, headBlob, newName, headCommit)
	if err != nil {
		if errors.Is(err, errRichDiffTooLarge) {
			ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.file_suppressed")
			ctx.HTML(http.StatusOK, tplRichDiffFragment)
			return
		}
		log.Error("error rendering head rich diff %s: %v", newName, err)
		ctx.Data["RichDiffError"] = ctx.Locale.TrString("repo.diff.rich_diff_unable_to_render")
		ctx.HTML(http.StatusOK, tplRichDiffFragment)
		return
	}

	ctx.Data["RichDiff"] = gitdiff.HTMLDiff(baseHTML, headHTML)
	ctx.HTML(http.StatusOK, tplRichDiffFragment)
}
