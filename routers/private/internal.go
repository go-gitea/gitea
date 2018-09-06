// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	macaron "gopkg.in/macaron.v1"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(ctx *macaron.Context) {
	tokens := ctx.Req.Header.Get("Authorization")
	fields := strings.Fields(tokens)
	if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
		ctx.Error(403)
	}
}

// UpdateDeployKey update deploy key updates
func UpdateDeployKey(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	deployKey, err := models.GetDeployKeyByRepo(keyID, repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	deployKey.UpdatedUnix = util.TimeStampNow()
	if err = models.UpdateDeployKeyCols(deployKey, "updated_unix"); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.PlainText(200, []byte("success"))
}

// UpdatePublicKey update publick key updates
func UpdatePublicKey(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	if err := models.UpdatePublicKeyUpdated(keyID); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.PlainText(200, []byte("success"))
}

//TODO move on specific file
//GetRepositoryByOwnerAndName chainload to models.GetRepositoryByOwnerAndName
func GetRepositoryByOwnerAndName(ctx *macaron.Context) {
	//TODO use repo.Get(ctx *context.APIContext) ?
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, repo)
}

//GetPublicKeyByID chainload to models.GetPublicKeyByID
func GetPublicKeyByID(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	key, err := models.GetPublicKeyByID(keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, key)
}

//GetUserByKeyID chainload to models.GetUserByKeyID
func GetUserByKeyID(ctx *macaron.Context) {
	keyID := ctx.ParamsInt64(":id")
	user, err := models.GetUserByKeyID(keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, user)
}

//HasDeployKey chainload to models.HasDeployKey
func HasDeployKey(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	if models.HasDeployKey(repoID, keyID) {
		ctx.PlainText(200, []byte("success"))
	}
	ctx.PlainText(404, []byte("not found"))
}

//GetUserByKeyID chainload to models.GetUserByKeyID
func AccessLevel(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	userID := ctx.ParamsInt64(":userid")
	repo, err := models.GetRepositoryByID(repoID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	al, err := models.AccessLevel(userID, repo)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, al)
}

/*
//GetDeployKeyByRepo chainload to models.GetDeployKeyByRepo
func GetDeployKeyByRepo(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":repoid")
	keyID := ctx.ParamsInt64(":keyid")
	key, err := models.GetDeployKeyByRepo(repoID, keyID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, key)
}
*/

// RegisterRoutes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func RegisterRoutes(m *macaron.Macaron) {
	m.Group("/", func() {
		m.Get("/ssh/:id", GetPublicKeyByID)
		m.Get("/ssh/:id/user", GetUserByKeyID)
		m.Post("/ssh/:id/update", UpdatePublicKey)
		m.Post("/repositories/:repoid/keys/:keyid/update", UpdateDeployKey)
		m.Get("/repositories/:repoid/user/:userid/accesslevel", AccessLevel)
		//m.Get("/repositories/:repoid/keys/:keyid", GetDeployKeyByRepo)
		m.Get("/repositories/:repoid/has-keys/:keyid", HasDeployKey)
		m.Post("/push/update", PushUpdate)
		m.Get("/protectedbranch/:pbid/:userid", CanUserPush)
		m.Get("/repo/:owner/:repo", GetRepositoryByOwnerAndName)
		m.Get("/branch/:id/*", GetProtectedBranchBy)
		m.Get("/repository/:rid", GetRepository)
		m.Get("/active-pull-request", GetActivePullRequest)
	}, CheckInternalToken)
}
