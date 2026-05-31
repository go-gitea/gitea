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
)

// CompareDiff compare two branches or commits
func CompareDiff(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{basehead} repository repoCompareDiff
	// ---
	// summary: Get commit comparison information
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
	// - name: basehead
	//   in: path
	//   description: compare two branches or commits
	//   type: string
	//   required: true
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

// DownloadCompareDiffOrPatch render a comparison's raw diff or patch
func DownloadCompareDiffOrPatch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{basehead}.{diffType} repository repoDownloadCompareDiffOrPatch
	// ---
	// summary: Get a comparison's diff or patch between two refs
	// produces:
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
	// - name: diffType
	//   in: path
	//   description: whether the output is diff or patch
	//   type: string
	//   enum: [diff, patch]
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/string"
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

	compareInfo, closer := parseCompareInfo(ctx, ctx.PathParam("basehead"))
	if ctx.Written() {
		return
	}
	defer closer()

	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	compareArg := compareInfo.BaseCommitID + compareInfo.CompareSeparator + compareInfo.HeadCommitID

	var err error
	if ctx.PathParam("diffType") == "patch" {
		err = compareInfo.HeadGitRepo.GetPatch(compareArg, ctx.Resp)
	} else {
		err = compareInfo.HeadGitRepo.GetDiff(compareArg, ctx.Resp)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
}
