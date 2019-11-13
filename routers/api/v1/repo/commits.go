// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// GetSingleCommit get a commit via
func GetSingleCommit(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/commits/{sha} repository repoGetSingleCommit
	// ---
	// summary: Get a single commit from a repository
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
	//   description: the commit hash
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Commit"
	//   "404":
	//     "$ref": "#/responses/notFound"

	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(ctx.Params(":sha"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommit", git.IsErrNotExist, err)
		return
	}

	// Retrieve author and committer information
	var apiAuthor, apiCommitter *api.User
	author, err := models.GetUserByEmail(commit.Author.Email)
	if err != nil && !models.IsErrUserNotExist(err) {
		ctx.ServerError("Get user by author email", err)
		return
	} else if err == nil {
		apiAuthor = author.APIFormat()
	}
	// Save one query if the author is also the committer
	if commit.Committer.Email == commit.Author.Email {
		apiCommitter = apiAuthor
	} else {
		committer, err := models.GetUserByEmail(commit.Committer.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			ctx.ServerError("Get user by committer email", err)
			return
		} else if err == nil {
			apiCommitter = committer.APIFormat()
		}
	}

	// Retrieve parent(s) of the commit
	apiParents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, _ := commit.ParentID(i)
		apiParents[i] = &api.CommitMeta{
			URL: ctx.Repo.Repository.APIURL() + "/git/commits/" + sha.String(),
			SHA: sha.String(),
		}
	}

	ctx.JSON(200, &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL: setting.AppURL + ctx.Link[1:],
			SHA: commit.ID.String(),
		},
		HTMLURL: ctx.Repo.Repository.HTMLURL() + "/commit/" + commit.ID.String(),
		RepoCommit: &api.RepoCommit{
			URL: setting.AppURL + ctx.Link[1:],
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
				},
				Date: commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.Message(),
			Tree: &api.CommitMeta{
				URL: ctx.Repo.Repository.APIURL() + "/git/trees/" + commit.ID.String(),
				SHA: commit.ID.String(),
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
	})
}
