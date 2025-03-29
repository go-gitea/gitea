// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	release_service "code.gitea.io/gitea/services/release"
)

// GetRelease get a single release of a repository
func GetRelease(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id} repository repoGetRelease
	// ---
	// summary: Get a release
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
	//   description: id of the release to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	id := ctx.PathParamInt64("id")
	release, err := repo_model.GetReleaseForRepoByID(ctx, ctx.Repo.Repository.ID, id)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.APIErrorInternal(err)
		return
	}
	if err != nil && repo_model.IsErrReleaseNotExist(err) || release.IsTag {
		ctx.APIErrorNotFound()
		return
	}

	if err := release.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, release))
}

// GetLatestRelease gets the most recent non-prerelease, non-draft release of a repository, sorted by created_at
func GetLatestRelease(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/latest repository repoGetLatestRelease
	// ---
	// summary: Gets the most recent non-prerelease, non-draft release of a repository, sorted by created_at
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
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"
	release, err := repo_model.GetLatestReleaseByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.APIErrorInternal(err)
		return
	}
	if err != nil && repo_model.IsErrReleaseNotExist(err) ||
		release.IsTag || release.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	if err := release.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, release))
}

// ListReleases list a repository's releases
func ListReleases(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases repository repoListReleases
	// ---
	// summary: List a repo's releases
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
	// - name: draft
	//   in: query
	//   description: filter (exclude / include) drafts, if you dont have repo write access none will show
	//   type: boolean
	// - name: pre-release
	//   in: query
	//   description: filter (exclude / include) pre-releases
	//   type: boolean
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
	//     "$ref": "#/responses/ReleaseList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	listOptions := utils.GetListOptions(ctx)

	opts := repo_model.FindReleasesOptions{
		ListOptions:   listOptions,
		IncludeDrafts: ctx.Repo.AccessMode >= perm.AccessModeWrite || ctx.Repo.UnitAccessMode(unit.TypeReleases) >= perm.AccessModeWrite,
		IncludeTags:   false,
		IsDraft:       ctx.FormOptionalBool("draft"),
		IsPreRelease:  ctx.FormOptionalBool("pre-release"),
		RepoID:        ctx.Repo.Repository.ID,
	}

	releases, err := db.Find[repo_model.Release](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	rels := make([]*api.Release, len(releases))
	for i, release := range releases {
		if err := release.LoadAttributes(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		rels[i] = convert.ToAPIRelease(ctx, ctx.Repo.Repository, release)
	}

	filteredCount, err := db.Count[repo_model.Release](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(int(filteredCount), listOptions.PageSize)
	ctx.SetTotalCountHeader(filteredCount)
	ctx.JSON(http.StatusOK, rels)
}

// CreateRelease create a release
func CreateRelease(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases repository repoCreateRelease
	// ---
	// summary: Create a release
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
	//     "$ref": "#/definitions/CreateReleaseOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateReleaseOption)
	if ctx.Repo.Repository.IsEmpty {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("repo is empty"))
		return
	}
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorInternal(err)
			return
		}
		// If target is not provided use default branch
		if len(form.Target) == 0 {
			form.Target = ctx.Repo.Repository.DefaultBranch
		}
		rel = &repo_model.Release{
			RepoID:       ctx.Repo.Repository.ID,
			PublisherID:  ctx.Doer.ID,
			Publisher:    ctx.Doer,
			TagName:      form.TagName,
			Target:       form.Target,
			Title:        form.Title,
			Note:         form.Note,
			IsDraft:      form.IsDraft,
			IsPrerelease: form.IsPrerelease,
			IsTag:        false,
			Repo:         ctx.Repo.Repository,
		}
		if err := release_service.CreateRelease(ctx.Repo.GitRepo, rel, nil, ""); err != nil {
			if repo_model.IsErrReleaseAlreadyExist(err) {
				ctx.APIError(http.StatusConflict, err)
			} else if release_service.IsErrProtectedTagName(err) {
				ctx.APIError(http.StatusUnprocessableEntity, err)
			} else if git.IsErrNotExist(err) {
				ctx.APIError(http.StatusNotFound, fmt.Errorf("target \"%v\" not found: %w", rel.Target, err))
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}
	} else {
		if !rel.IsTag {
			ctx.APIError(http.StatusConflict, "Release is has no Tag")
			return
		}

		rel.Title = form.Title
		rel.Note = form.Note
		rel.IsDraft = form.IsDraft
		rel.IsPrerelease = form.IsPrerelease
		rel.PublisherID = ctx.Doer.ID
		rel.IsTag = false
		rel.Repo = ctx.Repo.Repository
		rel.Publisher = ctx.Doer
		rel.Target = form.Target

		if err = release_service.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo, rel, nil, nil, nil); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIRelease(ctx, ctx.Repo.Repository, rel))
}

// EditRelease edit a release
func EditRelease(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id} repository repoEditRelease
	// ---
	// summary: Update a release
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
	// - name: id
	//   in: path
	//   description: id of the release to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReleaseOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditReleaseOption)
	id := ctx.PathParamInt64("id")
	rel, err := repo_model.GetReleaseForRepoByID(ctx, ctx.Repo.Repository.ID, id)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.APIErrorInternal(err)
		return
	}
	if err != nil && repo_model.IsErrReleaseNotExist(err) || rel.IsTag {
		ctx.APIErrorNotFound()
		return
	}

	if len(form.TagName) > 0 {
		rel.TagName = form.TagName
	}
	if len(form.Target) > 0 {
		rel.Target = form.Target
	}
	if len(form.Title) > 0 {
		rel.Title = form.Title
	}
	if len(form.Note) > 0 {
		rel.Note = form.Note
	}
	if form.IsDraft != nil {
		rel.IsDraft = *form.IsDraft
	}
	if form.IsPrerelease != nil {
		rel.IsPrerelease = *form.IsPrerelease
	}
	if err := release_service.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo, rel, nil, nil, nil); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// reload data from database
	rel, err = repo_model.GetReleaseByID(ctx, id)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := rel.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, rel))
}

// DeleteRelease delete a release from a repository
func DeleteRelease(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id} repository repoDeleteRelease
	// ---
	// summary: Delete a release
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
	//   description: id of the release to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	id := ctx.PathParamInt64("id")
	rel, err := repo_model.GetReleaseForRepoByID(ctx, ctx.Repo.Repository.ID, id)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.APIErrorInternal(err)
		return
	}
	if err != nil && repo_model.IsErrReleaseNotExist(err) || rel.IsTag {
		ctx.APIErrorNotFound()
		return
	}
	if err := release_service.DeleteReleaseByID(ctx, ctx.Repo.Repository, rel, ctx.Doer, false); err != nil {
		if release_service.IsErrProtectedTagName(err) {
			ctx.APIError(http.StatusUnprocessableEntity, "user not allowed to delete protected tag")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
