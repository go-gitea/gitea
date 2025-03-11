// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
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
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("no valid ref or sha: %s", sha))
		return
	}
	getNote(ctx, sha)
}

func getNote(ctx *context.APIContext, identifier string) {
	if ctx.Repo.GitRepo == nil {
		ctx.APIErrorInternal(fmt.Errorf("no open git repo"))
		return
	}

	commitID, err := ctx.Repo.GitRepo.ConvertToGitID(identifier)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	var note git.Note
	if err := git.GetNote(ctx, ctx.Repo.GitRepo, commitID.String(), &note); err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound(identifier)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	verification := ctx.FormString("verification") == "" || ctx.FormBool("verification")
	files := ctx.FormString("files") == "" || ctx.FormBool("files")

	cmt, err := convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, note.Commit, nil,
		convert.ToCommitOptions{
			Stat:         true,
			Verification: verification,
			Files:        files,
		})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiNote := api.Note{Message: string(note.Message), Commit: cmt}
	ctx.JSON(http.StatusOK, apiNote)
}
