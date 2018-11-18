// Copyright 2018 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"
)

// GetGitRefs get ref or an list all the refs of a repository
func GetGitRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs/{ref} repository repoListGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
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
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reference"
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.Error(500, "OpenRepository", err)
		return
	}
	filter := ctx.Params("*")
	if len(filter) > 0 {
		filter = "refs/" + filter
	}

	refs, err := gitRepo.GetRefsFiltered(filter)
	if err != nil {
		ctx.Error(500, "GetRefsFiltered", err)
		return
	}

	if len(refs) == 0 {
		ctx.Status(404)
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = &api.Reference{
			Ref: refs[i].Name,
			URL: ctx.Repo.Repository.APIURL() + "/git/" + refs[i].Name,
			Object: &api.GitObject{
				SHA:  refs[i].Object.String(),
				Type: refs[i].Type,
				// TODO: Add commit/tag info URL
				//URL:  ctx.Repo.Repository.APIURL() + "/git/" + refs[i].Type + "s/" + refs[i].Object.String(),
			},
		}
	}
	// If single reference is found and it matches filter exactly return it as object
	if len(apiRefs) == 1 && apiRefs[0].Ref == filter {
		ctx.JSON(200, &apiRefs[0])
		return
	}
	ctx.JSON(200, &apiRefs)
}
