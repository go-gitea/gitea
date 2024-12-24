// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	release_service "code.gitea.io/gitea/services/release"
)

// ListTags list all the tags of a repository
func ListTags(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tags repository repoListTags
	// ---
	// summary: List a repository's tags
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
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TagList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listOpts := utils.GetListOptions(ctx)

	tags, total, err := ctx.Repo.GitRepo.GetTagInfos(listOpts.Page, listOpts.PageSize)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTags", err)
		return
	}

	apiTags := make([]*api.Tag, len(tags))
	for i := range tags {
		apiTags[i] = convert.ToTag(ctx.Repo.Repository, tags[i])
	}

	ctx.SetTotalCountHeader(int64(total))
	ctx.JSON(http.StatusOK, &apiTags)
}

// GetAnnotatedTag get the tag of a repository.
func GetAnnotatedTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/tags/{sha} repository GetAnnotatedTag
	// ---
	// summary: Gets the tag object of an annotated tag (not lightweight tags)
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
	//   description: sha of the tag. The Git tags API only supports annotated tag objects, not lightweight tags.
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AnnotatedTag"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.PathParam("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "", "SHA not provided")
		return
	}

	if tag, err := ctx.Repo.GitRepo.GetAnnotatedTag(sha); err != nil {
		ctx.Error(http.StatusBadRequest, "GetAnnotatedTag", err)
	} else {
		commit, err := tag.Commit(ctx.Repo.GitRepo)
		if err != nil {
			ctx.Error(http.StatusBadRequest, "GetAnnotatedTag", err)
		}
		ctx.JSON(http.StatusOK, convert.ToAnnotatedTag(ctx, ctx.Repo.Repository, tag, commit))
	}
}

// GetTag get the tag of a repository
func GetTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tags/{tag} repository repoGetTag
	// ---
	// summary: Get the tag of a repository by tag name
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
	// - name: tag
	//   in: path
	//   description: name of tag
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Tag"
	//   "404":
	//     "$ref": "#/responses/notFound"
	tagName := ctx.PathParam("*")

	tag, err := ctx.Repo.GitRepo.GetTag(tagName)
	if err != nil {
		ctx.NotFound(tagName)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTag(ctx.Repo.Repository, tag))
}

// CreateTag create a new git tag in a repository
func CreateTag(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/tags repository repoCreateTag
	// ---
	// summary: Create a new git tag in a repository
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
	//     "$ref": "#/definitions/CreateTagOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Tag"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/empty"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm(ctx).(*api.CreateTagOption)

	// If target is not provided use default branch
	if len(form.Target) == 0 {
		form.Target = ctx.Repo.Repository.DefaultBranch
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(form.Target)
	if err != nil {
		ctx.Error(http.StatusNotFound, "target not found", fmt.Errorf("target not found: %w", err))
		return
	}

	if err := release_service.CreateNewTag(ctx, ctx.Doer, ctx.Repo.Repository, commit.ID.String(), form.TagName, form.Message); err != nil {
		if release_service.IsErrTagAlreadyExists(err) {
			ctx.Error(http.StatusConflict, "tag exist", err)
			return
		}
		if release_service.IsErrProtectedTagName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "CreateNewTag", "user not allowed to create protected tag")
			return
		}

		ctx.InternalServerError(err)
		return
	}

	tag, err := ctx.Repo.GitRepo.GetTag(form.TagName)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToTag(ctx.Repo.Repository, tag))
}

// DeleteTag delete a specific tag of in a repository by name
func DeleteTag(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/tags/{tag} repository repoDeleteTag
	// ---
	// summary: Delete a repository's tag by name
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
	// - name: tag
	//   in: path
	//   description: name of tag to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/empty"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	tagName := ctx.PathParam("*")

	tag, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if !tag.IsTag {
		ctx.Error(http.StatusConflict, "IsTag", errors.New("a tag attached to a release cannot be deleted directly"))
		return
	}

	if err = release_service.DeleteReleaseByID(ctx, ctx.Repo.Repository, tag, ctx.Doer, true); err != nil {
		if release_service.IsErrProtectedTagName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "delTag", "user not allowed to delete protected tag")
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteReleaseByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListTagProtection lists tag protections for a repo
func ListTagProtection(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tag_protections repository repoListTagProtection
	// ---
	// summary: List tag protections for a repository
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
	//     "$ref": "#/responses/TagProtectionList"

	repo := ctx.Repo.Repository
	pts, err := git_model.GetProtectedTags(ctx, repo.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTags", err)
		return
	}
	apiPts := make([]*api.TagProtection, len(pts))
	for i := range pts {
		apiPts[i] = convert.ToTagProtection(ctx, pts[i], repo)
	}

	ctx.JSON(http.StatusOK, apiPts)
}

// GetTagProtection gets a tag protection
func GetTagProtection(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/tag_protections/{id} repository repoGetTagProtection
	// ---
	// summary: Get a specific tag protection for the repository
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
	//   description: id of the tag protect to get
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TagProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	id := ctx.PathParamInt64("id")
	pt, err := git_model.GetProtectedTagByID(ctx, id)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTagByID", err)
		return
	}

	if pt == nil || repo.ID != pt.RepoID {
		ctx.NotFound()
		return
	}

	ctx.JSON(http.StatusOK, convert.ToTagProtection(ctx, pt, repo))
}

