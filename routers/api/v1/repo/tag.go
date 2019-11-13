// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"net/http"

	api "code.gitea.io/gitea/modules/structs"
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/TagList"
	tags, err := ctx.Repo.GitRepo.GetTagInfos()
	if err != nil {
		ctx.Error(500, "GetTags", err)
		return
	}

	apiTags := make([]*api.Tag, len(tags))
	for i := range tags {
		apiTags[i] = convert.ToTag(ctx.Repo.Repository, tags[i])
	}

	ctx.JSON(200, &apiTags)
}

// GetTag get the tag of a repository.
func GetTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/tags/{sha} repository GetTag
	// ---
	// summary: Gets the tag of a repository.
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
	//   description: sha of the tag
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AnnotatedTag"

	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "", "SHA not provided")
		return
	}

	if tag, err := ctx.Repo.GitRepo.GetAnnotatedTag(sha); err != nil {
		ctx.Error(http.StatusBadRequest, "GetTag", err)
	} else {
		commit, err := tag.Commit()
		if err != nil {
			ctx.Error(http.StatusBadRequest, "GetTag", err)
		}
		ctx.JSON(http.StatusOK, convert.ToAnnotatedTag(ctx.Repo.Repository, tag, commit))
	}
}
