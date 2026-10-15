// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"io/fs"
	"net/http"
	"slices"

	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/gitdiff"
)

// MarkPullReviewFileViewed marks a pull request file as viewed by the authenticated user.
func MarkPullReviewFileViewed(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/files/viewed repository repoMarkPullReviewFileViewed
	// ---
	// summary: Mark a pull request file as viewed
	// description: |
	//   Updates the authenticated user's state for a repository-relative path in the current pull request diff, including deleted files and rename destinations.
	//   commit_id is an optional optimistic guard and defaults to the current pull request head. If provided, it must equal the full current head commit SHA; otherwise the response is HTTP 409.
	//   An empty or invalid path, or a path outside the diff, is rejected with HTTP 422.
	//   Archived repositories return HTTP 404.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/MarkPullReviewFileOptions"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "500":
	//     "$ref": "#/responses/error"
	updatePullReviewFileState(ctx, pull_model.Viewed)
}

// MarkPullReviewFileUnviewed marks a pull request file as unviewed by the authenticated user.
func MarkPullReviewFileUnviewed(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/files/unviewed repository repoMarkPullReviewFileUnviewed
	// ---
	// summary: Mark a pull request file as unviewed
	// description: |
	//   Updates the authenticated user's state for a repository-relative path in the current pull request diff, including deleted files and rename destinations.
	//   commit_id is an optional optimistic guard and defaults to the current pull request head. If provided, it must equal the full current head commit SHA; otherwise the response is HTTP 409.
	//   An empty or invalid path, or a path outside the diff, is rejected with HTTP 422.
	//   Archived repositories return HTTP 404.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/MarkPullReviewFileOptions"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "500":
	//     "$ref": "#/responses/error"
	updatePullReviewFileState(ctx, pull_model.Unviewed)
}

func updatePullReviewFileState(ctx *context.APIContext, state pull_model.ViewedState) {
	opts := web.GetForm[*api.MarkPullReviewFileOptions](ctx)
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if !fs.ValidPath(opts.Path) {
		ctx.APIError(http.StatusUnprocessableEntity, "path must be a repository-relative file path")
		return
	}

	gitRepo := ctx.Repo.GitRepo
	headCommitID, err := gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if opts.CommitID != "" && opts.CommitID != headCommitID {
		ctx.APIError(http.StatusConflict, "commit_id must equal the current pull request head commit SHA")
		return
	}

	mergeBase := pr.MergeBase
	if !pr.HasMerged {
		mergeBase, err = git.MergeBase(ctx, ctx.Repo.Repository, git.BranchPrefix+pr.BaseBranch, headCommitID)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorInternal(err)
			return
		}
	}
	if mergeBase == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "pull request has no common ancestor")
		return
	}
	diffTree, err := gitdiff.GetDiffTree(ctx, gitRepo, false, mergeBase, headCommitID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	changedFiles := make([]string, 0, len(diffTree.Files))
	for _, file := range diffTree.Files {
		changedFiles = append(changedFiles, file.HeadPath)
	}
	if !slices.Contains(changedFiles, opts.Path) {
		ctx.APIError(http.StatusUnprocessableEntity, "path is not a changed file in the pull request")
		return
	}

	// Reconcile prior state before it is inherited by a new head commit.
	diff := &gitdiff.Diff{Files: make([]*gitdiff.DiffFile, len(changedFiles))}
	for i, path := range changedFiles {
		diff.Files[i] = &gitdiff.DiffFile{Name: path}
	}
	if _, err := gitdiff.SyncUserSpecificDiff(ctx, ctx.Doer.ID, pr, gitRepo, diff, &gitdiff.DiffOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{AfterCommitID: headCommitID},
	}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if _, err := pull_model.UpdateReviewState(ctx, ctx.Doer.ID, pr.ID, headCommitID, map[string]pull_model.ViewedState{opts.Path: state}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