// CreateTagProtection creates a tag protection for a repo
func CreateTagProtection(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/tag_protections repository repoCreateTagProtection
	// ---
	// summary: Create a tag protections for a repository
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
	//     "$ref": "#/definitions/CreateTagProtectionOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/TagProtection"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateTagProtectionOption)
	repo := ctx.Repo.Repository

	namePattern := strings.TrimSpace(form.NamePattern)
	if namePattern == "" {
		ctx.Error(http.StatusBadRequest, "name_pattern are empty", "name_pattern are empty")
		return
	}

	if len(form.WhitelistUsernames) == 0 && len(form.WhitelistTeams) == 0 {
		ctx.Error(http.StatusBadRequest, "both whitelist_usernames and whitelist_teams are empty", "both whitelist_usernames and whitelist_teams are empty")
		return
	}

	pt, err := git_model.GetProtectedTagByNamePattern(ctx, repo.ID, namePattern)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectTagOfRepo", err)
		return
	} else if pt != nil {
		ctx.Error(http.StatusForbidden, "Create tag protection", "Tag protection already exist")
		return
	}

	var whitelistUsers, whitelistTeams []int64
	whitelistUsers, err = user_model.GetUserIDsByNames(ctx, form.WhitelistUsernames, false)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}

	if repo.Owner.IsOrganization() {
		whitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.WhitelistTeams, false)
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
	}

	protectTag := &git_model.ProtectedTag{
		RepoID:           repo.ID,
		NamePattern:      strings.TrimSpace(namePattern),
		AllowlistUserIDs: whitelistUsers,
		AllowlistTeamIDs: whitelistTeams,
	}
	if err := git_model.InsertProtectedTag(ctx, protectTag); err != nil {
		ctx.Error(http.StatusInternalServerError, "InsertProtectedTag", err)
		return
	}

	pt, err = git_model.GetProtectedTagByID(ctx, protectTag.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTagByID", err)
		return
	}

	if pt == nil || pt.RepoID != repo.ID {
		ctx.Error(http.StatusInternalServerError, "New tag protection not found", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToTagProtection(ctx, pt, repo))
}

// EditTagProtection edits a tag protection for a repo
func EditTagProtection(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/tag_protections/{id} repository repoEditTagProtection
	// ---
	// summary: Edit a tag protections for a repository. Only fields that are set will be changed
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
	//   description: id of protected tag
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditTagProtectionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/TagProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	repo := ctx.Repo.Repository
	form := web.GetForm(ctx).(*api.EditTagProtectionOption)

	id := ctx.PathParamInt64("id")
	pt, err := git_model.GetProtectedTagByID(ctx, id)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTagByID", err)
		return
	}

	if pt == nil || pt.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	if form.NamePattern != nil {
		pt.NamePattern = *form.NamePattern
	}

	var whitelistUsers, whitelistTeams []int64
	if form.WhitelistTeams != nil {
		if repo.Owner.IsOrganization() {
			whitelistTeams, err = organization.GetTeamIDsByNames(ctx, repo.OwnerID, form.WhitelistTeams, false)
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		}
		pt.AllowlistTeamIDs = whitelistTeams
	}

	if form.WhitelistUsernames != nil {
		whitelistUsers, err = user_model.GetUserIDsByNames(ctx, form.WhitelistUsernames, false)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
		pt.AllowlistUserIDs = whitelistUsers
	}

	err = git_model.UpdateProtectedTag(ctx, pt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectedTag", err)
		return
	}

	pt, err = git_model.GetProtectedTagByID(ctx, id)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTagByID", err)
		return
	}

	if pt == nil || pt.RepoID != repo.ID {
		ctx.Error(http.StatusInternalServerError, "New tag protection not found", "New tag protection not found")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToTagProtection(ctx, pt, repo))
}

// DeleteTagProtection
func DeleteTagProtection(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/tag_protections/{id} repository repoDeleteTagProtection
	// ---
	// summary: Delete a specific tag protection for the repository
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
	//   description: id of protected tag
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	id := ctx.PathParamInt64("id")
	pt, err := git_model.GetProtectedTagByID(ctx, id)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedTagByID", err)
		return
	}

	if pt == nil || pt.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	err = git_model.DeleteProtectedTag(ctx, pt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProtectedTag", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
