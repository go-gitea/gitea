// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	git_service "gitea.dev/services/git"
)

// CompareDiff compare two branches or commits
func CompareDiff(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{basehead} repository repoCompareDiff
	// ---
	// summary: Get commit comparison information
	// description: |
	//   By default returns JSON commit comparison information. The raw diff or patch can be
	//   requested with the `output` query parameter set to `diff` or `patch` respectively.
	// produces:
	// - application/json
	// - text/plain
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
	// - name: basehead
	//   in: path
	//   description: compare two refs as `base...head` (or `base..head`); refs may be branches, tags, full or short SHAs, including branch names that contain slashes.
	//   type: string
	//   required: true
	// - name: output
	//   in: query
	//   description: return the raw comparison as `diff` or `patch` instead of JSON
	//   type: string
	//   enum:
	//   - diff
	//   - patch
	// responses:
	//   "200":
	//     "$ref": "#/responses/Compare"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.GitRepo == nil {
		var err error
		ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	compareInfo, closer := parseCompareInfo(ctx, ctx.PathParam("*"))
	if ctx.Written() {
		return
	}
	defer closer()

	// ?output=diff|patch returns the raw output, otherwise the JSON comparison is returned.
	switch ctx.FormString("output") {
	case "diff":
		downloadCompareDiffOrPatch(ctx, compareInfo, false)
		return
	case "patch":
		downloadCompareDiffOrPatch(ctx, compareInfo, true)
		return
	}

	verification := ctx.FormString("verification") == "" || ctx.FormBool("verification")
	files := ctx.FormString("files") == "" || ctx.FormBool("files")

	apiCommits := make([]*api.Commit, 0, len(compareInfo.Commits))
	userCache := make(map[string]*user_model.User)

	for i := 0; i < len(compareInfo.Commits); i++ {
		apiCommit, err := convert.ToCommit(
			ctx,
			compareInfo.HeadRepo,
			compareInfo.HeadGitRepo,
			compareInfo.Commits[i],
			userCache,
			convert.ToCommitOptions{
				Stat:         true,
				Verification: verification,
				Files:        files,
			},
		)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiCommits = append(apiCommits, apiCommit)
	}

	ctx.JSON(http.StatusOK, &api.Compare{
		TotalCommits: len(compareInfo.Commits),
		Commits:      apiCommits,
	})
}

// downloadCompareDiffOrPatch writes a comparison's raw diff or patch to the response.
func downloadCompareDiffOrPatch(ctx *context.APIContext, compareInfo *git_service.CompareInfo, patch bool) {
	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	compareArg := compareInfo.BaseCommitID + compareInfo.CompareSeparator + compareInfo.HeadCommitID

	var err error
	if patch {
		err = compareInfo.HeadGitRepo.GetPatch(compareArg, ctx.Resp)
	} else {
		err = compareInfo.HeadGitRepo.GetDiff(compareArg, ctx.Resp)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
}
