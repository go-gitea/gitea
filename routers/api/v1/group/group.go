// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"errors"
	"net/http"
	"strings"

	group_model "gitea.dev/models/group"
	org_model "gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	shared_group_model "gitea.dev/models/shared/group"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	group_service "gitea.dev/services/group"
)

func createCommonGroup(ctx *context.APIContext, parentGroupID, ownerID int64) *api.Group {
	if ownerID < 1 {
		if parentGroupID < 1 {
			ctx.APIError(http.StatusUnprocessableEntity,
				errors.New("cannot determine new group's owner"))
			return nil
		}
		npg, err := group_model.GetGroupByID(ctx, parentGroupID)
		if err != nil {
			if group_model.IsErrGroupNotExist(err) {
				ctx.APIErrorNotFound()
			} else {
				ctx.APIErrorInternal(err)
			}
			return nil
		}
		ownerID = npg.OwnerID
	}
	form := web.GetForm(ctx).(*api.NewGroupOption)
	group := &group_model.Group{
		Name:          form.Name,
		Description:   form.Description,
		OwnerID:       ownerID,
		LowerName:     strings.ToLower(form.Name),
		Visibility:    form.Visibility,
		ParentGroupID: parentGroupID,
	}
	if err := group_service.NewGroup(ctx, group); err != nil {
		if group_model.IsErrGroupTooDeep(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else if org_model.IsErrOrgNotExist(err) {
			ctx.APIErrorNotFound()
		}
		return nil
	}
	val, err := convert.ToAPIGroup(ctx, group, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	return val
}

// NewGroup create a new root-level group in an organization
func NewGroup(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/groups/new repository-group groupNew
	// ---
	// summary: create a root-level repository group for an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/NewGroupOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Group"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	ag := createCommonGroup(ctx, 0, ctx.Org.Organization.ID)
	if !ctx.Written() {
		ctx.JSON(http.StatusCreated, ag)
	}
}

// NewSubGroup create a new subgroup inside a group
func NewSubGroup(ctx *context.APIContext) {
	// swagger:operation POST /groups/{group_id}/new repository-group groupNewSubGroup
	// ---
	// summary: create a subgroup inside a group
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group to create a subgroup in
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/NewGroupOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Group"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	gid := ctx.PathParamInt64("group_id")
	group := createCommonGroup(ctx, gid, 0)
	if !ctx.Written() {
		ctx.JSON(http.StatusCreated, group)
	}
}

// MoveGroup - move a group to a different group in the same organization, or to the root level if
func MoveGroup(ctx *context.APIContext) {
	// swagger:operation POST /groups/{group_id}/move repository-group groupMove
	// ---
	// summary: move a group to a different parent group
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group to move
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/MoveGroupOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Group"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.MoveGroupOption)
	id := ctx.PathParamInt64("group_id")
	var err error
	npos := -1
	if form.NewPos != nil {
		npos = *form.NewPos
	}
	err = group_service.MoveGroupItem(ctx, group_service.MoveGroupOptions{
		NewParent: form.NewParent,
		ItemID:    id,
		IsGroup:   true,
		NewPos:    npos,
	}, ctx.Doer)
	if group_model.IsErrGroupNotExist(err) {
		ctx.APIErrorNotFound()
		return
	}
	if group_model.IsErrUserDoesNotHaveAccessToGroup(err) {
		ctx.APIError(http.StatusForbidden, err)
		return
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	var (
		ng       *group_model.Group
		apiGroup *api.Group
	)
	ng, err = group_model.GetGroupByID(ctx, id)
	if group_model.IsErrGroupNotExist(err) {
		ctx.APIErrorNotFound()
		return
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiGroup, err = convert.ToAPIGroup(ctx, ng, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
	}
	ctx.JSON(http.StatusOK, apiGroup)
}

// EditGroup - update a group in an organization
func EditGroup(ctx *context.APIContext) {
	// swagger:operation PATCH /groups/{group_id} repository-group groupEdit
	// ---
	// summary: edits a group in an organization. only fields that are set will be changed.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/EditGroupOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Group"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	var (
		err   error
		group *group_model.Group
	)
	form := web.GetForm(ctx).(*api.EditGroupOption)
	group = ctx.RepoGroup.Group

	serviceOpts := &group_service.UpdateOptions{}
	serviceOpts.Visibility = optional.FromPtr(form.Visibility)
	serviceOpts.Description = optional.FromPtr(form.Description)
	serviceOpts.Name = optional.FromPtr(form.Name)

	err = group_service.UpdateGroup(ctx, group, serviceOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	var newAPIGroup *api.Group
	newAPIGroup, err = convert.ToAPIGroup(ctx, group, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, newAPIGroup)
}

func GetGroup(ctx *context.APIContext) {
	// swagger:operation GET /groups/{group_id} repository-group groupGet
	// ---
	// summary: gets a group in an organization
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group to retrieve
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Group"
	//   "404":
	//     "$ref": "#/responses/notFound"
	apiGroup, err := convert.ToAPIGroup(ctx, ctx.RepoGroup.Group, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, apiGroup)
}

func DeleteGroup(ctx *context.APIContext) {
	// swagger:operation DELETE /groups/{group_id} repository-group groupDelete
	// ---
	// summary: Delete a repository group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group to delete
	//   type: string
	//   required: true
	// - name: recursive
	//   in: query
	//   description: whether to delete all descendant groups and repos
	//   type: boolean
	//   required: false
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	err := group_service.DeleteGroup(ctx, ctx.Doer, ctx.PathParamInt64("group_id"), ctx.FormBool("recursive"))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func GetGroupRepos(ctx *context.APIContext) {
	// swagger:operation GET /groups/{group_id}/repos repository-group groupGetRepos
	// ---
	// summary: gets the repos contained within a group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the group containing the repositories
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	gid := ctx.PathParamInt64("group_id")
	_, err := group_model.GetGroupByID(ctx, gid)
	if err != nil {
		if group_model.IsErrGroupNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	groupRepos, err := shared_group_model.GetGroupRepos(ctx, gid, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	repos := make([]*api.Repository, len(groupRepos))
	for i, repo := range groupRepos {
		permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		repos[i] = convert.ToRepo(ctx, repo, permission)
	}
	ctx.SetTotalCountHeader(int64(len(repos)))
	ctx.JSON(http.StatusOK, repos)
}

func GetGroupSubGroups(ctx *context.APIContext) {
	// swagger:operation GET /groups/{group_id}/subgroups repository-group groupGetSubGroups
	// ---
	// summary: gets the subgroups contained within a group
	// produces:
	// - application/json
	// parameters:
	// - name: group_id
	//   in: path
	//   description: id of the parent group
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/GroupList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	g := ctx.RepoGroup.Group
	err := g.LoadAccessibleSubgroups(ctx, false, ctx.Doer, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	groups := make([]*api.Group, len(g.Subgroups))
	for i, group := range g.Subgroups {
		groups[i], err = convert.ToAPIGroup(ctx, group, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	ctx.SetTotalCountHeader(int64(len(groups)))
	ctx.JSON(http.StatusOK, groups)
}
