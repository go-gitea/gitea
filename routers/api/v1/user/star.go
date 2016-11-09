// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/routers/api/v1/repo"
)

func getStarredRepos(userID int64) ([]*api.Repository, error) {
	starredRepos, err := models.GetStarredRepos(userID)
	if err != nil {
		return nil, err
	}
	repos := make([]*api.Repository, len(starredRepos))
	for i, starred := range starredRepos {
		repos[i] = starred.APIFormat(&api.Permission{true, true, true})
	}
	return repos, nil
}

func GetStarredRepos(ctx *context.APIContext) {
	user := GetUserByParams(ctx)
	repos, err := getStarredRepos(user.ID)
	if err != nil {
		ctx.Error(500, "getStarredRepos", err)
	}
	ctx.JSON(200, &repos)
}

func GetMyStarredRepos(ctx *context.APIContext) {
	repos, err := getStarredRepos(ctx.User.ID)
	if err != nil {
		ctx.Error(500, "getStarredRepos", err)
	}
	ctx.JSON(200, &repos)
}

func IsStarring(ctx *context.APIContext) {
	_, repository := repo.ParseOwnerAndRepo(ctx)
	if ctx.Written() {
		return
	}
	if models.IsStaring(ctx.User.ID, repository.ID) {
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

func Star(ctx *context.APIContext) {
	_, repository := repo.ParseOwnerAndRepo(ctx)
	if ctx.Written() {
		return
	}
	err := models.StarRepo(ctx.User.ID, repository.ID, true)
	if err != nil {
		ctx.Error(500, "StarRepo", err)
		return
	}
	ctx.Status(204)
}

func Unstar(ctx *context.APIContext) {
	_, repository := repo.ParseOwnerAndRepo(ctx)
	if ctx.Written() {
		return
	}
	err := models.StarRepo(ctx.User.ID, repository.ID, false)
	if err != nil {
		ctx.Error(500, "StarRepo", err)
		return
	}
	ctx.Status(204)
}
