// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// GetNote Get a note corresponding to a single commit from a repository
func GetNote(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/notes/{sha} repository repoGetNote
	// ---
	// summary: Get a note corresponding to a single commit from a repository
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
	// - name: sha
	//   in: path
	//   description: a git ref or commit sha
	//   type: string
	//   required: true
	// - name: verification
	//   in: query
	//   description: include verification for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: files
	//   in: query
	//   description: include a list of affected files for every commit (disable for speedup, default 'true')
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/Note"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.PathParam("sha")
	if !git.IsValidRefPattern(sha) {
		ctx.APIError(http.StatusUnprocessableEntity, "no valid ref or sha: "+sha)
		return
	}
	getNote(ctx, sha)
}

func getNote(ctx *context.APIContext, ref string) {
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, ref)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	note, lastCommit, err := git.GetNoteWithLastCommit(ctx, ctx.Repo.GitRepo, commit.ID.String())
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	verification := ctx.FormString("verification") == "" || ctx.FormBool("verification")
	files := ctx.FormString("files") == "" || ctx.FormBool("files")

	cmt, err := convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, lastCommit, nil,
		convert.ToCommitOptions{
			Stat:         true,
			Verification: verification,
			Files:        files,
		})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiNote := api.Note{Message: note.BlobMessage.MessageUTF8(), Commit: cmt}
	ctx.JSON(http.StatusOK, apiNote)
}
