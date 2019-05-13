// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

// ListGitHooks list all Git hooks of a repository
func ListGitHooks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks/git repository repoListGitHooks
	// ---
	// summary: List the Git hooks in a repository
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitHookList"
	hooks, err := ctx.Repo.GitRepo.Hooks()
	if err != nil {
		ctx.Error(500, "Hooks", err)
		return
	}

	apiHooks := make([]*api.GitHook, len(hooks))
	for i := range hooks {
		apiHooks[i] = convert.ToGitHook(hooks[i])
	}
	ctx.JSON(200, &apiHooks)
}

// GetGitHook get a repo's Git hook by id
func GetGitHook(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks/git/{id} repository repoGetGitHook
	// ---
	// summary: Get a Git hook
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
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitHook"
	//   "404":
	//     "$ref": "#/responses/notFound"
	hookID := ctx.Params(":id")
	hook, err := ctx.Repo.GitRepo.GetHook(hookID)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetHook", err)
		}
		return
	}
	ctx.JSON(200, convert.ToGitHook(hook))
}

// EditGitHook modify a Git hook of a repository
func EditGitHook(ctx *context.APIContext, form api.EditGitHookOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/hooks/git/{id} repository repoEditGitHook
	// ---
	// summary: Edit a Git hook in a repository
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
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditGitHookOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitHook"
	//   "404":
	//     "$ref": "#/responses/notFound"
	hookID := ctx.Params(":id")
	hook, err := ctx.Repo.GitRepo.GetHook(hookID)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetHook", err)
		}
		return
	}

	hook.Content = form.Content
	if err = hook.Update(); err != nil {
		ctx.Error(500, "hook.Update", err)
		return
	}

	ctx.JSON(200, convert.ToGitHook(hook))
}

// DeleteGitHook delete a Git hook of a repository
func DeleteGitHook(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/hooks/git/{id} repository repoDeleteGitHook
	// ---
	// summary: Delete a Git hook in a repository
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
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	hookID := ctx.Params(":id")
	hook, err := ctx.Repo.GitRepo.GetHook(hookID)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetHook", err)
		}
		return
	}

	hook.Content = ""
	if err = hook.Update(); err != nil {
		ctx.Error(500, "hook.Update", err)
		return
	}

	ctx.Status(204)
}
