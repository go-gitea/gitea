// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"net/url"

	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
)

// GetGitAllRefs get ref or a list of all the refs of a repository
func GetGitAllRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs repository repoListAllGitRefs
	// ---
	// summary: Get all the refs of a repository
	// description: Always returns a list, even when the repository has only one reference.
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
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, "")
}

// GetGitRefs get ref or an filteresd list of refs of a repository
func GetGitRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs/{ref} repository repoListGitRefs
	// ---
	// summary: Get a specified ref or the refs matching a prefix
	// description: |
	//   The "ref" path parameter is matched against reference names without the leading "refs/",
	//   so "heads/main" matches the reference "refs/heads/main".
	//   When "ref" is the complete name of an existing reference, a single Reference object is returned.
	//   Otherwise a list of all references starting with that prefix is returned.
	//   The response shape only depends on "ref", never on the other references in the repository:
	//   with the branches "main" and "main1", "heads/main" returns the single "refs/heads/main" object,
	//   "heads/main1" returns the single "refs/heads/main1" object, and "heads/mai" returns both as a list.
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
	// - name: ref
	//   in: path
	//   description: part or full name of the ref
	//   type: string
	//   required: true
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, ctx.PathParam("*"))
}

func getGitRefsInternal(ctx *context.APIContext, filter string) {
	refs, lastMethodName, err := utils.GetGitRefs(ctx, filter)
	if err != nil {
		ctx.APIErrorInternal(fmt.Errorf("%s: %w", lastMethodName, err))
		return
	}

	if len(refs) == 0 {
		ctx.APIErrorNotFound()
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = &api.Reference{
			Ref: refs[i].Name,
			URL: ctx.Repo.Repository.APIURL() + "/git/" + util.PathEscapeSegments(refs[i].Name),
			Object: &api.GitObject{
				SHA:  refs[i].Object.String(),
				Type: refs[i].Type,
				URL:  ctx.Repo.Repository.APIURL() + "/git/" + url.PathEscape(refs[i].Type) + "s/" + url.PathEscape(refs[i].Object.String()),
			},
		}
	}
	// If the filter names an existing reference exactly, return that reference as an object.
	// The filter never carries the "refs/" prefix here because utils.GetGitRefs prepends it before matching,
	// while a reference name always has it.
	// Only the filter decides the response shape, the other references in the repository never do:
	// with the branches "main" and "main1", "heads/main" returns the "refs/heads/main" object,
	// while "heads/mai" matches no reference exactly and returns both of them as a list.
	if filter != "" {
		fullRefName := "refs/" + filter
		for _, apiRef := range apiRefs {
			if apiRef.Ref == fullRefName {
				ctx.JSON(http.StatusOK, apiRef)
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, &apiRefs)
}
