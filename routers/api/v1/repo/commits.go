// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// GetSingleCommit get a commit via
func GetSingleCommit(c *context.APIContext) {
	gitRepo, err := git.OpenRepository(c.Repo.Repository.RepoPath())
	if err != nil {
		c.ServerError("OpenRepository", err)
		return
	}
	commit, err := gitRepo.GetCommit(c.Params(":sha"))
	if err != nil {
		c.NotFoundOrServerError("GetCommit", git.IsErrNotExist, err)
		return
	}

	// Retrieve author and committer information
	var apiAuthor, apiCommitter *api.User
	author, err := models.GetUserByEmail(commit.Author.Email)
	if err != nil && !models.IsErrUserNotExist(err) {
		c.ServerError("Get user by author email", err)
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
			c.ServerError("Get user by committer email", err)
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
			URL: c.BaseURL + "/repos/" + c.Repo.Repository.FullName() + "/commits/" + sha.String(),
			SHA: sha.String(),
		}
	}

	c.JSON(200, &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL: setting.AppURL + c.Link[1:],
			SHA: commit.ID.String(),
		},
		HTMLURL: c.Repo.Repository.HTMLURL() + "/commits/" + commit.ID.String(),
		RepoCommit: &api.RepoCommit{
			URL: setting.AppURL + c.Link[1:],
			Author: &api.CommitUser{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				Date:  commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
				Date:  commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.Summary(),
			Tree: &api.CommitMeta{
				URL: c.BaseURL + "/repos/" + c.Repo.Repository.FullName() + "/tree/" + commit.ID.String(),
				SHA: commit.ID.String(),
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
	})
}
