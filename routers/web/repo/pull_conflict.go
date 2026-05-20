// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	pull_service "code.gitea.io/gitea/services/pull"
)

const tplConflictResolution templates.TplName = "repo/pulls/conflict_resolution"

// conflictEditorFilePath extracts and validates the wildcard file path from the
// URL, returning an empty string when the path looks invalid.
func conflictEditorFilePath(ctx *context.Context) string {
	p := ctx.PathParam("*")
	if p == "" || strings.Contains(p, "..") {
		return ""
	}
	return p
}

// checkConflictWriteAccess returns true when the doer may push to pr.HeadBranch.
func checkConflictWriteAccess(ctx *context.Context, pull *issues_model.PullRequest) bool {
	if err := pull.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return false
	}
	if pull.HeadRepo == nil {
		ctx.NotFound(nil)
		return false
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetDoerRepoPermission", err)
		return false
	}
	if perm.CanWrite(unit.TypeCode) {
		return true
	}
	if err := pull.LoadBaseRepo(ctx); err != nil {
		ctx.ServerError("LoadBaseRepo", err)
		return false
	}
	basePerm, err := access_model.GetDoerRepoPermission(ctx, pull.BaseRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetDoerRepoPermission", err)
		return false
	}
	if pull.AllowMaintainerEdit && basePerm.CanWrite(unit.TypeCode) {
		return true
	}
	ctx.NotFound(nil)
	return false
}

// ResolveConflictsEditorRedirect redirects to the editor for the first
// conflicting file when no specific file is given in the URL.
func ResolveConflictsEditorRedirect(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest
	if !pull.IsFilesConflicted() || len(pull.ConflictedFiles) == 0 {
		ctx.Redirect(issue.Link())
		return
	}
	ctx.Redirect(issue.Link() + "/conflicts/editor/" + util.PathEscapeSegments(pull.ConflictedFiles[0]))
}

// ResolveConflictsEditor renders the multi-file conflict resolution page.
func ResolveConflictsEditor(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	if pull.HasMerged || issue.IsClosed {
		ctx.NotFound(nil)
		return
	}
	if !pull.IsFilesConflicted() {
		ctx.Redirect(issue.Link())
		return
	}

	filePath := conflictEditorFilePath(ctx)
	if filePath == "" || !slices.Contains(pull.ConflictedFiles, filePath) {
		ctx.NotFound(nil)
		return
	}

	if !checkConflictWriteAccess(ctx, pull) {
		return
	}

	WebGitOperationCommonData(ctx)

	editorCfg := CodeEditorConfig{
		Filename:              path.Base(filePath),
		Autofocus:             true,
		PreviewableExtensions: markup.PreviewableExtensions(),
		LineWrapExtensions:    setting.Repository.Editor.LineWrapExtensions,
		LineWrap:              util.SliceContainsString(setting.Repository.Editor.LineWrapExtensions, path.Ext(filePath), true),
		Previewable:           util.SliceContainsString(markup.PreviewableExtensions(), path.Ext(filePath), true),
	}

	ctx.Data["PageIsPullConflictResolution"] = true
	ctx.Data["Issue"] = issue
	ctx.Data["PullRequest"] = pull
	ctx.Data["InitialFilePath"] = filePath
	ctx.Data["CodeEditorConfig"] = editorCfg
	ctx.Data["ResolveConflictsURL"] = issue.Link() + "/conflicts/resolve"
	ctx.Data["FileContentURL"] = issue.Link() + "/conflicts/file-content"
	ctx.HTML(http.StatusOK, tplConflictResolution)
}

// GetConflictedFileContentJSON returns the conflict-marker content for a file
// as JSON {"content":"..."}.  The frontend loads files lazily.
func GetConflictedFileContentJSON(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	if !pull.IsFilesConflicted() {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	filePath := ctx.FormString("path")
	if filePath == "" || strings.Contains(filePath, "..") || !slices.Contains(pull.ConflictedFiles, filePath) {
		ctx.NotFound(nil)
		return
	}

	if !checkConflictWriteAccess(ctx, pull) {
		return
	}

	content, err := pull_service.GetConflictedFileContent(ctx, pull, filePath)
	if err != nil {
		log.Error("GetConflictedFileContent pr #%d file %q: %v", pull.Index, filePath, err)
		ctx.ServerError("GetConflictedFileContent", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{"content": content})
}

// resolveConflictsBatchRequest is the JSON body accepted by ResolveConflictsBatchPost.
type resolveConflictsBatchRequest struct {
	Message string                      `json:"message"`
	Files   []resolveConflictsBatchFile `json:"files"`
}

type resolveConflictsBatchFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ResolveConflictsBatchPost receives all resolved files and creates a merge
// commit on the head branch that sets the base branch tip as a second parent.
// This is the correct git-level resolution: it makes the PR conflict check
// find no conflicts because the base tip is already reachable from head.
func ResolveConflictsBatchPost(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	if pull.HasMerged || issue.IsClosed {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	var req resolveConflictsBatchRequest
	if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err != nil {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	if len(req.Files) == 0 {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}

	if !checkConflictWriteAccess(ctx, pull) {
		return
	}

	for _, f := range req.Files {
		if f.Path == "" || strings.Contains(f.Path, "..") {
			ctx.HTTPError(http.StatusBadRequest)
			return
		}
		if strings.Contains(f.Content, "<<<<<<<") || strings.Contains(f.Content, ">>>>>>>") {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": ctx.Locale.TrString("repo.pulls.conflict_resolution_has_markers"),
				"path":  f.Path,
			})
			return
		}
	}

	commitMsg := strings.TrimSpace(req.Message)
	if commitMsg == "" {
		commitMsg = ctx.Locale.TrString("repo.pulls.conflict_resolution_batch_commit_message")
	}

	// Build the commit author / committer environment.
	sig := ctx.Doer.NewGitSig()
	now := time.Now().Format(time.RFC3339)
	commitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+now,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+now,
	)

	resolved := make([]pull_service.ResolvedFile, 0, len(req.Files))
	for _, f := range req.Files {
		resolved = append(resolved, pull_service.ResolvedFile{Path: f.Path, Content: f.Content})
	}

	if err := pull_service.CommitConflictResolution(ctx, pull, ctx.Doer, resolved, commitMsg, commitEnv); err != nil {
		log.Error("CommitConflictResolution pr #%d: %v", pull.Index, err)
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": ctx.Locale.TrString("repo.pulls.conflict_resolution_commit_failed"),
		})
		return
	}

	// Mark PR as "checking" so the PR page shows an in-progress state and
	// auto-refreshes instead of still showing the stale conflict status.
	pull_service.StartPullRequestCheckImmediately(ctx, pull)

	ctx.JSON(http.StatusOK, map[string]string{"redirect": issue.Link()})
}
