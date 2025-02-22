// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// ListHooks list all hooks of a repository
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks repository repoListHooks
	// ---
	// summary: List the hooks in a repository
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/HookList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := &webhook.ListWebhookOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
	}

	hooks, count, err := db.FindAndCount[webhook.Webhook](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i := range hooks {
		apiHooks[i], err = webhook_service.ToHook(ctx.Repo.RepoLink, hooks[i])
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiHooks)
}

// GetHook get a repo's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks/{id} repository repoGetHook
	// ---
	// summary: Get a hook
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
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo
	hookID := ctx.PathParamInt64("id")
	hook, err := utils.GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	apiHook, err := webhook_service.ToHook(repo.RepoLink, hook)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiHook)
}

// TestHook tests a hook
func TestHook(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/hooks/{id}/tests repository repoTestHook
	// ---
	// summary: Test a push webhook
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
	//   description: id of the hook to test
	//   type: integer
	//   format: int64
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag, indicates which commit will be loaded to the webhook payload."
	//   type: string
	//   required: false
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Commit == nil {
		// if repo does not have any commits, then don't send a webhook
		ctx.Status(http.StatusNoContent)
		return
	}

	ref := git.BranchPrefix + ctx.Repo.Repository.DefaultBranch
	if r := ctx.FormTrim("ref"); r != "" {
		ref = r
	}

	hookID := ctx.PathParamInt64("id")
	hook, err := utils.GetRepoHook(ctx, ctx.Repo.Repository.ID, hookID)
	if err != nil {
		return
	}

	commit := convert.ToPayloadCommit(ctx, ctx.Repo.Repository, ctx.Repo.Commit)

	commitID := ctx.Repo.Commit.ID.String()
	if err := webhook_service.PrepareWebhook(ctx, hook, webhook_module.HookEventPush, &api.PushPayload{
		Ref:          ref,
		Before:       commitID,
		After:        commitID,
		CompareURL:   setting.AppURL + ctx.Repo.Repository.ComposeCompareURL(commitID, commitID),
		Commits:      []*api.PayloadCommit{commit},
		TotalCommits: 1,
		HeadCommit:   commit,
		Repo:         convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Pusher:       convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone),
		Sender:       convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone),
	}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateHook create a hook for a repository
func CreateHook(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/hooks repository repoCreateHook
	// ---
	// summary: Create a hook
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateHookOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"
	//   "404":
	//     "$ref": "#/responses/notFound"

	utils.AddRepoHook(ctx, web.GetForm(ctx).(*api.CreateHookOption))
}

// EditHook modify a hook of a repository
func EditHook(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/hooks/{id} repository repoEditHook
	// ---
	// summary: Edit a hook in a repository
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
	//   description: index of the hook
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditHookOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.EditHookOption)
	hookID := ctx.PathParamInt64("id")
	utils.EditRepoHook(ctx, form, hookID)
}

// DeleteHook delete a hook of a repository
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/hooks/{id} repository repoDeleteHook
	// ---
	// summary: Delete a hook in a repository
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
	//   description: id of the hook to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	if err := webhook.DeleteWebhookByRepoID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id")); err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
