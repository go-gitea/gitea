// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// Avatar redirect browser to user avatar of requested size
func Avatar(ctx *context.Context) {
	userName := ctx.Params(":username")
	size, err := strconv.Atoi(ctx.Params(":size"))
	if err != nil {
		ctx.ServerError("Invalid avatar size", err)
		return
	}

	log.Debug("Asked avatar for user %v and size %v", userName, size)

	var user *models.User
	if strings.ToLower(userName) != "ghost" {
		user, err = models.GetUserByName(userName)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.ServerError("Requested avatar for invalid user", err)
			} else {
				ctx.ServerError("Retrieving user by name", err)
			}
			return
		}
	} else {
		user = models.NewGhostUser()
	}

	ctx.Redirect(user.RealSizedAvatarLink(size))
}

// AvatarByEmailHash redirects the browser to the appropriate Avatar link
func AvatarByEmailHash(ctx *context.Context) {
	hash := ctx.Params(":hash")
	if len(hash) == 0 {
		ctx.ServerError("invalid avatar hash", errors.New("hash cannot be empty"))
		return
	}
	email, err := models.GetEmailForHash(hash)
	if err != nil {
		ctx.ServerError("invalid avatar hash", err)
		return
	}
	if len(email) == 0 {
		ctx.Redirect(base.DefaultAvatarLink())
		return
	}
	size := ctx.QueryInt("size")
	if size == 0 {
		size = base.DefaultAvatarSize
	}
	ctx.Redirect(base.SizedAvatarLinkWithDomain(email, size))
}
